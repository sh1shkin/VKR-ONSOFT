#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import json
import torch
from datasets import Dataset, DatasetDict
from transformers import (
    AutoTokenizer,
    AutoModelForCausalLM,
    BitsAndBytesConfig,
    TrainingArguments,
    Trainer,
)
from peft import (
    LoraConfig,
    get_peft_model,
    prepare_model_for_kbit_training
)

# ============================================
# Конфигурация
# ============================================
MODEL_NAME = "Qwen/Qwen2.5-7B-Instruct"
DATASET_PATH = "data.jsonl"
OUTPUT_DIR = "./qwen2.5-7b-tender-lora"

LORA_R = 16
LORA_ALPHA = 32
LORA_DROPOUT = 0.05

BATCH_SIZE = 4
GRAD_ACCUM_STEPS = 4
MAX_LENGTH = 3072
LEARNING_RATE = 2e-4
NUM_EPOCHS = 3
WARMUP_RATIO = 0.03
LR_SCHEDULER_TYPE = "cosine"
SAVE_STEPS = None  # Будет рассчитан автоматически
LOGGING_STEPS = None

USE_BFLOAT16 = True
USE_TF32 = True

try:
    import flash_attn
    ATTENTION_IMPLEMENTATION = "flash_attention_2"
    print("🚀 Flash Attention 2 обнаружен, будет использован.")
except ImportError:
    ATTENTION_IMPLEMENTATION = "sdpa"
    print("⚡ Flash Attention не найден, используется SDPA.")

# ============================================
# 1. 4-битное квантование
# ============================================
bnb_config = BitsAndBytesConfig(
    load_in_4bit=True,
    bnb_4bit_quant_type="nf4",
    bnb_4bit_compute_dtype=torch.bfloat16 if USE_BFLOAT16 else torch.float16,
    bnb_4bit_use_double_quant=True,
)

# ============================================
# 2. Загрузка модели и токенизатора
# ============================================
print("Загрузка модели и токенизатора...")
tokenizer = AutoTokenizer.from_pretrained(MODEL_NAME, trust_remote_code=True)
tokenizer.padding_side = "right"
if tokenizer.pad_token is None:
    tokenizer.pad_token = tokenizer.eos_token

model = AutoModelForCausalLM.from_pretrained(
    MODEL_NAME,
    quantization_config=bnb_config,
    device_map="auto",
    trust_remote_code=True,
    attn_implementation=ATTENTION_IMPLEMENTATION,
    dtype=torch.bfloat16 if USE_BFLOAT16 else torch.float16,
)
model = prepare_model_for_kbit_training(model)

# ============================================
# 3. LoRA
# ============================================
lora_config = LoraConfig(
    r=LORA_R,
    lora_alpha=LORA_ALPHA,
    target_modules=["q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"],
    lora_dropout=LORA_DROPOUT,
    bias="none",
    task_type="CAUSAL_LM",
)
model = get_peft_model(model, lora_config)
model.print_trainable_parameters()

# ============================================
# 4. Подготовка данных (УСТОЙЧИВЫЙ ПАРСЕР)
# ============================================
def format_chat(example: dict) -> str:
    system_message = example["instruction"]
    user_content = json.dumps(example["input"], ensure_ascii=False, indent=2)
    assistant_content = json.dumps(example["output"], ensure_ascii=False, indent=2)
    
    messages = [
        {"role": "system", "content": system_message},
        {"role": "user", "content": user_content},
        {"role": "assistant", "content": assistant_content}
    ]
    return tokenizer.apply_chat_template(
        messages,
        tokenize=False,
        add_generation_prompt=False
    )

def load_and_prepare_dataset(file_path: str) -> DatasetDict:
    data = []
    with open(file_path, "r", encoding="utf-8") as f:
        content = f.read().strip()

    # 1. Пробуем загрузить как единый JSON
    if content.startswith("[") or content.startswith("{"):
        try:
            loaded = json.loads(content)
            data = loaded if isinstance(loaded, list) else [loaded]
            print(f"📦 Загружено как единый JSON: {len(data)} примеров")
        except json.JSONDecodeError:
            pass  # Fallback к построчному чтению

    # 2. Построчный парсинг (JSONL)
    if not data:
        lines = content.splitlines()
        for i, line in enumerate(lines, 1):
            line = line.strip()
            if not line: continue
            try:
                data.append(json.loads(line))
            except json.JSONDecodeError as e:
                print(f"⚠️ Строка {i} пропущена (невалидный JSON): {str(e)[:60]}...")
        
        if not 
            raise ValueError("Файл пустой или не содержит валидных JSON-объектов.")
        print(f"📄 Загружено построчно (JSONL): {len(data)} примеров")

    # Проверка структуры
    for i, item in enumerate(data):
        missing = [k for k in ("instruction", "input", "output") if k not in item]
        if missing:
            raise ValueError(f"Пример {i} не содержит полей: {missing}. Найдено: {list(item.keys())}")

    formatted = [{"text": format_chat(item)} for item in data]
    dataset = Dataset.from_list(formatted)
    split = dataset.train_test_split(test_size=0.1, seed=42)
    return DatasetDict({"train": split["train"], "validation": split["test"]})

