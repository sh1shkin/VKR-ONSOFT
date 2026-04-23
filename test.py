#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import json
import torch
from transformers import AutoModelForCausalLM, AutoTokenizer, BitsAndBytesConfig
from peft import PeftConfig, PeftModel

# =========================
# НАСТРОЙКИ
# =========================
ADAPTER_PATH = "./qwen_output"
BASE_MODEL = "Qwen/Qwen2.5-7B-Instruct"
LOAD_IN_4BIT = True
TRUST_REMOTE_CODE = True
MAX_NEW_TOKENS = 2048
TEMPERATURE = 0.0
TOP_P = 0.95

# =========================
# ПРИМЕР ДЛЯ ТЕСТА
# =========================
INSTRUCTION = """
Ты — тендерный риск-менеджер.
Проанализируй структурированные данные закупки и данные о компании.

Верни ответ СТРОГО в виде одного валидного JSON-объекта.
Не пиши никаких пояснений, markdown, комментариев и текста вне JSON.

Используй СТРОГО следующие верхнеуровневые ключи:
summary, company_fit, risks, pitfalls, recommendations, loss_estimate, final_decision

Обязательные требования:
1. Не добавляй новые верхнеуровневые ключи.
2. Не переименовывай ключи.
3. Для company_fit используй label только из:
   - fit
   - conditional_fit
   - not_fit
4. Для final_decision используй label только из:
   - bid
   - conditional_bid
   - no_bid
5. Поле pitfalls должно быть массивом строк.
6. Поле recommendations должно быть массивом строк.
7. Поле risks должно быть массивом объектов.
8. Поле loss_estimate должно называться именно loss_estimate.
9. Поле final_decision должно называться именно final_decision.
10. Ответ должен начинаться с символа { и быть валидным JSON.

Минимальная форма ответа:
{
  "summary": "...",
  "company_fit": {
    "status": "...",
    "details": ["..."],
    "label": "fit | conditional_fit | not_fit",
    "status_comment": "..."
  },
  "risks": [
    {
      "name": "...",
      "level": "низкий | средний | высокий",
      "basis": ["..."],
      "impact": "..."
    }
  ],
  "pitfalls": ["..."],
  "recommendations": ["..."],
  "loss_estimate": {
    "status": "...",
    "label": "...",
    "details": ["..."]
  },
  "final_decision": {
    "status": "...",
    "details": ["..."],
    "label": "bid | conditional_bid | no_bid",
    "decision_comment": "..."
  }
}
""".strip()

INPUT_DATA = {
    "broker_payload": {
        "purchase_subject": "Поставка одноразовых медицинских перчаток для нужд городской клинической больницы",
        "region": "Воронежская область",
        "nmck": 4850000,
        "security_bid": 48500,
        "security_contract": 242500,
        "contract_guarantee_percent": 5,
        "delivery_place": "г. Воронеж, склад заказчика",
        "delivery_period": "с даты заключения контракта по 30 ноября 2026 года, партиями по заявкам заказчика",
        "payment_terms": "оплата в течение 45 рабочих дней после приемки",
        "procurement_method": "электронный аукцион",
        "tender_summary": [
            "Требуется поставка одноразовых медицинских перчаток несколькими партиями по заявкам заказчика.",
            "Поставка осуществляется на склад заказчика за счет поставщика.",
            "Оплата отсроченная, после приемки товара.",
            "Необходимо соблюдение требований к регистрационным документам и подтверждению качества."
        ],
        "all_requirements": [
            "Наличие опыта поставки медицинских расходных материалов.",
            "Поставка партиями по заявкам заказчика в течение срока действия контракта.",
            "Доставка, разгрузка и сопутствующие расходы за счет поставщика.",
            "Предоставление регистрационных удостоверений и документов о качестве.",
            "Остаточный срок годности продукции не менее 70% на дату поставки.",
            "Соблюдение требований к упаковке, маркировке и транспортировке.",
            "Готовность к замене некачественного товара в короткий срок.",
            "Наличие достаточного товарного запаса для оперативных поставок."
        ],
        "key_requirements": [
            "Поставка по заявкам",
            "Медицинская продукция с подтверждающими документами",
            "Отсрочка платежа 45 рабочих дней",
            "Доставка за счет поставщика",
            "Остаточный срок годности не менее 70%"
        ]
    },
    "company_profile": {
        "company_name": "ООО МедСнабРегион",
        "industry": "Поставка медицинских изделий и расходных материалов",
        "experience_years": 4,
        "employees": 18,
        "annual_revenue": 26500000,
        "regions_of_operation": ["Воронежская область", "Липецкая область", "Белгородская область"],
        "completed_contracts": [
            "Поставка расходных материалов в районную больницу на 1.8 млн руб.",
            "Поставка перчаток и масок в частную клинику на 2.4 млн руб.",
            "Поставка одноразовых изделий для стационара на 3.1 млн руб."
        ],
        "financial_state": {
            "has_credit_line": True,
            "available_working_capital": 900000,
            "tax_debts": False
        },
        "logistics": {
            "own_transport": False,
            "partner_logistics": True,
            "warehouse": True
        },
        "certifications": [
            "Регистрационные документы от производителей",
            "Декларации соответствия по части ассортимента"
        ],
        "known_limitations": [
            "Небольшой собственный оборотный капитал",
            "Нет собственного автотранспорта",
            "Часть ассортимента поставляется через дистрибьюторов"
        ]
    },
    "missing_data": [
        "Не подтвержден полный ассортимент и наличие на складе по всем позициям.",
        "Неясно, выдержит ли компания длительную отсрочку оплаты без кассового разрыва.",
        "Нет точных данных о сроках замены брака и резервном запасе.",
        "Не подтверждены условия логистики в пиковые периоды."
    ],
    "task_type": "tender_company_analysis"
}

