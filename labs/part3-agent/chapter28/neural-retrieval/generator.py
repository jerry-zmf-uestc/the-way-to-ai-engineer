#!/usr/bin/env python3
# coding: utf-8
"""Fixed local model for a query-only hypothetical passage; output is never evidence."""
import hashlib
import json
import sys
from pathlib import Path

PROMPT = "Write one short technical passage that could answer the question. This is hypothetical text for a search query, not verified evidence. Do not cite sources."


def verify(directory):
    config = json.loads(Path(__file__).with_name("generator-lock.json").read_text())
    root = Path(directory).resolve()
    for item in config["files"]:
        file = root / item["path"]
        if file.is_symlink() or file.stat().st_size != item["bytes"]:
            raise ValueError("asset shape mismatch")
        digest = hashlib.sha256()
        with file.open("rb") as source:
            for chunk in iter(lambda: source.read(1 << 20), b""):
                digest.update(chunk)
        if digest.hexdigest() != item["sha256"]:
            raise ValueError("asset checksum mismatch")
    return root, config


def generate(directory, questions):
    if not isinstance(questions, list) or not 0 < len(questions) <= 8:
        raise ValueError("invalid questions")
    if any(not isinstance(q, str) or not q.strip() or len(q.encode("utf-8")) > 4096 for q in questions):
        raise ValueError("invalid question")
    root, config = verify(directory)
    import torch
    from transformers import AutoTokenizer, AutoModelForCausalLM
    torch.set_num_threads(1)
    tokenizer = AutoTokenizer.from_pretrained(str(root), local_files_only=True, trust_remote_code=False)
    model = AutoModelForCausalLM.from_pretrained(str(root), local_files_only=True,
                                               trust_remote_code=False, use_safetensors=True).to("cpu").eval()
    rows = []
    for question in questions:
        messages = [{"role": "system", "content": PROMPT}, {"role": "user", "content": question}]
        prompt = tokenizer.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
        inputs = tokenizer(prompt, return_tensors="pt", add_special_tokens=False)
        count = inputs["input_ids"].shape[1]
        if count > config["max_input_tokens"]:
            raise ValueError("input token bound exceeded")
        with torch.inference_mode():
            output = model.generate(**inputs, max_new_tokens=config["max_new_tokens"],
                                    do_sample=False, use_cache=True,
                                    pad_token_id=tokenizer.eos_token_id)
        suffix = output[0, count:]
        text = tokenizer.decode(suffix, skip_special_tokens=True).strip()
        if not text or len(text.encode("utf-8")) > 4096:
            raise ValueError("invalid hypothetical output")
        rows.append({"text": text, "input_tokens": count, "output_tokens": len(suffix),
                     "at_output_limit": len(suffix) >= config["max_new_tokens"], "trust": "hypothetical"})
    return {"revision": config["revision"], "prompt_version": config["prompt_version"], "rows": rows}


def main():
    raw = sys.stdin.buffer.read((64 << 10) + 1)
    if len(raw) > 64 << 10 or len(sys.argv) != 2:
        raise ValueError("input/asset bound")
    request = json.loads(raw)
    if set(request) != {"questions"}:
        raise ValueError("unexpected payload")
    print(json.dumps(generate(sys.argv[1], request["questions"]), allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("local hypothetical generation failed", file=sys.stderr)
        sys.exit(1)
