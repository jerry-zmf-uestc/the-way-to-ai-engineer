"""Original model-proposed/evaluated arithmetic search; exact verification never runs code."""
from __future__ import annotations
import argparse
from dataclasses import dataclass
from fractions import Fraction
import hashlib
import importlib.metadata
import json
import math
import os
from pathlib import Path
import signal
import sys
import time
from bounded_search import Limits, search
from local_strategies import load_model, unique


@dataclass(frozen=True)
class Term:
    value: Fraction
    expression: str
    leaves: frozenset


def transition(state, step):
    if not isinstance(step, dict) or set(step) != {'left', 'right', 'op'}:
        raise ValueError('step_shape')
    i, j, op = step['left'], step['right'], step['op']
    if type(i) is not int or type(j) is not int or not 0 <= i < len(state) or not 0 <= j < len(state) or i == j:
        raise ValueError('operand_index')
    if op not in ('+', '-', '*', '/'):
        raise ValueError('operator')
    left, right = state[i], state[j]
    if left.leaves & right.leaves:
        raise ValueError('reused_leaf')
    if op == '+':
        value = left.value + right.value
    elif op == '-':
        value = left.value - right.value
    elif op == '*':
        value = left.value * right.value
    else:
        if right.value == 0:
            raise ValueError('division_zero')
        value = left.value / right.value
    if value.numerator.bit_length() > 64 or value.denominator.bit_length() > 64:
        raise ValueError('arithmetic_bound')
    term = Term(value, '(' + left.expression + op + right.expression + ')', left.leaves | right.leaves)
    return tuple(x for index, x in enumerate(state) if index not in (i, j)) + (term,)


def term_id(term):
    return 't' + '_'.join(str(x) for x in sorted(term.leaves))


def stable_transition(state, step):
    if not isinstance(step, dict) or set(step) != {'left', 'right', 'op'}:
        raise ValueError('step_shape')
    identities = {term_id(term): i for i, term in enumerate(state)}
    left, right = step['left'], step['right']
    if type(left) is not str or type(right) is not str or left not in identities or right not in identities or left == right:
        raise ValueError('operand_identity')
    return transition(state, {'left': identities[left], 'right': identities[right], 'op': step['op']})


def arithmetic_reference(numbers, max_attempts=32768):
    if not isinstance(numbers, list) or len(numbers) != 4 or any(type(x) is not int or not 1 <= x <= 20 for x in numbers):
        raise ValueError('task_shape')
    if type(max_attempts) is not int or not 0 < max_attempts <= 32768:
        raise ValueError('reference_budget')
    initial = tuple(Term(Fraction(x), str(x), frozenset({i})) for i, x in enumerate(numbers))
    seen, attempts = set(), 0

    def visit(state, steps):
        nonlocal attempts
        key = tuple(sorted((x.value, tuple(sorted(x.leaves))) for x in state))
        if key in seen:
            return None
        seen.add(key)
        if len(state) == 1:
            return (state[0], steps) if state[0].value == 24 and state[0].leaves == frozenset(range(4)) else None
        for left in range(len(state)):
            for right in range(len(state)):
                if left == right:
                    continue
                for op in ('+', '-', '*', '/'):
                    if op == '/' and state[right].value == 0:
                        continue
                    if attempts == max_attempts:
                        raise TimeoutError('reference_node_budget')
                    attempts += 1
                    step = {'left': left, 'right': right, 'op': op}
                    found = visit(transition(state, step), steps + [step])
                    if found is not None:
                        return found
        return None

    try:
        found = visit(initial, [])
    except TimeoutError:
        return {'reason': 'node_budget', 'solvable': None, 'attempts': attempts, 'steps': [], 'expression': None}
    return {'reason': 'solved' if found else 'exhausted', 'solvable': found is not None,
            'attempts': attempts, 'steps': found[1] if found else [], 'expression': found[0].expression if found else None}


def view(state, target, stable=False):
    if stable:
        return {'target': target, 'terms': [{'term_id': term_id(x), 'value': str(x.value), 'expression': x.expression} for x in state]}
    return {'target': target, 'terms': [{'index': i, 'value': str(x.value), 'expression': x.expression} for i, x in enumerate(state)]}


