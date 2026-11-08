"""Original local summary experiment. Candidate facts never establish their own truth."""
import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time


def views(case):
    # The execution projection contains no authoritative Snapshot or scoring labels.
    base = {'snapshot_version': case['snapshot_version'],
            'task': 'Preserve the current budget in CNY, mandatory offline requirement, and option B offline knowledge state.'}
    return [(mode, dict(base, history=case['history'] if mode == 'full' else case['history'][-3:]))
            for mode in ('full', 'recent')]


def load_model(directory):
    lock = json.loads(Path(__file__).with_name('model-lock.json').read_text())
    root = Path(directory).resolve()
    for item in lock['files']:
        path = root / item['path']
        if path.is_symlink() or path.stat().st_size != item['bytes']:
            raise ValueError('asset identity')
        digest = hashlib.sha256()
        with path.open('rb') as stream:
            for chunk in iter(lambda: stream.read(1 << 20), b''):
                digest.update(chunk)
        if digest.hexdigest() != item['sha256']:
            raise ValueError('asset checksum')
    import torch
    from transformers import AutoTokenizer, AutoModelForCausalLM
    torch.set_num_threads(1)
    tokenizer = AutoTokenizer.from_pretrained(str(root), local_files_only=True, trust_remote_code=False)
    model = AutoModelForCausalLM.from_pretrained(str(root), local_files_only=True,
                                              trust_remote_code=False, use_safetensors=True).to('cpu').eval()
    return torch, tokenizer, model, lock['revision']


def execute(case, generate, judge, checkpoint=lambda rows: None):
    rows = []
    for mode, view in views(case):
        raw, usage = generate(view)
        # No fence removal, field filling or JSON repair. Raw output remains independent evidence.
        row = dict(mode=mode, view=view, raw=raw, usage=usage, review='output_limit', validation=None)
        rows.append(row)
        checkpoint(rows)
        if not usage['at_output_limit']:
            row['validation'] = judge(raw, mode)
            row['review'] = 'see_validator'
        checkpoint(rows)
    return rows


def run(args):
    if sys.version_info[:3] != (3, 12, 14):
        raise ValueError('locked Python required')
    versions = {}
    for line in Path(__file__).with_name('generation-requirements.lock').read_text().splitlines():
        name, version = line.split('==')
        if importlib.metadata.version(name) != version:
            raise ValueError('locked dependency required')
        versions[name] = version
    case_raw = Path(__file__).with_name('compression-case.json').read_bytes()
    if len(case_raw) > 128 << 10:
        raise ValueError('dataset budget')
    case = json.loads(case_raw)
    if case['version'] != 'compression-development-v1' or case['split'] != 'development':
        raise ValueError('registered development case required')
    os.environ['HF_HUB_OFFLINE'] = os.environ['TRANSFORMERS_OFFLINE'] = '1'
    torch, tokenizer, model, revision = load_model(args.assets)
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    report = dict(complete=False, dataset=case['version'], split=case['split'],
                  case_sha256=hashlib.sha256(case_raw).hexdigest(), model_revision=revision,
                  prompt_version='compression-v1', runtime=versions, rows=[])

    def persist():
        data = json.dumps(report, ensure_ascii=False, indent=2).encode()
        if len(data) > 128 << 10:
            raise ValueError('report budget')
        temp = output.with_suffix('.tmp')
        temp.write_bytes(data)
        temp.replace(output)

    persist()
    instruction = ('Compress the supplied history into one raw JSON object with exactly Version and Facts. '
                   'Version is the supplied snapshot_version. Facts is an array of objects with exactly '
                   'Key, Value, Source, State; all four fields are strings. Keys: budget_cny, offline_required, '
                   'option_b_offline. Preserve latest user changes and unchanged mandatory constraints. '
                   'Use the ID of the history item supporting each value as Source. State is known or unknown. '
                   'For known facts, Value is the actual number or true/false string; for unknown, Value is empty. '
                   'Do not invent missing constraints or documents. No markdown or explanation.')

    def generate(view):
        prefix = json.dumps({'Version': view['snapshot_version']})[:-1] + ', "Facts": ['
        prompt = tokenizer.apply_chat_template([{'role': 'system', 'content': instruction},
                                               {'role': 'user', 'content': json.dumps(view)}],
                                              tokenize=False, add_generation_prompt=True, enable_thinking=False) + prefix
        inputs = tokenizer(prompt, return_tensors='pt', add_special_tokens=False)
        count = inputs['input_ids'].shape[1]
        if count > 1536:
            raise ValueError('input token budget')
        started = time.monotonic()
        with torch.inference_mode():
            reply = model.generate(**inputs, do_sample=False, max_new_tokens=320,
                                   use_cache=True, pad_token_id=tokenizer.eos_token_id)
        suffix = reply[0, count:]
        generated = tokenizer.decode(suffix, skip_special_tokens=True)
        return prefix + generated, dict(application_prefix=prefix, generated_text=generated,
                                       input_tokens=count, output_tokens=len(suffix),
                                       at_output_limit=len(suffix) == 320, seconds=time.monotonic() - started)

    def judge(raw, mode):
        candidate = output.with_name(output.stem + '-' + mode + '.txt')
        candidate.write_text(raw)
        process = subprocess.run([args.validator, '-summary-candidate', str(candidate)],
                                 capture_output=True, text=True, timeout=10)
        if process.returncode != 0 or len(process.stdout) > 4096:
            raise ValueError('validator failed')
        return process.stdout.strip()

    # Save each raw generation before validation or the next call.
    def checkpoint(rows):
        report['rows'] = rows
        persist()
    report['rows'] = execute(case, generate, judge, checkpoint)
    report['complete'] = True
    persist()
    print(json.dumps([dict(mode=r['mode'], usage=r['usage'], review=r['review'], validation=r['validation'])
                      for r in report['rows']], ensure_ascii=False))


if __name__ == '__main__':
    if os.environ.get('CONTEXT_COMPRESSION_E2E') != '1':
        sys.exit('Set CONTEXT_COMPRESSION_E2E=1 explicitly; default validation does not load a model.')
    parser = argparse.ArgumentParser()
    parser.add_argument('--assets', required=True)
    parser.add_argument('--validator', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    signal.signal(signal.SIGALRM, lambda *_: (_ for _ in ()).throw(TimeoutError('experiment deadline')))
    signal.alarm(180)
    run(args)
