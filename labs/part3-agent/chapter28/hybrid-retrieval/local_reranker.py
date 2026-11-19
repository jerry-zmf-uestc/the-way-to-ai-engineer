"""Bounded original wrapper for a pinned local cross-encoder."""
import hashlib
import importlib.metadata
import json
import math
import os
import signal
from pathlib import Path
import sys
import time


def run(directory, requests):
    if sys.version_info[:3] != (3, 12, 14):
        raise ValueError('locked Python required')
    for line in Path(__file__).with_name('reranker-requirements.lock').read_text().splitlines():
        name, version = line.split('==')
        if importlib.metadata.version(name) != version:
            raise ValueError('runtime mismatch')
    lock = json.loads(Path(__file__).with_name('reranker-lock.json').read_text())
    root = Path(directory).resolve()
    for item in lock['files']:
        file = root / item['path']
        if file.is_symlink() or file.stat().st_size != item['bytes']:
            raise ValueError('asset mismatch')
        h = hashlib.sha256()
        with file.open('rb') as source:
            for chunk in iter(lambda: source.read(1 << 20), b''):
                h.update(chunk)
        if h.hexdigest() != item['sha256']:
            raise ValueError('asset hash')
    if not isinstance(requests, list) or not 0 < len(requests) <= 6:
        raise ValueError('batch bound')
    import torch
    from transformers import AutoTokenizer, AutoModelForSequenceClassification
    torch.set_num_threads(1)
    tokenizer = AutoTokenizer.from_pretrained(root, local_files_only=True, trust_remote_code=False)
    model = AutoModelForSequenceClassification.from_pretrained(root, local_files_only=True,
               trust_remote_code=False, use_safetensors=True).to('cpu').eval()
    rows = []
    for request in requests:
        if set(request) != {'query', 'candidates'} or not isinstance(request['query'], str) or not 0 < len(request['query'].encode()) <= 4096:
            raise ValueError('request shape')
        candidates = request['candidates']
        if not isinstance(candidates, list) or not 0 < len(candidates) <= 8:
            raise ValueError('candidate bound')
        seen = set()
        for candidate in candidates:
            if set(candidate) != {'id', 'text'} or not isinstance(candidate['id'], str) or not candidate['id'] or len(candidate['id']) > 128 or candidate['id'] in seen or not isinstance(candidate['text'], str) or not 0 < len(candidate['text'].encode()) <= 4096:
                raise ValueError('candidate shape')
            seen.add(candidate['id'])
        lengths = [len(tokenizer(request['query'], c['text'], truncation=False)['input_ids']) for c in candidates]
        if any(length > 512 for length in lengths):
            raise ValueError('pair token bound')
        inputs = tokenizer([request['query']] * len(candidates), [c['text'] for c in candidates],
                           padding=True, truncation=False, return_tensors='pt')
        start = time.monotonic()
        with torch.inference_mode():
            logits = model(**inputs).logits
        if logits.shape != (len(candidates), 1):
            raise ValueError('score shape')
        scores = logits[:, 0].tolist()
        if not all(math.isfinite(score) for score in scores):
            raise ValueError('nonfinite score')
        rows.append({'scores': [{'id': c['id'], 'score': score, 'tokens': length}
                    for c, score, length in zip(candidates, scores, lengths)],
                    'seconds': time.monotonic() - start})
    return {'revision': lock['revision'], 'runtime': {'device': 'cpu', 'threads': 1, 'dtype': str(next(model.parameters()).dtype)}, 'rows': rows}


if __name__ == '__main__':
    signal.alarm(60)
    try:
        if os.environ.get('RERANK_E2E') != '1' or len(sys.argv) != 2:
            raise ValueError('explicit local rerank required')
        raw = sys.stdin.buffer.read((64 << 10) + 1)
        if len(raw) > 64 << 10:
            raise ValueError('input bound')
        print(json.dumps(run(sys.argv[1], json.loads(raw)), allow_nan=False))
    except Exception:
        print('local rerank failed', file=sys.stderr)
        sys.exit(1)