EXPECTED_TOP_KEYS = [
    "summary",
    "company_fit",
    "risks",
    "pitfalls",
    "recommendations",
    "loss_estimate",
    "final_decision",
]

ALLOWED_COMPANY_FIT_LABELS = {"fit", "conditional_fit", "not_fit"}
ALLOWED_FINAL_DECISION_LABELS = {"bid", "conditional_bid", "no_bid"}


def choose_dtype():
    if torch.cuda.is_available():
        if torch.cuda.is_bf16_supported():
            return torch.bfloat16
        return torch.float16
    return torch.float32


def get_model_device(model):
    try:
        return next(model.parameters()).device
    except StopIteration:
        return torch.device("cuda" if torch.cuda.is_available() else "cpu")


def load_model_and_tokenizer():
    dtype = choose_dtype()

    model_kwargs = {
        "trust_remote_code": TRUST_REMOTE_CODE,
        "low_cpu_mem_usage": True,
    }

    if torch.cuda.is_available():
        model_kwargs["device_map"] = "auto"
        model_kwargs["dtype"] = dtype
    else:
        model_kwargs["device_map"] = None
        model_kwargs["dtype"] = torch.float32

    if LOAD_IN_4BIT:
        if not torch.cuda.is_available():
            raise ValueError("LOAD_IN_4BIT=True, но CUDA недоступна")
        compute_dtype = torch.bfloat16 if torch.cuda.is_bf16_supported() else torch.float16
        model_kwargs["quantization_config"] = BitsAndBytesConfig(
            load_in_4bit=True,
            bnb_4bit_quant_type="nf4",
            bnb_4bit_use_double_quant=True,
            bnb_4bit_compute_dtype=compute_dtype,
        )

    peft_config = PeftConfig.from_pretrained(ADAPTER_PATH)
    base_model_name = getattr(peft_config, "base_model_name_or_path", None) or BASE_MODEL

    tokenizer = AutoTokenizer.from_pretrained(
        base_model_name,
        use_fast=True,
        trust_remote_code=TRUST_REMOTE_CODE,
    )

    if tokenizer.chat_template is None:
        raise ValueError(f"У tokenizer для модели {base_model_name} нет chat_template")

    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    base_model = AutoModelForCausalLM.from_pretrained(base_model_name, **model_kwargs)
    model = PeftModel.from_pretrained(base_model, ADAPTER_PATH)
    model.eval()
    model.config.use_cache = True

    return model, tokenizer


def try_parse_json(text: str):
    text = text.strip()
    if not text:
        return None, "Пустой вывод"

    try:
        return json.loads(text), None
    except Exception:
        start = text.find("{")
        end = text.rfind("}")
        if start != -1 and end != -1 and end > start:
            chunk = text[start:end + 1]
            try:
                return json.loads(chunk), None
            except Exception as e:
                return None, f"JSON parse error: {e}"
        return None, "В выводе не найден JSON-объект"


