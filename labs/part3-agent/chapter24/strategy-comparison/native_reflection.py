"""Scoped language advice from a recorded native failure, compared in next Actors."""
import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import signal
import sys
import time
from local_strategies import load_model
from model_search import persist
from self_ask_model import execute, score, registered_cases
from native_self_ask import adapt_generate


def task_fingerprint(task):
    return hashlib.sha256(json.dumps({'question':task['question'],'seed':task['seed']}).encode()).hexdigest()


def reflection_input(case, prior):
    return {'question': case['question'], 'observations': json.loads(json.dumps(prior['observations'])),
            'proposals': [call['provider_raw'] for call in prior['calls']], 'error': prior['error'],
            'interface': 'one lookup call per response; final answer/refuse are normal JSON'}


def make_memo(text, usage, scope, source_sha):
    if usage['at_output_limit'] or not text.strip() or len(text.encode()) > 1024:
        raise ValueError('reflection_incomplete')
    return {'scope': dict(scope), 'source_attempt': 1, 'valid_through_attempt': 2,
            'source_sha256': source_sha, 'advice': text, 'status': 'unverified_suggestion'}


def select_memo(memo, scope, attempt):
    if (set(memo) != {'scope','source_attempt','valid_through_attempt','source_sha256','advice','status'}
            or not isinstance(memo['advice'], str) or not memo['advice'].strip()
            or len(memo['advice'].encode()) > 1024
            or not isinstance(memo['source_sha256'], str) or len(memo['source_sha256']) != 64
            or any(c not in '0123456789abcdef' for c in memo['source_sha256'])
            or type(memo['source_attempt']) is not int or type(memo['valid_through_attempt']) is not int
            or type(attempt) is not int or not 1 <= memo['source_attempt'] <= memo['valid_through_attempt'] <= 16):
        raise ValueError('invalid_suggestion')
    if (memo['scope'] != scope or memo['status'] != 'unverified_suggestion'
            or not memo['source_attempt'] < attempt <= memo['valid_through_attempt']):
        raise ValueError('inapplicable_suggestion')
    if len(json.dumps(memo).encode()) > 4096:
        raise ValueError('suggestion_budget')
    return json.loads(json.dumps(memo))


def actor(task, complete, memo, scope):
    if memo is not None and scope.get('task_fingerprint') != task_fingerprint(task):
        raise ValueError('task_scope_mismatch')
    selected = select_memo(memo, scope, 2) if memo is not None else None
    def generate(view, _):
        projected = json.loads(json.dumps(view))
        if selected is not None:
            projected['prior_suggestion'] = selected
        return adapt_generate(projected, complete, 'v2')
    return execute(task, 'self-ask', generate, repair_protocol=True, expose_budget=True)


def main(args):
    if sys.version_info[:3] != (3, 12, 14):
        raise ValueError('locked Python required')
    root = Path(__file__).parent
    for line in (root/'generation-requirements.lock').read_text().splitlines():
        name, version = line.split('==')
        if importlib.metadata.version(name) != version:
            raise ValueError('locked dependency required')
    raw_cases, dataset = registered_cases('independent-v1')
    prior_bytes = (root/'testdata/native-independent-workflow-v1-result.json').read_bytes()
    saved = json.loads(prior_bytes)
    if not saved['complete'] or saved['case_sha256'] != hashlib.sha256(raw_cases).hexdigest():
        raise ValueError('source identity')
    case = next(c for c in dataset['cases'] if c['id'] == 'equally-current-conflict')
    prior = next(r for r in saved['rows'] if r['case_id'] == case['id'] and r['mode'] == 'self-ask')
    task = {k: case[k] for k in ('id', 'question', 'seed', 'corpus')}
    os.environ['HF_HUB_OFFLINE'] = os.environ['TRANSFORMERS_OFFLINE'] = '1'
    torch, tokenizer, model, revision = load_model(args.assets, args.model)
    scope = {'task_fingerprint': task_fingerprint(task),
             'data_version':dataset['version'], 'tool_version':'native-lookup-single-v2',
             'evaluator_version':'term-quote-observed-ids-v3', 'model_revision':revision}
    result = {'complete':False, 'dataset':'native-language-feedback-v1', 'evaluation_use':'regression',
              'case_sha256':hashlib.sha256(raw_cases).hexdigest(), 'prior_sha256':hashlib.sha256(prior_bytes).hexdigest(),
              'model_name':args.model, 'model_revision':revision, 'scope':scope,
              'reflection':None, 'memo':None, 'rows':[]}
    output = Path(args.output)
    persist(output, result)

    def complete(messages, tools):
        prompt = tokenizer.apply_chat_template(messages, tools=tools, tokenize=False,
                                               add_generation_prompt=True, enable_thinking=False)
        inputs = tokenizer(prompt, return_tensors='pt', add_special_tokens=False)
        count = inputs['input_ids'].shape[1]
        if count > 1024:
            raise ValueError('input_token_budget')
        start = time.monotonic()
        with torch.inference_mode():
            reply = model.generate(**inputs, do_sample=False, max_new_tokens=192, use_cache=True,
                                   pad_token_id=tokenizer.eos_token_id)
        suffix = reply[0,count:]
        return tokenizer.decode(suffix,skip_special_tokens=True).strip(), {
            'input_tokens':count,'output_tokens':len(suffix),'at_output_limit':len(suffix)==192,
            'seconds':time.monotonic()-start}

    view = reflection_input(case, prior)
    messages = [{'role':'system','content':'Review the recorded failure. Write one concise suggestion for the next attempt. '
                 'Use only actual observations and proposals. Do not provide an answer, invent evidence or grant permissions. '
                 'Suggest how to respect the interface and verify the goal. Plain advice only, no hidden reasoning.'},
                {'role':'user','content':json.dumps(view)}]
    text, usage = complete(messages, [])
    result['reflection'] = dict(usage, raw=text, view=view, model_messages=messages)
    persist(output,result)
    try:
        result['memo'] = make_memo(text,usage,scope,result['prior_sha256'])
    except ValueError as error:
        result['reflection_error'] = str(error)
    persist(output,result)
    for mode, memo in (('without_advice',None),('with_advice',result['memo'])):
        if mode == 'with_advice' and memo is None:
            continue
        row = actor(task,complete,memo,scope)
        row['mode'] = mode
        row['coverage_pass'] = score(case,row)
        result['rows'].append(row)
        persist(output,result)
        print(mode,row['reads'],row['coverage_pass'],row['error'],flush=True)
    result['complete'] = True
    persist(output,result)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--assets',required=True)
    parser.add_argument('--output',required=True)
    parser.add_argument('--model',choices=('qwen3-0.6b','qwen3-1.7b'),default='qwen3-0.6b')
    args = parser.parse_args()
    if os.environ.get('LOCAL_STRATEGY_E2E') != '1':
        parser.error('explicit LOCAL_STRATEGY_E2E=1 required')
    if Path(args.output).exists():
        parser.error('output must be a new file')
    signal.alarm(300)
    main(args)
