import json
import os
import re
import threading
import logging
from typing import Any

import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field, ConfigDict
from peft import PeftConfig, PeftModel
from transformers import AutoModelForCausalLM, AutoTokenizer, BitsAndBytesConfig

logger = logging.getLogger("uvicorn.error")

MODEL_REPO_ID = (os.getenv("MODEL_REPO_ID") or "shishk1n/ONS_TEN").strip()
ADAPTER_SUBFOLDER = (os.getenv("ADAPTER_SUBFOLDER") or "qwen_output").strip()
BASE_MODEL = (os.getenv("BASE_MODEL") or "").strip()

HF_TOKEN = os.getenv("HF_TOKEN")
if HF_TOKEN is not None:
    HF_TOKEN = HF_TOKEN.strip()
if not HF_TOKEN:
    HF_TOKEN = None

TRUST_REMOTE_CODE = os.getenv("TRUST_REMOTE_CODE", "true").lower() == "true"
USE_4BIT = os.getenv("USE_4BIT", "true").lower() == "true"
MAX_NEW_TOKENS = int(os.getenv("MAX_NEW_TOKENS", "2048"))
TEMPERATURE = float(os.getenv("TEMPERATURE", "0.0"))
TOP_P = float(os.getenv("TOP_P", "0.95"))
STARTUP_LOAD = os.getenv("STARTUP_LOAD", "true").lower() == "true"
JSON_RETRY = os.getenv("JSON_RETRY", "true").lower() == "true"

_model = None
_tokenizer = None
_model_lock = threading.Lock()
_model_info: dict[str, Any] = {}


class AnalyzeRequest(BaseModel):
    instruction: str = Field(default="")
    input: dict[str, Any]
    max_new_tokens: int | None = None


class AnalyzeResponse(BaseModel):
    model_config = ConfigDict(protected_namespaces=())
    output: dict[str, Any]
    model_info: dict[str, Any]


def snake_case(name: str) -> str:
    if not isinstance(name, str):
        return name
    name = name.replace("-", "_")
    s1 = re.sub(r"(.)([A-Z][a-z]+)", r"\1_\2", name)
    s2 = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", s1)
    return s2.lower()


def camel_case(name: str) -> str:
    if not isinstance(name, str) or "_" not in name:
        return name
    parts = name.split("_")
    return parts[0] + "".join(part.capitalize() for part in parts[1:])


def transform_keys(obj: Any, key_fn) -> Any:
    if isinstance(obj, dict):
        return {key_fn(k): transform_keys(v, key_fn) for k, v in obj.items()}
    if isinstance(obj, list):
        return [transform_keys(v, key_fn) for v in obj]
    return obj


def normalize_input_for_model(payload: dict[str, Any]) -> dict[str, Any]:
    return transform_keys(payload, snake_case)


def normalize_output_for_server(payload: dict[str, Any]) -> dict[str, Any]:
    payload = transform_keys(payload, camel_case)

    if "company_fit" in payload and "companyFit" not in payload:
        payload["companyFit"] = payload.pop("company_fit")
    if "loss_estimate" in payload and "lossEstimate" not in payload:
        payload["lossEstimate"] = payload.pop("loss_estimate")
    if "final_decision" in payload and "finalDecision" not in payload:
        payload["finalDecision"] = payload.pop("final_decision")

    company_fit = payload.get("companyFit")
    if isinstance(company_fit, dict):
        if "status_comment" in company_fit and "statusComment" not in company_fit:
            company_fit["statusComment"] = company_fit.pop("status_comment")
        company_fit = transform_keys(company_fit, camel_case)
        payload["companyFit"] = company_fit

    loss_estimate = payload.get("lossEstimate")
    if isinstance(loss_estimate, dict):
        loss_estimate = transform_keys(loss_estimate, camel_case)
        payload["lossEstimate"] = loss_estimate

    final_decision = payload.get("finalDecision")
    if isinstance(final_decision, dict):
        if "decision_comment" in final_decision and "decisionComment" not in final_decision:
            final_decision["decisionComment"] = final_decision.pop("decision_comment")
        final_decision = transform_keys(final_decision, camel_case)
        payload["finalDecision"] = final_decision

    return payload