def validate_output(obj):
    errors = []

    if not isinstance(obj, dict):
        return ["Вывод не является JSON-объектом"]

    missing = [k for k in EXPECTED_TOP_KEYS if k not in obj]
    extra = [k for k in obj.keys() if k not in EXPECTED_TOP_KEYS]

    if missing:
        errors.append(f"Отсутствуют верхнеуровневые ключи: {missing}")
    if extra:
        errors.append(f"Лишние верхнеуровневые ключи: {extra}")

    company_fit = obj.get("company_fit")
    if isinstance(company_fit, dict):
        label = company_fit.get("label")
        if label not in ALLOWED_COMPANY_FIT_LABELS:
            errors.append(
                f"Некорректный company_fit.label: {label!r}. "
                f"Ожидалось одно из {sorted(ALLOWED_COMPANY_FIT_LABELS)}"
            )
    else:
        errors.append("company_fit отсутствует или не является объектом")

    final_decision = obj.get("final_decision")
    if isinstance(final_decision, dict):
        label = final_decision.get("label")
        if label not in ALLOWED_FINAL_DECISION_LABELS:
            errors.append(
                f"Некорректный final_decision.label: {label!r}. "
                f"Ожидалось одно из {sorted(ALLOWED_FINAL_DECISION_LABELS)}"
            )
    else:
        errors.append("final_decision отсутствует или не является объектом")

    if "pitfalls" in obj and not isinstance(obj["pitfalls"], list):
        errors.append("pitfalls должен быть массивом")
    if "recommendations" in obj and not isinstance(obj["recommendations"], list):
        errors.append("recommendations должен быть массивом")
    if "risks" in obj and not isinstance(obj["risks"], list):
        errors.append("risks должен быть массивом")
    if "loss_estimate" in obj and not isinstance(obj["loss_estimate"], dict):
        errors.append("loss_estimate должен быть объектом")

    return errors


def main():
    model, tokenizer = load_model_and_tokenizer()

    # Добавляем prefill для assistant, чтобы модель продолжала JSON.
    messages = [
        {"role": "system", "content": INSTRUCTION},
        {"role": "user", "content": json.dumps(INPUT_DATA, ensure_ascii=False, indent=2)},
        {"role": "assistant", "content": "{"},
    ]

    tokenized = tokenizer.apply_chat_template(
        messages,
        tokenize=True,
        return_dict=True,
        return_tensors="pt",
        continue_final_message=True,
    )

    device = get_model_device(model)
    tokenized = {k: v.to(device) for k, v in tokenized.items()}

    gen_kwargs = {
        "max_new_tokens": MAX_NEW_TOKENS,
        "pad_token_id": tokenizer.pad_token_id,
        "eos_token_id": tokenizer.eos_token_id,
        "do_sample": TEMPERATURE > 0,
    }

    if TEMPERATURE > 0:
        gen_kwargs["temperature"] = TEMPERATURE
        gen_kwargs["top_p"] = TOP_P

    with torch.no_grad():
        output_ids = model.generate(**tokenized, **gen_kwargs)

    prompt_len = tokenized["input_ids"].shape[1]
    generated_ids = output_ids[0][prompt_len:]
    generated_continuation = tokenizer.decode(generated_ids, skip_special_tokens=True).strip()

    # Полный JSON = prefill "{" + продолжение модели
    generated_text = "{" + generated_continuation

    parsed, parse_error = try_parse_json(generated_text)

    print("\n=== INPUT ===")
    print(json.dumps(INPUT_DATA, ensure_ascii=False, indent=2))

    print("\n=== MODEL OUTPUT ===")
    print(generated_text)

    print("\n=== PARSE STATUS ===")
    if parse_error:
        print(f"JSON invalid: {parse_error}")
    else:
        print("JSON valid")

    if parsed is not None:
        print("\n=== SCHEMA CHECK ===")
        validation_errors = validate_output(parsed)
        if validation_errors:
            for err in validation_errors:
                print(f"- {err}")
        else:
            print("Схема соблюдена")


if __name__ == "__main__":
    main()