def run_case(case, width, generate, checkpoint=lambda row: None, stable=False, goal_policy='retained'):
    numbers = case['numbers']
    if len(numbers) != 4 or any(type(x) is not int or not 1 <= x <= 20 for x in numbers) or case['target'] != 24:
        raise ValueError('task_shape')
    states = {'root': tuple(Term(Fraction(x), str(x), frozenset({i})) for i, x in enumerate(numbers))}
    row = {'case_id': case['id'], 'width': width, 'solved': False, 'reason': '', 'error': None,
           'expression': None, 'calls': [], 'states': {}, 'scores': {}, 'generated': 0, 'evaluated': 0, 'expanded': 0}

    def call(kind, state):
        if len(row['calls']) >= 18:
            raise ValueError('model_call_budget')
        raw, record = generate(kind, view(state, case['target'], stable))
        row['calls'].append(dict(record, kind=kind, raw=raw))
        checkpoint(row)
        if record['at_output_limit']:
            raise ValueError('truncated_' + kind)
        result = json.loads(raw, object_pairs_hook=unique)
        if not isinstance(result, dict):
            raise ValueError('response_shape')
        return result

    def propose(path):
        parent = states[path[-1]]
        data = call('propose', parent)
        if set(data) != {'proposals'} or not isinstance(data['proposals'], list) or len(data['proposals']) > 2:
            raise ValueError('proposal_shape')
        # Validate the whole batch before publishing any child state.
        convert = stable_transition if stable else transition
        children = [convert(parent, step) for step in data['proposals']]
        keys = [tuple(sorted((x.value, tuple(sorted(x.leaves))) for x in child)) for child in children]
        if len(set(keys)) != len(keys):
            raise ValueError('duplicate_state')
        ids = []
        for child in children:
            identity = 's' + str(len(states))
            states[identity] = child
            row['states'][identity] = view(child, case['target'], stable)
            ids.append(identity)
        return ids

    def evaluate(path):
        data = call('evaluate', states[path[-1]])
        score = data.get('score')
        if set(data) != {'score'} or type(score) not in (int, float) or not 0 <= score <= 1 or not math.isfinite(score):
            raise ValueError('score_shape')
        row['scores'][path[-1]] = score
        return score

    def verify(path):
        state = states[path[-1]]
        return len(state) == 1 and state[0].value == case['target'] and state[0].leaves == frozenset(range(4))

    try:
        result = search('root', propose, evaluate, verify, Limits(width=width, depth=3, nodes=10, per_parent=2), goal_policy=goal_policy)
        row.update(reason=result.reason, generated=result.generated, evaluated=result.evaluated, expanded=result.expanded)
        if result.path:
            row['solved'] = verify(result.path)
            row['expression'] = states[result.path[-1]][0].expression
    except (ValueError, TypeError, KeyError, OverflowError) as exc:
        row['error'] = str(exc) if isinstance(exc, ValueError) else 'invalid_response_type'
        row['reason'] = 'failed'
        # Engine return counters are unknown on an exception, never fabricated as zero.
        row['generated'] = row['evaluated'] = row['expanded'] = None
    checkpoint(row)
    return row


def persist(path, result):
    data = json.dumps(result, ensure_ascii=False, allow_nan=False).encode()
    if len(data) > 512 << 10:
        raise ValueError('report_bound')
    temporary = path.with_name(path.name + '.tmp')
    try:
        temporary.write_bytes(data)
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)


