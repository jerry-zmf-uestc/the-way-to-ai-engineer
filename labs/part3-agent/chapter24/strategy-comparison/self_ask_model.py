"""Original external-answer Self-Ask experiment; generated questions are not hidden thoughts."""
import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import re
import signal
import sys
import time
from local_strategies import load_model, unique
from model_search import persist


def action_shape(data, sole_reference):
    if not isinstance(data, dict):
        raise ValueError('action_shape')
    action = data.get('action')
    if action == 'ask':
        allowed = [{'action', 'question', 'reference'}]
        if sole_reference:
            allowed.append({'action', 'question'})
        if set(data) not in allowed or not isinstance(data['question'], str) or not 0 < len(data['question']) <= 256:
            raise ValueError('question_shape')
    elif action == 'answer':
        if set(data) != {'action', 'answer', 'citations'} or not isinstance(data['answer'], str) or not 0 < len(data['answer']) <= 4096 or not isinstance(data['citations'], list) or not 0 < len(data['citations']) <= 3:
            raise ValueError('answer_shape')
    elif action == 'refuse':
        if set(data) != {'action', 'reason'} or data['reason'] != 'insufficient_evidence':
            raise ValueError('refusal_shape')
    else:
        raise ValueError('unknown_action')
    return action


def v5_model_view(view):
    result = {
        'question': view['question'],
        'unread_document_ids': list(view['available_references']),
        'queries_remaining': view['queries_remaining'],
        'read_documents': [],
    }
    for observation in view['observations']:
        doc = observation['document']
        item = {key: doc[key] for key in ('id', 'text', 'references')}
        item['follow_up'] = observation['follow_up']
        if 'error' in doc:
            item['error'] = doc['error']
        result['read_documents'].append(item)
    if 'protocol_feedback' in view:
        result['protocol_feedback'] = dict(view['protocol_feedback'])
    return json.loads(json.dumps(result))


def v6_model_view(view):
    result = v5_model_view(view)
    result['allowed_actions'] = ['answer', 'refuse']
    if result['queries_remaining'] > 0 and result['unread_document_ids']:
        result['allowed_actions'].append('ask')
    for doc in result['read_documents']:
        del doc['references']
        del doc['follow_up']
    return result


def question_reference(question, remaining, corpus):
    stop = {'a', 'an', 'the', 'is', 'are', 'of', 'for', 'to', 'what', 'which', 'does', 'do', 'in', 'and'}
    tokens = set(re.findall(r'[a-z0-9]+', question.casefold())) - stop
    ranked = []
    for reference in remaining:
        doc = corpus.get(reference)
        if doc is None:
            continue
        if doc['id'] != reference or not isinstance(doc['text'], str) or len(doc['text']) > 1024:
            raise ValueError('observation_shape')
        overlap = len(tokens & set(re.findall(r'[a-z0-9]+', doc['text'].casefold())))
        if overlap:
            ranked.append((-overlap, reference))
    if not ranked:
        raise ValueError('query_no_match')
    return sorted(ranked)[0][1]


