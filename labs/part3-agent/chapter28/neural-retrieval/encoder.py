#!/usr/bin/env python3
# coding: utf-8
"""Local fixed-revision ONNX embedding. No downloads, remote code or dynamic executable selection."""
import hashlib
import json
import sys
from pathlib import Path

MAX_INPUT = 128 << 10
MAX_TEXTS = 32


def assets(directory):
    expected = json.loads(Path(__file__).with_name("assets-lock.json").read_text())
    root = Path(directory).resolve()
    for item in expected["files"]:
        file = root / item["path"]
        if file.is_symlink() or file.stat().st_size != item["bytes"]:
            raise ValueError("asset shape mismatch")
        digest = hashlib.sha256()
        with file.open("rb") as source:
            for chunk in iter(lambda: source.read(1 << 20), b""):
                digest.update(chunk)
        if digest.hexdigest() != item["sha256"]:
            raise ValueError("asset checksum mismatch")
    return root, expected


def encode(directory, texts):
    if not isinstance(texts, list) or not 0 < len(texts) <= MAX_TEXTS:
        raise ValueError("invalid batch")
    if any(not isinstance(t, str) or not t.strip() or len(t.encode("utf-8")) > 4096 for t in texts):
        raise ValueError("invalid text")
    root, config = assets(directory)
    import numpy as np
    import onnxruntime as ort
    from tokenizers import Tokenizer
    tokenizer = Tokenizer.from_file(str(root / "tokenizer.json"))
    tokenizer.no_truncation()
    tokenizer.no_padding()
    uncut = tokenizer.encode_batch(texts)
    truncated = [len(row.ids) > config["max_tokens"] for row in uncut]
    tokenizer.enable_truncation(max_length=config["max_tokens"])
    tokenizer.enable_padding(pad_id=0, pad_token="[PAD]")
    encoded = tokenizer.encode_batch(texts)
    batch = {"input_ids": np.array([row.ids for row in encoded], dtype=np.int64),
             "attention_mask": np.array([row.attention_mask for row in encoded], dtype=np.int64),
             "token_type_ids": np.array([row.type_ids for row in encoded], dtype=np.int64)}
    options = ort.SessionOptions()
    options.intra_op_num_threads = 1
    options.inter_op_num_threads = 1
    session = ort.InferenceSession(str(root / "onnx/model.onnx"), sess_options=options,
                                   providers=["CPUExecutionProvider"])
    feed = {input.name: batch[input.name] for input in session.get_inputs()}
    outputs = session.run(None, feed)
    hidden = outputs[0]
    if hidden.ndim != 3 or hidden.shape[0] != len(texts) or hidden.shape[2] != config["dimension"]:
        raise ValueError("unexpected encoder output")
    mask = batch["attention_mask"][..., None].astype(hidden.dtype)
    pooled = (hidden * mask).sum(axis=1) / np.maximum(mask.sum(axis=1), 1)
    norms = np.linalg.norm(pooled, axis=1, keepdims=True)
    if not np.all(np.isfinite(pooled)) or np.any(norms <= 0):
        raise ValueError("invalid embedding")
    normalized = pooled / norms
    return {"revision": config["revision"], "space": "minilm-l6-v2-onnx-mean-l2-256-v1",
            "dimension": config["dimension"], "truncated": truncated,
            "vectors": normalized.tolist()}


def main():
    if len(sys.argv) != 2:
        raise ValueError("fixed model directory required")
    raw = sys.stdin.buffer.read(MAX_INPUT + 1)
    if len(raw) > MAX_INPUT:
        raise ValueError("input too large")
    request = json.loads(raw)
    if set(request) != {"texts"}:
        raise ValueError("invalid request")
    print(json.dumps(encode(sys.argv[1], request["texts"]), allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("local embedding failed", file=sys.stderr)
        sys.exit(1)