def main(args):
    if sys.version_info[:3] != (3, 12, 14):
        raise ValueError('locked Python required')
    for line in Path(__file__).with_name('generation-requirements.lock').read_text().splitlines():
        name, version = line.split('==')
        if importlib.metadata.version(name) != version:
            raise ValueError('locked dependency required')
    raw_cases = Path(__file__).with_name('search-cases.json').read_bytes()
    cases = json.loads(raw_cases)
    if cases['version'] != 'arithmetic-search-v1' or len(cases['cases']) != 4:
        raise ValueError('registered dataset required')
    os.environ['HF_HUB_OFFLINE'] = os.environ['TRANSFORMERS_OFFLINE'] = '1'
    torch, tokenizer, model, revision = load_model(args.assets)
    report = {'complete': False, 'dataset': cases['version'], 'split': cases['split'],
              'case_sha256': hashlib.sha256(raw_cases).hexdigest(), 'model_revision': revision,
              'prompt_version': args.prompt, 'rows': [], 'active': None}
    output = Path(args.output)
    persist(output, report)

    def generate(kind, state):
        if kind == 'propose':
            instruction = 'Return raw JSON only, without markdown fences. Exactly one key proposals, an array of at most two steps. Each step has exactly left and right (distinct integer indices from the current terms) and op (one of +, -, *, /). Combine two current terms to progress toward the target using every initial number once. You may return an empty proposals array when no useful step is available. Do not invent indices or use division by zero.'
            maximum = 160
            if args.prompt in ('arithmetic-search-v3', 'arithmetic-search-v4'):
                instruction = 'Return raw JSON only. Exactly one key proposals, an array of at most two steps. Each step has exactly left, right, op. Copy left and right as two DISTINCT term_id strings from the CURRENT terms. op is +, -, *, or /. IDs name current operands, not positions: after combining t0 and t1, use t0_1 for that combined value and never reuse t0 or t1 separately. Use all initial numbers exactly once to reach the target. Do not invent IDs, use integer indices, or divide by zero. An empty proposals array is allowed.'
                if args.prompt == 'arithmetic-search-v4':
                    instruction += ' Write each step in key order op, left, right. The op field is REQUIRED even when only two terms remain. A pair without an operator is INVALID, not a final answer. Consider all four operators, not only addition. Return no duplicate steps. Do not omit op to signal completion.'
        else:
            instruction = 'Return raw JSON only, without markdown fences. Exactly one key score: a finite number from 0 to 1 indicating how promising this partial arithmetic state is for reaching the target using all terms. This is a heuristic estimate, not proof of a solution. No other fields.'
            maximum = 64
        prompt = tokenizer.apply_chat_template([{'role': 'system', 'content': instruction},
                                               {'role': 'user', 'content': json.dumps(state)}], tokenize=False, add_generation_prompt=True, enable_thinking=False)
        prefix = ''
        if args.prompt in ('arithmetic-search-v2', 'arithmetic-search-v3', 'arithmetic-search-v4'):
            prefix = '{"proposals":' if kind == 'propose' else '{"score":'
            prompt += prefix
        inputs = tokenizer(prompt, return_tensors='pt', add_special_tokens=False)
        count = inputs['input_ids'].shape[1]
        if count > 1024:
            raise ValueError('input_token_budget')
        started = time.monotonic()
        with torch.inference_mode():
            tokens = model.generate(**inputs, do_sample=False, max_new_tokens=maximum, use_cache=True, pad_token_id=tokenizer.eos_token_id)
        suffix = tokens[0, count:]
        generated_text = tokenizer.decode(suffix, skip_special_tokens=True).strip()
        raw = prefix + generated_text
        return raw, {'input_tokens': count, 'output_tokens': len(suffix), 'max_new_tokens': maximum,
                     'at_output_limit': len(suffix) == maximum, 'seconds': time.monotonic() - started, 'view': state,
                     'application_prefix': prefix, 'generated_text': generated_text}

    for case in cases['cases']:
        for width in (1, 2):
            def checkpoint(row):
                report['active'] = row
                persist(output, report)
            row = run_case(case, width, generate, checkpoint, stable=args.prompt in ('arithmetic-search-v3', 'arithmetic-search-v4'))
            report['rows'].append(row)
            report['active'] = None
            persist(output, report)
            print(f"case={case['id']} width={width} solved={row['solved']} reason={row['reason']} error={row['error']} calls={len(row['calls'])}", flush=True)
    report['complete'] = True
    persist(output, report)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--assets', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--prompt', choices=('arithmetic-search-v1', 'arithmetic-search-v2', 'arithmetic-search-v3', 'arithmetic-search-v4'), default='arithmetic-search-v1')
    args = parser.parse_args()
    if os.environ.get('LOCAL_STRATEGY_E2E') != '1':
        parser.error('explicit LOCAL_STRATEGY_E2E=1 required')
    if Path(args.output).exists():
        parser.error('output must be a new file')
    signal.alarm(300)
    try:
        main(args)
    except Exception:
        print('model search failed; saved report remains incomplete', file=sys.stderr)
        sys.exit(1)