def execute(case, mode, generate, checkpoint=lambda row: None, sole_reference=False, repair_protocol=False, expose_budget=False, query_lookup=False):
    observed = {}
    available = {case['seed']}
    row = {'case_id': case['id'], 'mode': mode, 'calls': [], 'questions': [], 'observations': [],
           'answer': '', 'citations': [], 'refused': False, 'error': None, 'reads': 0,
           'repairs': [], 'semantic_quality': 'unreviewed'}
    feedback = None
    try:
        for _ in range(5):
            view = {'question': case['question'], 'available_references': sorted(available - observed.keys()),
                    'observations': row['observations']}
            if expose_budget:
                view['queries_remaining'] = (1 if mode == 'single-query' else 3) - row['reads']
            if feedback is not None:
                view['protocol_feedback'] = feedback
            raw, record = generate(view, mode)
            row['calls'].append(dict(record, raw=raw, view=json.loads(json.dumps(view))))
            checkpoint(row)
            if record.get('adapter_error'):
                raise ValueError(record['adapter_error'])
            if record['at_output_limit']:
                raise ValueError('truncated_proposal')
            try:
                data = json.loads(raw, object_pairs_hook=unique)
                action = action_shape(data, sole_reference or query_lookup)
            except ValueError as error:
                if not repair_protocol or row['repairs']:
                    raise
                code = str(error)
                if code not in ('action_shape', 'question_shape', 'answer_shape', 'refusal_shape', 'unknown_action'):
                    code = 'json_format'
                row['repairs'].append({'code': code})
                feedback = row['repairs'][-1]
                checkpoint(row)
                continue
            feedback = None
            if action == 'ask':
                remaining = sorted(available - observed.keys())
                bound = 'model_reference'
                reference = data.get('reference')
                if query_lookup:
                    if reference is not None:
                        raise ValueError('query_reference_not_allowed')
                    if row['reads'] >= 3 or (mode == 'single-query' and row['reads'] >= 1):
                        raise ValueError('query_budget')
                    row['reads'] += 1
                    reference = question_reference(data['question'], remaining, case['corpus'])
                    bound = 'application_query_match'
                if reference is None and sole_reference and len(remaining) == 1:
                    reference = remaining[0]
                    bound = 'sole_available_reference'
                if not isinstance(reference, str) or reference not in available or reference in observed:
                    raise ValueError('unobserved_reference')
                if not query_lookup and (row['reads'] >= 3 or (mode == 'single-query' and row['reads'] >= 1)):
                    raise ValueError('query_budget')
                if not query_lookup:
                    row['reads'] += 1
                row['questions'].append({'question': data['question'], 'reference': reference, 'binding': bound})
                doc = case['corpus'].get(reference)
                if doc is None:
                    doc = {'id': reference, 'text': '', 'references': [], 'error': 'not_found'}
                elif doc['id'] != reference or len(doc['text']) > 1024 or len(doc['references']) > 8:
                    raise ValueError('observation_shape')
                observed[reference] = doc
                available.update(doc['references'])
                if len(available) > 8:
                    raise ValueError('reference_budget')
                row['observations'].append({'follow_up': data['question'], 'document': dict(doc)})
            elif action == 'answer':
                for citation in data['citations']:
                    if not isinstance(citation, dict) or set(citation) != {'id', 'quote'}:
                        raise ValueError('citation_shape')
                    identity, quote = citation['id'], citation['quote']
                    if not isinstance(identity, str) or identity not in observed or not isinstance(quote, str) or not quote or quote not in observed[identity]['text']:
                        raise ValueError('unobserved_citation')
                row['answer'] = data['answer']
                row['citations'] = json.loads(json.dumps(data['citations']))
                break
            elif action == 'refuse':
                row['refused'] = True
                break
            else:
                raise ValueError('unknown_action')
        else:
            raise ValueError('round_budget')
    except (ValueError, TypeError, KeyError) as error:
        row['error'] = str(error) if isinstance(error, ValueError) else 'invalid_field_type'
    checkpoint(row)
    return row


def score(case, row):
    if row['error']:
        return False
    observed = {item['document']['id'] for item in row['observations']}
    if not set(case.get('required_observed_ids', [])).issubset(observed):
        return False
    if case['expected_refuse']:
        return row['refused'] and not row['answer'] and row['reads'] > 0
    words = set(re.findall(r'[A-Za-z]+|\d+', row['answer'].lower()))
    quoted = set(re.findall(r'[A-Za-z]+|\d+', ' '.join(c['quote'] for c in row['citations']).lower()))
    return bool(row['answer']) and not row['refused'] and all(term.lower() in words and term.lower() in quoted for term in case['required_terms'])


def registered_cases(name='development-v1'):
    entries = {'development-v1': ('self-ask-cases.json', 'followup-development-v1'),
               'independent-v1': ('followup-independent-cases.json', 'followup-independent-v1')}
    if name not in entries:
        raise ValueError('unregistered dataset')
    filename, version = entries[name]
    raw = Path(__file__).with_name(filename).read_bytes()
    dataset = json.loads(raw)
    if dataset['version'] != version or len(dataset['cases']) != 4:
        raise ValueError('registered dataset required')
    if len({case['id'] for case in dataset['cases']}) != 4:
        raise ValueError('duplicate case identity')
    return raw, dataset