print("Загрузка датасета...")
dataset = load_and_prepare_dataset(DATASET_PATH)
train_len = len(dataset["train"])
val_len = len(dataset["validation"])
print(f"✅ Train: {train_len}, Validation: {val_len}")

# ============================================
# 5. Токенизация и Коллатор (ТЕНЗОРНЫЙ)
# ============================================
def tokenize_function(examples: dict) -> dict:
    tokenized = tokenizer(
        examples["text"],
        truncation=True,
        padding=False,
        max_length=MAX_LENGTH,
        return_tensors=None,
    )
    tokenized["labels"] = tokenized["input_ids"].copy()
    return tokenized

tokenized_dataset = dataset.map(
    tokenize_function,
    batched=True,
    remove_columns=dataset["train"].column_names,
    desc="Tokenizing"
)

# ✅ Коллатор возвращает ТЕНЗОРЫ и маскирует паддинги (-100)
def causal_lm_data_collator(features):
    input_ids = [torch.tensor(f["input_ids"], dtype=torch.long) for f in features]
    attention_mask = [torch.tensor(f["attention_mask"], dtype=torch.long) for f in features]
    labels = [torch.tensor(f["labels"], dtype=torch.long) for f in features]

    input_ids = torch.nn.utils.rnn.pad_sequence(input_ids, batch_first=True, padding_value=tokenizer.pad_token_id)
    attention_mask = torch.nn.utils.rnn.pad_sequence(attention_mask, batch_first=True, padding_value=0)
    labels = torch.nn.utils.rnn.pad_sequence(labels, batch_first=True, padding_value=-100)

    return {"input_ids": input_ids, "attention_mask": attention_mask, "labels": labels}

# ============================================
# 6. Аргументы обучения (АВТО-РАСЧЁТ ШАГОВ)
# ============================================
steps_per_epoch = max(1, train_len // (BATCH_SIZE * GRAD_ACCUM_STEPS))
total_steps = steps_per_epoch * NUM_EPOCHS

training_args = TrainingArguments(
    output_dir=OUTPUT_DIR,
    per_device_train_batch_size=BATCH_SIZE,
    per_device_eval_batch_size=BATCH_SIZE,
    gradient_accumulation_steps=GRAD_ACCUM_STEPS,
    learning_rate=LEARNING_RATE,
    num_train_epochs=NUM_EPOCHS,
    warmup_steps=int(total_steps * WARMUP_RATIO),
    lr_scheduler_type=LR_SCHEDULER_TYPE,
    logging_steps=max(1, steps_per_epoch // 2),
    save_steps=max(1, steps_per_epoch),
    eval_strategy="steps",
    eval_steps=max(1, steps_per_epoch),
    save_total_limit=3,
    bf16=USE_BFLOAT16,
    tf32=USE_TF32,
    gradient_checkpointing=True,
    gradient_checkpointing_kwargs={"use_reentrant": False},
    report_to="none",
    remove_unused_columns=False,
    dataloader_pin_memory=True,
    ddp_find_unused_parameters=False,
    seed=42,
    optim="adamw_torch",
)

# ============================================
# 7. Обучение
# ============================================
trainer = Trainer(
    model=model,
    args=training_args,
    train_dataset=tokenized_dataset["train"],
    eval_dataset=tokenized_dataset["validation"],
    data_collator=causal_lm_data_collator,
)

print(f"🚀 Начинаем обучение: {steps_per_epoch} шагов/эпоха, всего {total_steps} шагов")
trainer.train()

print("💾 Сохранение адаптера LoRA...")
model.save_pretrained(OUTPUT_DIR)
tokenizer.save_pretrained(OUTPUT_DIR)
print("✅ Готово!")