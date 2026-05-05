#!/usr/bin/env python3
"""Fine-tune a chat/instruct LLM on the tender-risk dataset.

Input dataset format supported:
1) JSONL where each line is an object with keys: instruction, input, output
2) JSON array of such objects

The script converts each record into a conversational prompt-completion example:
- system: instruction
- user: pretty-printed JSON from input
- assistant: pretty-printed JSON from output

Recommended usage:
    python finetune_tender_sft.py \
      --model_name Qwen/Qwen3-0.6B \
      --dataset_path /mnt/data/dataset_train.jsonl \
      --output_dir ./runs/tender-qwen3-0.6b-lora

Notes:
- Best with an instruct/chat model that already has a tokenizer chat template.
- If the tokenizer has no chat template, pass --chat_template_model pointing to a
  compatible instruct model tokenizer, or add your own template file.
"""

from __future__ import annotations

import argparse
import json
import math
import os
from pathlib import Path
from typing import Any, Dict, List

import torch
from datasets import Dataset
from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    BitsAndBytesConfig,
    set_seed,
)
from peft import LoraConfig, prepare_model_for_kbit_training
from trl import SFTConfig, SFTTrainer


REQUIRED_KEYS = {"instruction", "input", "output"}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="QLoRA/LoRA SFT for tender-risk dataset")
    parser.add_argument("--model_name", type=str, required=True, help="Base instruct/chat model")
    parser.add_argument("--dataset_path", type=str, required=True, help="Path to dataset_master.json or dataset_train.jsonl")
    parser.add_argument("--output_dir", type=str, required=True, help="Directory for checkpoints and adapter")
    parser.add_argument("--chat_template_model", type=str, default=None,
                        help="Optional tokenizer source or Jinja template path if base model tokenizer has no chat template")

    parser.add_argument("--max_length", type=int, default=4096)
    parser.add_argument("--num_train_epochs", type=float, default=6.0)
    parser.add_argument("--learning_rate", type=float, default=1e-4)
    parser.add_argument("--weight_decay", type=float, default=0.0)
    parser.add_argument("--warmup_ratio", type=float, default=0.05)
    parser.add_argument("--per_device_train_batch_size", type=int, default=1)
    parser.add_argument("--per_device_eval_batch_size", type=int, default=1)
    parser.add_argument("--gradient_accumulation_steps", type=int, default=8)
    parser.add_argument("--eval_ratio", type=float, default=0.1)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--save_total_limit", type=int, default=2)
    parser.add_argument("--logging_steps", type=int, default=5)

    parser.add_argument("--use_4bit", action="store_true", help="Use 4-bit QLoRA if CUDA is available")
    parser.add_argument("--no_use_4bit", dest="use_4bit", action="store_false")
    parser.set_defaults(use_4bit=True)

    parser.add_argument("--lora_r", type=int, default=16)
    parser.add_argument("--lora_alpha", type=int, default=32)
    parser.add_argument("--lora_dropout", type=float, default=0.05)
    parser.add_argument("--lora_target_modules", type=str, default="all-linear",
                        help="Usually 'all-linear' is the safest generic choice")

    parser.add_argument("--merge_and_save", action="store_true",
                        help="After training, merge LoRA into a full model copy and save it (needs extra RAM/VRAM).")
    return parser.parse_args()


def read_records(path: str) -> List[Dict[str, Any]]:
    path_obj = Path(path)
    if not path_obj.exists():
        raise FileNotFoundError(f"Dataset file not found: {path}")

    text = path_obj.read_text(encoding="utf-8").strip()
    if not text:
        raise ValueError(f"Dataset file is empty: {path}")

    # Try JSON array first.
    try:
        data = json.loads(text)
        if isinstance(data, list):
            records = data
        elif isinstance(data, dict):
            records = [data]
        else:
            raise ValueError("Top-level JSON must be an object or an array")
    except json.JSONDecodeError:
        # Fallback to JSONL.
        records = []
        with path_obj.open("r", encoding="utf-8") as f:
            for line_no, line in enumerate(f, 1):
                line = line.strip()
                if not line:
                    continue
                try:
                    records.append(json.loads(line))
                except json.JSONDecodeError as e:
                    raise ValueError(f"Invalid JSON on line {line_no}: {e}") from e

    if not records:
        raise ValueError("No dataset records were found")

    for idx, rec in enumerate(records):
        if not isinstance(rec, dict):
            raise ValueError(f"Record #{idx} is not a JSON object")
        missing = REQUIRED_KEYS - set(rec)
        if missing:
            raise ValueError(f"Record #{idx} is missing keys: {sorted(missing)}")

    return records