def main(args):
    if sys.version_info[:3] != (3, 12, 14):
        raise ValueError('locked Python required')
    for line in Path(__file__).with_name('generation-requirements.lock').read_text().splitlines():
        name, version = line.split('==')
        if importlib.metadata.version(name) != version:
            raise ValueError('locked dependency required')
    raw_cases = Path(__file__).with_name('self-ask-cases.json').read_bytes()
    dataset = json.loads(raw_cases)
    if dataset['version'] != 'followup-development-v1' or len(dataset['cases']) != 4:
        raise ValueError('registered dataset required')
    os.environ['HF_HUB_OFFLINE'] = os.environ['TRANSFORMERS_OFFLINE'] = '1'
    torch, tokenizer, model, revision = load_model(args.assets, args.model)
    report = {'complete': False, 'dataset': dataset['version'], 'split': dataset['split'],
              'case_sha256': hashlib.sha256(raw_cases).hexdigest(), 'model_revision': revision,
              'prompt_version': args.prompt, 'scorer_version': 'term-and-cited-quote-v2', 'rows': [], 'active': None}
    report['model_name'] = args.model
    report['runtime'] = {'device': 'cpu', 'threads': torch.get_num_threads(),
                         'dtype': str(next(model.parameters()).dtype)}
    output = Path(args.output)
    persist(output, report)

    def generate(view, mode):
        instructions = 'Complete a question by asking public follow-up questions and using external document answers. Return one raw JSON object. For action ask: exactly action, question (one follow-up question string), reference (one current available_references string). For action answer: exactly action, answer (answer to original question), citations (array of id/quote objects with exact observed quotes). For action refuse: exactly action, reason=insufficient_evidence. Never invent facts or references. Intermediate answers must come from observations. Do not repeat an observed reference. No markdown fences or private reasoning. '
        if mode == 'single-query':
            instructions += 'This strategy permits only one external query, then answer or refuse from that observation.'
        else:
            instructions += 'After an intermediate answer, ask another focused question if needed, using newly discovered references; at most three external queries.'
        prefix = '{"action":'
        if args.prompt in ('self-ask-v2', 'self-ask-v3', 'self-ask-v4'):
            instructions = 'Choose one action enum: "ask", "answer", or "refuse". Do not write the words exactly action as the action value. For "ask", the only keys are action, question, reference. question is one focused follow-up question; reference is one exact available_references ID. For "answer", the only keys are action, answer, citations. citations is an array of objects with id and quote; quote must exactly match observed document text. For "refuse", the only keys are action, reason with value insufficient_evidence. Never combine ask fields and answer fields. Do not invent numerical values, citations or document text. Facts come only from observations. Output JSON only, no markdown fences or hidden reasoning. '
            instructions += 'Only one lookup is permitted before answering or refusing.' if mode == 'single-query' else 'Use intermediate answers to form the next follow-up question, then combine observed facts for the original question.'
            if args.prompt in ('self-ask-v3', 'self-ask-v4'):
                instructions += ' If available_references has exactly one ID, ask may omit reference; the application binds that sole known reference. With more than one ID, reference must be an exact chosen ID.'
            if args.prompt == 'self-ask-v4':
                instructions += ' A protocol_feedback code means a previous response had invalid fields. Correct the required action fields. For answer, include citations with exact observed id and quote; if the observations lack the answer, ask a follow-up using an available reference. For refuse, include reason="insufficient_evidence". The feedback is not evidence about the correct factual answer.'
            if not view['observations']:
                # No factual answer can be justified before any external observation.
                prefix = '{"action":"ask","question":'
        model_view = view
        if args.prompt == 'self-ask-v5':
            model_view = v5_model_view(view)
            instructions = ('Answer the original question using only read_documents. '
                            'An unread document ID is a location, not an observed fact. '
                            'If a needed fact is missing and queries_remaining is positive, ask a focused follow-up using an unread_document_ids entry. '
                            'Return one JSON object, no markdown or reasoning. '
                            'ask keys: action="ask", question, reference. reference must copy an unread ID; it may be omitted only when exactly one ID is available. '
                            'answer keys: action="answer", answer, citations. Each citation has id and quote copied from a read document. '
                            'If facts are missing and no further query is possible, use action="refuse", reason="insufficient_evidence". '
                            'protocol_feedback reports invalid action fields; correct the format without inventing evidence.')
            if not view['observations']:
                prefix = '{"action":"ask","question":'
        if args.prompt == 'self-ask-v6':
            model_view = v6_model_view(view)
            instructions = ('Decide the next step for the original question. Only use an allowed_actions value. '
                            'If read_documents contain the needed facts, answer now. '
                            'answer JSON has action="answer", answer, citations; each citation has id and quote copied exactly from a read document. '
                            'Otherwise, if ask is allowed, ask for the specific missing fact using the intermediate information. '
                            'ask JSON has action="ask", question, reference; copy reference from unread_document_ids, not from a document already read. '
                            'With exactly one unread ID, reference may be omitted. Never repeat a read. '
                            'If facts are missing and ask is unavailable, output action="refuse", reason="insufficient_evidence". '
                            'Return one raw JSON object with only the fields of that action. No markdown or reasoning. '
                            'A protocol_feedback code requests format repair, not new evidence.')
            if not view['observations']:
                prefix = '{"action":"ask","question":'
        prompt = tokenizer.apply_chat_template([{'role': 'system', 'content': instructions},
                                               {'role': 'user', 'content': json.dumps(model_view)}], tokenize=False, add_generation_prompt=True, enable_thinking=False) + prefix
        inputs = tokenizer(prompt, return_tensors='pt', add_special_tokens=False)
        count = inputs['input_ids'].shape[1]
        if count > 1024:
            raise ValueError('input_token_budget')
        started = time.monotonic()
        with torch.inference_mode():
            reply = model.generate(**inputs, do_sample=False, max_new_tokens=192, use_cache=True, pad_token_id=tokenizer.eos_token_id)
        suffix = reply[0, count:]
        generated = tokenizer.decode(suffix, skip_special_tokens=True).strip()
        return prefix + generated, {'application_prefix': prefix, 'generated_text': generated,
                                    'model_view': json.loads(json.dumps(model_view)),
                                    'input_tokens': count, 'output_tokens': len(suffix),
                                    'at_output_limit': len(suffix) == 192, 'seconds': time.monotonic() - started}

    for case in dataset['cases']:
        # Execution projection excludes expected terms/refusal labels and the case identifier from the view.
        task = {k: case[k] for k in ('id', 'question', 'seed', 'corpus')}
        for mode in ('single-query', 'self-ask'):
            def checkpoint(row):
                report['active'] = row
                persist(output, report)
            row = execute(task, mode, generate, checkpoint,
                          sole_reference=args.prompt in ('self-ask-v3', 'self-ask-v4', 'self-ask-v5', 'self-ask-v6'),
                          repair_protocol=args.prompt in ('self-ask-v4', 'self-ask-v5', 'self-ask-v6'),
                          expose_budget=args.prompt in ('self-ask-v5', 'self-ask-v6'))
            row['coverage_pass'] = score(case, row)
            report['rows'].append(row)
            report['active'] = None
            persist(output, report)
            print(f"case={case['id']} mode={mode} coverage={row['coverage_pass']} reads={row['reads']} error={row['error']}", flush=True)
    report['complete'] = True
    persist(output, report)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--assets', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--model', choices=('qwen3-0.6b', 'qwen3-1.7b'), default='qwen3-0.6b')
    parser.add_argument('--prompt', choices=('self-ask-v1', 'self-ask-v2', 'self-ask-v3', 'self-ask-v4', 'self-ask-v5', 'self-ask-v6'), default='self-ask-v1')
    args = parser.parse_args()
    if os.environ.get('LOCAL_STRATEGY_E2E') != '1':
        parser.error('explicit LOCAL_STRATEGY_E2E=1 required')
    if Path(args.output).exists():
        parser.error('output must be a new file')
    signal.alarm(300)
    try:
        main(args)
    except Exception:
        print('self-ask failed; saved report remains incomplete', file=sys.stderr)
        sys.exit(1)
