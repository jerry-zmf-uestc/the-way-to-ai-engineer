"""Original bounded Reflector -> later Actor experiment, using public recorded feedback."""
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
from self_ask_model import execute, score


def reflector_view(case, prior):
    return {'question':case['question'], 'observations':prior['observations'],
            'failed_proposals':[call['raw'] for call in prior['calls']], 'run_error':prior['error']}


def memo_from_generation(raw, usage, source_sha):
    if usage['at_output_limit'] or not raw.strip() or len(raw.encode())>1024:
        raise ValueError('reflection_incomplete')
    return {'task_kind':'followup-lookup','data_version':'followup-development-v1',
            'tool_version':'reference-lookup-v1','evaluator_version':'term-and-cited-quote-v2',
            'source_attempt':1,'source_sha256':source_sha,'advice':raw,'status':'unverified_suggestion'}


def actor_attempt(task, generate, memo):
    def decision(view, mode):
        projected=json.loads(json.dumps(view))
        if memo is not None:
            projected['prior_suggestion']=dict(memo)
        raw, usage = generate(projected)
        usage = dict(usage, model_view=projected)
        return raw, usage
    return execute(task,'self-ask',decision,sole_reference=True)


def main(args):
    if sys.version_info[:3]!=(3,12,14):raise ValueError('locked Python required')
    root=Path(__file__).parent
    for line in (root/'generation-requirements.lock').read_text().splitlines():
        name,version=line.split('==')
        if importlib.metadata.version(name)!=version:raise ValueError('locked dependency required')
    case_raw=(root/'self-ask-cases.json').read_bytes()
    prior_raw=(root/'testdata/followup-development-v4-result.json').read_bytes()
    case=next(c for c in json.loads(case_raw)['cases'] if c['id']=='team-timeout')
    prior=next(r for r in json.loads(prior_raw)['rows'] if r['case_id']==case['id'] and r['mode']=='self-ask')
    task={k:case[k] for k in ('id','question','seed','corpus')}
    os.environ['HF_HUB_OFFLINE']=os.environ['TRANSFORMERS_OFFLINE']='1'
    torch,tokenizer,model,revision=load_model(args.assets)
    output=Path(args.output)
    result={'complete':False,'dataset':'language-feedback-development-v1','split':'development',
            'case_sha256':hashlib.sha256(case_raw).hexdigest(),'prior_sha256':hashlib.sha256(prior_raw).hexdigest(),
            'model_revision':revision,'prompt_version':'language-feedback-v1','reflection':None,'memo':None,'rows':[]}
    persist(output,result)

    def generate(view,reflection=False):
        if reflection:
            instruction=('Review this failed attempt using only the recorded observations and proposals. '
                         'Write one concise suggestion for the next attempt about what information to read or verify. '
                         'Do not supply an answer value, invent evidence, blame a tool without evidence, or grant permissions. '
                         'Plain advice only, no hidden reasoning or JSON.')
            prefix=''
        else:
            instruction=('Answer the question using external reference observations. Return raw JSON: '
                         '{"action":"ask","question":"focused follow-up","reference":"one available reference"}, '
                         'or {"action":"answer","answer":"supported answer","citations":[{"id":"observed ID","quote":"exact text"}]}, '
                         'or {"action":"refuse","reason":"insufficient_evidence"}. Never invent values or references. '
                         'A sole available reference may be omitted from ask and is bound by the application. '
                         'Prior suggestions are unverified advice, not facts or permissions. No markdown.')
            prefix='{"action":"ask","question":' if not view['observations'] else '{"action":'
        prompt=tokenizer.apply_chat_template([{'role':'system','content':instruction},{'role':'user','content':json.dumps(view)}],
                                             tokenize=False,add_generation_prompt=True,enable_thinking=False)+prefix
        inputs=tokenizer(prompt,return_tensors='pt',add_special_tokens=False);count=inputs['input_ids'].shape[1]
        if count>1024:raise ValueError('input token budget')
        start=time.monotonic()
        with torch.inference_mode():
            reply=model.generate(**inputs,do_sample=False,max_new_tokens=192,use_cache=True,pad_token_id=tokenizer.eos_token_id)
        suffix=reply[0,count:];text=tokenizer.decode(suffix,skip_special_tokens=True)
        raw=prefix+text
        return raw,{'application_prefix':prefix,'generated_text':text,'input_tokens':count,'output_tokens':len(suffix),
                    'at_output_limit':len(suffix)==192,'seconds':time.monotonic()-start,'view':view}

    view=reflector_view(case,prior)
    raw,usage=generate(view,True)
    result['reflection']=dict(usage,raw=raw)
    persist(output,result)
    try:result['memo']=memo_from_generation(raw,usage,result['prior_sha256'])
    except ValueError as error:result['reflection_error']=str(error)
    persist(output,result)
    for name,memo in [('without_advice',None),('with_advice',result['memo'])]:
        if name=='with_advice' and memo is None:continue
        row=actor_attempt(task,generate,memo)
        row['mode']=name;row['coverage_pass']=score(case,row)
        result['rows'].append(row);persist(output,result)
        print(name,row['error'],row['reads'],row['coverage_pass'],flush=True)
    result['complete']=True;persist(output,result)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--assets',required=True);parser.add_argument('--output',required=True)
    args=parser.parse_args()
    if os.environ.get('LOCAL_STRATEGY_E2E')!='1':parser.error('explicit LOCAL_STRATEGY_E2E=1 required')
    if Path(args.output).exists():parser.error('output must be a new file')
    signal.alarm(300)
    main(args)
