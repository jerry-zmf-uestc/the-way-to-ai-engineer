"""Application integration of fixed public encoding CLI and local rerank CLI."""
import hashlib
import json
import math
import os
from pathlib import Path
import signal
import subprocess
import sys


def invoke(python, script, assets, payload):
    if not Path(python).is_absolute() or not Path(assets).is_absolute():
        raise ValueError('trusted absolute runtime/asset paths required')
    result = subprocess.run([python, str(script), assets], input=json.dumps(payload),
                            capture_output=True, text=True, timeout=65)
    if result.returncode or len(result.stdout.encode()) > 128 << 10:
        raise ValueError('bounded local stage failed')
    return json.loads(result.stdout)


def compare():
    if os.environ.get('EMBEDDING_E2E') != '1' or os.environ.get('RERANK_E2E') != '1':
        raise ValueError('both explicit local stages required')
    root = Path(__file__).resolve().parent
    data = (root / 'rerank-cases.json').read_bytes()
    dataset = json.loads(data)
    encoder = root.parent / 'neural-retrieval' / 'encoder.py'
    texts = []
    for case in dataset['cases']:
        texts.append(case['query'])
        texts.extend(c['text'] for c in case['candidates'])
    encoded = invoke(os.environ['EMBEDDING_PYTHON'], encoder, os.environ['EMBEDDING_ASSETS'], {'texts': texts})
    if encoded['revision'] != '1110a243fdf4706b3f48f1d95db1a4f5529b4d41' or encoded['space'] != 'minilm-l6-v2-onnx-mean-l2-256-v1' or encoded['dimension'] != 384 or len(encoded['vectors']) != len(texts) or len(encoded['truncated']) != len(texts) or any(encoded['truncated']):
        raise ValueError('encoding batch identity or completeness')
    for vector in encoded['vectors']:
        if len(vector) != 384 or not all(type(x) in (float, int) and math.isfinite(x) for x in vector) or not 0.999 < sum(x*x for x in vector) < 1.001:
            raise ValueError('encoding shape or norm')
    requests = [{k: case[k] for k in ('query', 'candidates')} for case in dataset['cases']]
    reranked = invoke(os.environ['RERANK_PYTHON'], root / 'local_reranker.py', os.environ['RERANK_ASSETS'], requests)
    if reranked['revision'] != '233902d25c440f23af6f7d6e94d2946bac0bee0a' or len(reranked['rows']) != len(requests):
        raise ValueError('rerank identity')
    rows, offset = [], 0
    for case, scores in zip(dataset['cases'], reranked['rows']):
        query = encoded['vectors'][offset]
        offset += 1
        candidates = case['candidates']
        dense = []
        for c in candidates:
            vector = encoded['vectors'][offset]
            offset += 1
            dense.append({'id': c['id'], 'score': sum(x*y for x, y in zip(query, vector))})
        expected = {c['id'] for c in candidates}
        pairs = scores['scores']
        if len(pairs) != len(expected) or {p['id'] for p in pairs} != expected or any(not math.isfinite(p['score']) or not 1 <= p['tokens'] <= 512 for p in pairs):
            raise ValueError('rerank candidate mismatch')
        rows.append({'case_id': case['id'], 'dense_scores': dense,
                     'dense_order': [p['id'] for p in sorted(dense, key=lambda p: (-p['score'], p['id']))],
                     'rerank_order': [p['id'] for p in sorted(pairs, key=lambda p: (-p['score'], p['id']))]})
    return {'case_version': dataset['version'], 'case_sha256': hashlib.sha256(data).hexdigest(),
            'encoder': encoded, 'reranker': reranked, 'rows': rows}


if __name__ == '__main__':
    signal.alarm(150)
    try:
        print(json.dumps(compare(), allow_nan=False))
    except Exception:
        print('local vector/rerank comparison failed', file=sys.stderr)
        sys.exit(1)
