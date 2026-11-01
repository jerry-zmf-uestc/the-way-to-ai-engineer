import json
import hashlib
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from fractions import Fraction
from model_search import Term, transition, stable_transition, view, run_case, arithmetic_reference


class ArithmeticChecks(unittest.TestCase):
    def test_exact_reference_replays_solutions_and_proves_exhaustion(self):
        dataset = json.loads(Path(__file__).with_name('search-cases.json').read_text())
        for case in dataset['cases']:
            result = arithmetic_reference(case['numbers'])
            self.assertLessEqual(result['attempts'], 32768)
            if case['id'] == 'low-range':
                self.assertEqual((result['reason'], result['solvable']), ('exhausted', False))
                self.assertEqual(result['steps'], [])
                continue
            self.assertEqual((result['reason'], result['solvable']), ('solved', True))
            state = tuple(Term(Fraction(x), str(x), frozenset({i})) for i, x in enumerate(case['numbers']))
            self.assertEqual(len(result['steps']), 3)
            for step in result['steps']:
                state = transition(state, step)
            self.assertEqual((len(state), state[0].value, state[0].leaves), (1, Fraction(24), frozenset(range(4))))
            self.assertEqual(result['expression'], state[0].expression)

    def test_reference_budget_unknown_is_distinct_from_no_solution(self):
        result = arithmetic_reference([3, 3, 8, 8], max_attempts=1)
        self.assertEqual((result['reason'], result['solvable'], result['attempts']), ('node_budget', None, 1))
        self.assertEqual(result['steps'], [])
        for numbers, budget in [([True, 2, 3, 4], 16), ([0, 2, 3, 4], 16), ([1, 2, 3], 16), ([1, 2, 3, 4], True), ([1, 2, 3, 4], 32769)]:
            with self.assertRaises(ValueError):
                arithmetic_reference(numbers, budget)

    def test_stable_operands_survive_other_merges_and_reject_consumed_ids(self):
        root = tuple(Term(Fraction(x), str(x), frozenset({i})) for i, x in enumerate((1, 2, 3, 4)))
        child = stable_transition(root, {'left': 't0', 'right': 't1', 'op': '*'})
        self.assertEqual([t['term_id'] for t in view(child, 24, True)['terms']], ['t2', 't3', 't0_1'])
        self.assertTrue(all('index' not in t for t in view(child, 24, True)['terms']))
        next_state = stable_transition(child, {'left': 't3', 'right': 't0_1', 'op': '/'})
        self.assertEqual(next_state[-1].value, 2)
        for left, right in [('t0', 't2'), ('missing', 't2'), ('t2', 't2'), (0, 1), (True, 't2')]:
            with self.assertRaises(ValueError):
                stable_transition(child, {'left': left, 'right': right, 'op': '+'})

    def test_stable_contract_uses_same_engine_and_whole_batch_rejection(self):
        def generate(kind, current):
            if kind == 'evaluate':
                raw = '{"score":1}'
            else:
                terms = current['terms']
                right = 2 if len(terms) == 3 else 1
                raw = json.dumps({'proposals': [{'left': terms[0]['term_id'], 'right': terms[right]['term_id'], 'op': '*'}]})
            return raw, {'at_output_limit': False}
        case = {'id': 'p', 'numbers': [1, 2, 3, 4], 'target': 24}
        row = run_case(case, 2, generate, stable=True)
        self.assertTrue(row['solved'])
        self.assertEqual(row['expression'], '(4*(3*(1*2)))')
        self.assertEqual(len(row['calls']), 6)
        bad = run_case(case, 2, lambda kind, current: ('{"proposals":[{"left":"t0","right":"t1","op":"+"},{"left":"t0","right":"missing","op":"+"}]}', {'at_output_limit': False}), stable=True)
        self.assertEqual(bad['error'], 'operand_identity')
        self.assertEqual(bad['states'], {})
        self.assertIsNone(bad['generated'])

    def test_opt_in_gate_precedes_asset_loading(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'result.json'
            env = dict(os.environ)
            env.pop('LOCAL_STRATEGY_E2E', None)
            result = subprocess.run([sys.executable, str(Path(__file__).with_name('model_search.py')),
                                     '--assets', directory, '--output', str(output)],
                                    env=env, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 2)
            self.assertIn(b'LOCAL_STRATEGY_E2E=1', result.stderr)
            self.assertFalse(output.exists())

    def test_exact_arithmetic_and_exclusive_operands(self):
        state = (Term(Fraction(8), '8', frozenset({0})), Term(Fraction(3), '3', frozenset({1})))
        child = transition(state, {'left': 0, 'right': 1, 'op': '/'})
        self.assertEqual(child[0].value, Fraction(8, 3))
        self.assertEqual(child[0].leaves, frozenset({0, 1}))
        self.assertEqual(state[0].value, 8)
        for step in ({'left': 0, 'right': 0, 'op': '+'}, {'left': True, 'right': 1, 'op': '+'},
                     {'left': 0, 'right': 1, 'op': '__import__'}, {'left': 0, 'right': 9, 'op': '+'}):
            with self.assertRaises(ValueError):
                transition(state, step)
        with self.assertRaises(ValueError):
            transition((state[0], Term(Fraction(0), '0', frozenset({1}))), {'left': 0, 'right': 1, 'op': '/'})

    def test_actual_engine_closes_fake_model_loop_and_rejects_bad_batch(self):
        def generate(kind, view):
            self.assertNotIn('id', view)
            if kind == 'evaluate':
                raw = '{"score":1}'
            else:
                right = 2 if len(view['terms']) == 3 else 1
                raw = json.dumps({'proposals': [{'left': 0, 'right': right, 'op': '*'}]})
            return raw, {'at_output_limit': False}
        case = {'id': 'p', 'numbers': [1, 2, 3, 4], 'target': 24}
        row = run_case(case, 2, generate)
        self.assertTrue(row['solved'])
        self.assertEqual(row['expression'], '(4*(3*(1*2)))')
        self.assertEqual(len(row['calls']), 6)
        def invalid(kind, view):
            return '{"proposals":[{"left":0,"right":1,"op":"+"},{"left":0,"right":8,"op":"+"}]}', {'at_output_limit': False}
        bad = run_case(case, 2, invalid)
        self.assertEqual(bad['error'], 'operand_index')
        self.assertEqual(bad['states'], {})
        self.assertFalse(bad['solved'])
        self.assertIsNone(bad['generated'])

    def test_recorded_raw_calls_replay_without_model_or_code_execution(self):
        base = Path(__file__).parent
        raw = (base / 'search-cases.json').read_bytes()
        cases = json.loads(raw)['cases']
        for version in ('v1', 'v2', 'v3', 'v4'):
            record = json.loads((base / 'testdata' / ('arithmetic-search-' + version + '-result.json')).read_text())
            self.assertTrue(record['complete'])
            self.assertEqual(record['case_sha256'], hashlib.sha256(raw).hexdigest())
            self.assertEqual(len(record['rows']), 8)
            for case, width in ((c, w) for c in cases for w in (1, 2)):
                saved = next(x for x in record['rows'] if x['case_id'] == case['id'] and x['width'] == width)
                calls = iter(saved['calls'])
                def replay(kind, current):
                    call = next(calls)
                    self.assertEqual(call['kind'], kind)
                    self.assertEqual(call['view'], current)
                    if version in ('v2', 'v3', 'v4'):
                        self.assertEqual(call['raw'], call['application_prefix'] + call['generated_text'])
                    return call['raw'], call
                actual = run_case(case, width, replay, stable=version in ('v3', 'v4'))
                for key in ('solved', 'reason', 'error', 'expression', 'states', 'scores', 'generated', 'evaluated', 'expanded'):
                    self.assertEqual(actual[key], saved[key])
                with self.assertRaises(StopIteration):
                    next(calls)

    def test_generated_goal_policy_replays_v4_without_new_model_calls(self):
        base = Path(__file__).parent
        cases = json.loads((base / 'search-cases.json').read_text())['cases']
        record = json.loads((base / 'testdata/arithmetic-search-v4-result.json').read_text())
        for saved in record['rows']:
            case = next(c for c in cases if c['id'] == saved['case_id'])
            calls = iter(saved['calls'])
            def replay(kind, current):
                call = next(calls)
                self.assertEqual((call['kind'], call['view']), (kind, current))
                return call['raw'], call
            actual = run_case(case, saved['width'], replay, stable=True, goal_policy='generated')
            for key in ('solved', 'reason', 'error', 'expression', 'states', 'scores', 'generated', 'evaluated', 'expanded'):
                self.assertEqual(actual[key], saved[key])
            with self.assertRaises(StopIteration):
                next(calls)


if __name__ == '__main__':
    unittest.main()