def build_strict_instruction(user_instruction: str) -> str:
    extra = (
        "Верни только один валидный JSON-объект без markdown и без пояснений. "
        "Используй именно поля: summary, company_fit, risks, pitfalls, recommendations, loss_estimate, final_decision. "
        "Для company_fit.label используй только fit, conditional_fit, not_fit. "
        "Для final_decision.label используй только bid, conditional_bid, no_bid. "
        "Для loss_estimate.label по возможности используй scenario_only."
    )
    base = (user_instruction or "Ты — тендерный риск-менеджер.").strip()
    return f"{base} {extra}".strip()


def choose_dtype() -> torch.dtype:
    if torch.cuda.is_available():
        if torch.cuda.is_bf16_supported():
            return torch.bfloat16
        return torch.float16
    return torch.float32


def get_model_and_tokenizer():
    global _model, _tokenizer, _model_info

    if _model is not None and _tokenizer is not None:
        return _model, _tokenizer

    with _model_lock:
        if _model is not None and _tokenizer is not None:
            return _model, _tokenizer

        hf_token = (HF_TOKEN or "").strip() or None
        adapter_subfolder = (ADAPTER_SUBFOLDER or "").strip()

        if not MODEL_REPO_ID:
            raise RuntimeError("MODEL_REPO_ID пустой")
        if not adapter_subfolder:
            raise RuntimeError("ADAPTER_SUBFOLDER пустой")

        if not torch.cuda.is_available():
            raise RuntimeError(
                "CUDA недоступна внутри контейнера llm. "
                "Проверь Docker Desktop + WSL2 + NVIDIA GPU support + gpus: all."
            )

        peft_kwargs: dict[str, Any] = {}
        if hf_token is not None:
            peft_kwargs["token"] = hf_token

        peft_config = PeftConfig.from_pretrained(
            MODEL_REPO_ID,
            subfolder=adapter_subfolder,
            **peft_kwargs,
        )

        base_model_name = (BASE_MODEL or "").strip() or peft_config.base_model_name_or_path
        if not base_model_name:
            raise RuntimeError("Не удалось определить base model из adapter_config.json")

        dtype = choose_dtype()

        model_kwargs: dict[str, Any] = {
            "trust_remote_code": TRUST_REMOTE_CODE,
            "low_cpu_mem_usage": True,
            "device_map": "auto",
            "max_memory": {0: "10GiB", "cpu": "24GiB"},
            "torch_dtype": dtype,
        }

        if hf_token is not None:
            model_kwargs["token"] = hf_token

        if USE_4BIT:
            compute_dtype = torch.bfloat16 if torch.cuda.is_bf16_supported() else torch.float16
            model_kwargs["quantization_config"] = BitsAndBytesConfig(
                load_in_4bit=True,
                bnb_4bit_quant_type="nf4",
                bnb_4bit_use_double_quant=True,
                bnb_4bit_compute_dtype=compute_dtype,
            )

        tokenizer_kwargs: dict[str, Any] = {
            "use_fast": False,
            "trust_remote_code": TRUST_REMOTE_CODE,
        }
        if hf_token is not None:
            tokenizer_kwargs["token"] = hf_token

        tokenizer = AutoTokenizer.from_pretrained(
            base_model_name,
            **tokenizer_kwargs,
        )

        if tokenizer.pad_token is None:
            tokenizer.pad_token = tokenizer.eos_token

        base_model = AutoModelForCausalLM.from_pretrained(
            base_model_name,
            **model_kwargs,
        )

        model = PeftModel.from_pretrained(
            base_model,
            MODEL_REPO_ID,
            subfolder=adapter_subfolder,
            **peft_kwargs,
        )

        model.eval()

        if hasattr(model, "generation_config"):
            model.generation_config.temperature = None
            model.generation_config.top_p = None
            model.generation_config.top_k = None

        _model = model
        _tokenizer = tokenizer
        _model_info = {
            "repo_id": MODEL_REPO_ID,
            "adapter_subfolder": adapter_subfolder,
            "base_model": base_model_name,
            "cuda": torch.cuda.is_available(),
            "device": str(next(model.parameters()).device),
            "dtype": str(dtype),
            "use_4bit": bool(USE_4BIT),
            "hf_token_set": hf_token is not None,
        }

        return _model, _tokenizer