def pretty_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, indent=2, sort_keys=False)


def to_prompt_completion(records: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    converted: List[Dict[str, Any]] = []
    for rec in records:
        system_text = str(rec["instruction"]).strip()
        user_text = pretty_json(rec["input"])
        assistant_text = pretty_json(rec["output"])
        converted.append(
            {
                "prompt": [
                    {"role": "system", "content": system_text},
                    {"role": "user", "content": user_text},
                ],
                "completion": [
                    {"role": "assistant", "content": assistant_text},
                ],
            }
        )
    return converted


def get_split_dataset(examples: List[Dict[str, Any]], eval_ratio: float, seed: int) -> tuple[Dataset, Dataset | None]:
    dataset = Dataset.from_list(examples)
    if len(dataset) < 10 or eval_ratio <= 0:
        return dataset, None

    test_size = max(1, int(len(dataset) * eval_ratio))
    # Keep at least 1 sample in train.
    test_size = min(test_size, len(dataset) - 1)
    split = dataset.train_test_split(test_size=test_size, seed=seed)
    return split["train"], split["test"]


def is_bf16_available() -> bool:
    return torch.cuda.is_available() and torch.cuda.is_bf16_supported()


def load_tokenizer(args: argparse.Namespace):
    tokenizer = AutoTokenizer.from_pretrained(args.model_name, use_fast=True)

    if tokenizer.chat_template is None and args.chat_template_model:
        try:
            template_tokenizer = AutoTokenizer.from_pretrained(args.chat_template_model, use_fast=True)
            if template_tokenizer.chat_template is not None:
                tokenizer.chat_template = template_tokenizer.chat_template
        except Exception:
            # If this is a Jinja file path, read it directly.
            if os.path.exists(args.chat_template_model):
                tokenizer.chat_template = Path(args.chat_template_model).read_text(encoding="utf-8")
            else:
                raise

    if tokenizer.chat_template is None:
        raise ValueError(
            "The tokenizer has no chat template. Use an instruct/chat base model or pass --chat_template_model."
        )

    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    return tokenizer


def load_model(args: argparse.Namespace):
    bf16 = is_bf16_available()
    use_4bit = bool(args.use_4bit and torch.cuda.is_available())

    model_kwargs: Dict[str, Any] = {
        "trust_remote_code": True,
    }

    if torch.cuda.is_available():
        model_kwargs["device_map"] = "auto"
        model_kwargs["torch_dtype"] = torch.bfloat16 if bf16 else torch.float16
    else:
        model_kwargs["device_map"] = None
        model_kwargs["torch_dtype"] = torch.float32

    if use_4bit:
        model_kwargs["quantization_config"] = BitsAndBytesConfig(
            load_in_4bit=True,
            bnb_4bit_quant_type="nf4",
            bnb_4bit_use_double_quant=True,
            bnb_4bit_compute_dtype=torch.bfloat16 if bf16 else torch.float16,
        )

    model = AutoModelForCausalLM.from_pretrained(args.model_name, **model_kwargs)
    model.config.use_cache = False

    if use_4bit:
        model = prepare_model_for_kbit_training(model)

    return model


def build_trainer(
    args: argparse.Namespace,
    model,
    tokenizer,
    train_dataset: Dataset,
    eval_dataset: Dataset | None,
) -> SFTTrainer:
    bf16 = is_bf16_available()
    fp16 = torch.cuda.is_available() and not bf16

    sft_args = SFTConfig(
        output_dir=args.output_dir,
        num_train_epochs=args.num_train_epochs,
        learning_rate=args.learning_rate,
        weight_decay=args.weight_decay,
        warmup_ratio=args.warmup_ratio,
        per_device_train_batch_size=args.per_device_train_batch_size,
        per_device_eval_batch_size=args.per_device_eval_batch_size,
        gradient_accumulation_steps=args.gradient_accumulation_steps,
        logging_steps=args.logging_steps,
        logging_strategy="steps",
        save_strategy="epoch",
        eval_strategy="epoch" if eval_dataset is not None else "no",
        bf16=bf16,
        fp16=fp16,
        gradient_checkpointing=True,
        gradient_checkpointing_kwargs={"use_reentrant": False},
        max_length=args.max_length,
        packing=False,
        completion_only_loss=True,
        report_to="none",
        save_total_limit=args.save_total_limit,
        seed=args.seed,
        dataset_num_proc=1,
        remove_unused_columns=False,
    )

    peft_config = LoraConfig(
        r=args.lora_r,
        lora_alpha=args.lora_alpha,
        lora_dropout=args.lora_dropout,
        bias="none",
        task_type="CAUSAL_LM",
        target_modules=args.lora_target_modules,
    )

    trainer = SFTTrainer(
        model=model,
        args=sft_args,
        train_dataset=train_dataset,
        eval_dataset=eval_dataset,
        processing_class=tokenizer,
        peft_config=peft_config,
    )
    return trainer


def save_dataset_preview(output_dir: str, train_dataset: Dataset, eval_dataset: Dataset | None) -> None:
    out = Path(output_dir)
    out.mkdir(parents=True, exist_ok=True)
    preview = {
        "train_size": len(train_dataset),
        "eval_size": 0 if eval_dataset is None else len(eval_dataset),
        "first_train_example": train_dataset[0] if len(train_dataset) else None,
    }
    (out / "dataset_preview.json").write_text(
        json.dumps(preview, ensure_ascii=False, indent=2), encoding="utf-8"
    )


def maybe_merge_adapter(args: argparse.Namespace, tokenizer) -> None:
    if not args.merge_and_save:
        return

    from peft import AutoPeftModelForCausalLM

    merged_dir = Path(args.output_dir) / "merged"
    model = AutoPeftModelForCausalLM.from_pretrained(
        args.output_dir,
        torch_dtype=torch.bfloat16 if is_bf16_available() else torch.float16,
        low_cpu_mem_usage=True,
        trust_remote_code=True,
    )
    merged = model.merge_and_unload()
    merged.save_pretrained(merged_dir)
    tokenizer.save_pretrained(merged_dir)


def main() -> None:
    args = parse_args()
    set_seed(args.seed)

    records = read_records(args.dataset_path)
    examples = to_prompt_completion(records)
    train_dataset, eval_dataset = get_split_dataset(examples, args.eval_ratio, args.seed)

    tokenizer = load_tokenizer(args)
    model = load_model(args)
    save_dataset_preview(args.output_dir, train_dataset, eval_dataset)

    trainer = build_trainer(args, model, tokenizer, train_dataset, eval_dataset)
    trainer.train()

    trainer.save_model(args.output_dir)
    tokenizer.save_pretrained(args.output_dir)

    metrics = trainer.state.log_history
    Path(args.output_dir).mkdir(parents=True, exist_ok=True)
    (Path(args.output_dir) / "train_log_history.json").write_text(
        json.dumps(metrics, ensure_ascii=False, indent=2), encoding="utf-8"
    )

    maybe_merge_adapter(args, tokenizer)

    print("Training finished successfully.")
    print(f"Adapter/checkpoints saved to: {args.output_dir}")
    if args.merge_and_save:
        print(f"Merged model saved to: {Path(args.output_dir) / 'merged'}")


if __name__ == "__main__":
    main()