def get_device(model) -> torch.device:
    try:
        return next(model.parameters()).device
    except Exception:
        return torch.device("cuda" if torch.cuda.is_available() else "cpu")


def try_parse_json(text: str) -> dict[str, Any]:
    text = text.strip()
    if not text:
        raise ValueError("Пустой вывод модели")

    text = re.sub(r"^```json\s*", "", text)
    text = re.sub(r"^```\s*", "", text)
    text = re.sub(r"\s*```$", "", text)

    try:
        parsed = json.loads(text)
        if isinstance(parsed, dict):
            return parsed
    except Exception as e:
        logger.error("FULL JSON PARSE FAILED: %s", e)

    start = text.find("{")
    end = text.rfind("}")
    if start != -1 and end != -1 and end > start:
        chunk = text[start:end + 1]
        try:
            parsed = json.loads(chunk)
            if isinstance(parsed, dict):
                return parsed
        except Exception as e:
            logger.error("CHUNK JSON PARSE FAILED: %s", e)
            logger.error("BROKEN JSON CHUNK START")
            logger.error(chunk)
            logger.error("BROKEN JSON CHUNK END")
            raise

    raise ValueError("Не удалось распарсить JSON из ответа модели")


def generate_once(model, tokenizer, instruction: str, normalized_input: dict[str, Any], max_new_tokens: int) -> dict[str, Any]:
    messages = [
        {"role": "system", "content": instruction},
        {"role": "user", "content": json.dumps(normalized_input, ensure_ascii=False, indent=2)},
        {"role": "assistant", "content": "{"},
    ]

    tokenized = tokenizer.apply_chat_template(
        messages,
        tokenize=True,
        return_dict=True,
        return_tensors="pt",
        continue_final_message=True,
    )

    device = get_device(model)
    tokenized = {k: v.to(device) for k, v in tokenized.items()}

    gen_kwargs = {
        "max_new_tokens": max_new_tokens,
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
    generated_text = "{" + generated_continuation

    logger.info("RAW MODEL OUTPUT START")
    logger.info(generated_text)
    logger.info("RAW MODEL OUTPUT END")

    return try_parse_json(generated_text)


def run_analysis(instruction: str, payload: dict[str, Any], max_new_tokens: int) -> dict[str, Any]:
    model, tokenizer = get_model_and_tokenizer()
    strict_instruction = build_strict_instruction(instruction)
    normalized_input = normalize_input_for_model(payload)

    try:
        parsed = generate_once(model, tokenizer, strict_instruction, normalized_input, max_new_tokens)
    except Exception:
        if not JSON_RETRY:
            raise
        retry_instruction = strict_instruction + " Повтори: ответ должен быть строго JSON-объектом и начинаться с символа { ."
        parsed = generate_once(model, tokenizer, retry_instruction, normalized_input, max_new_tokens)

    return normalize_output_for_server(parsed)

def hf_auth_kwargs() -> dict[str, Any]:
    return {"token": HF_TOKEN} if HF_TOKEN else {}

app = FastAPI(title="Tender LLM Service", version="1.0.0")


@app.on_event("startup")
def startup_event():
    if STARTUP_LOAD:
        get_model_and_tokenizer()


@app.get("/health")
def health():
    loaded = _model is not None and _tokenizer is not None
    return {
        "status": "ok",
        "model_loaded": loaded,
        "model_info": _model_info,
    }


@app.post("/analyze", response_model=AnalyzeResponse)
def analyze(req: AnalyzeRequest):
    try:
        output = run_analysis(req.instruction, req.input, req.max_new_tokens or MAX_NEW_TOKENS)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    return AnalyzeResponse(output=output, model_info=_model_info)
