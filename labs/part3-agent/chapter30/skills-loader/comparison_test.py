"""Offline protocol checks; no model import or inference."""
import json
import hashlib
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from compare_model import inspect_output, model_messages


class ComparisonChecks(unittest.TestCase):
    def test_cli_without_opt_in_does_not_load_assets_or_create_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'result.json'
            environment = dict(os.environ)
            environment.pop('SKILL_COMPARISON_E2E', None)
            process = subprocess.run([sys.executable, str(Path(__file__).with_name('compare_model.py')),
                                      '--assets', str(Path(directory) / 'missing-assets'), '--output', str(output)],
                                     env=environment, capture_output=True, timeout=10)
            self.assertEqual(process.returncode, 2)
            self.assertIn(b'SKILL_COMPARISON_E2E=1', process.stderr)
            self.assertFalse(output.exists())

    def test_shape_coverage_and_unknown_semantics_remain_separate(self):
        case = {'required_numbers': ['30'], 'evidence': [{'id': 'e1'}], 'expected_missing': []}
        correct = '{"summary":"Timeout is 30 seconds.","source_ids":["e1"],"missing":[]}'
        result = inspect_output(correct, False, case)
        self.assertTrue(result['shape_ok'] and result['coverage_ok'])
        self.assertEqual(result['task_quality'], 'unreviewed')
        for raw, limit in [(correct, True), (correct.replace('30', '300'), False), (correct.replace('e1', 'wrong'), False), (correct[:-1] + ',"Summary":"extra"}', False)]:
            self.assertFalse(inspect_output(raw, limit, case)['coverage_ok'])
        fabricated = correct.replace('Timeout is', 'Unsupported cause; timeout is')
        self.assertEqual(inspect_output(fabricated, False, case)['semantic_support'], 'unreviewed')

    def test_only_execution_projection_enters_model(self):
        case = {'id': 'secret-case-label', 'question': 'question', 'evidence': [], 'required_numbers': ['987654']}
        mode = {'catalog': [{'Name': 'skill', 'Description': 'task', 'HeaderHash': 'do-not-send'}],
                'skills': [{'Name': 'skill', 'Text': 'instructions', 'Digest': 'do-not-send'}], 'resources': []}
        for version in ('skills-comparison-v1', 'skills-comparison-v2'):
            raw = json.dumps(model_messages(case, mode, version))
            for value in ['secret-case-label', '987654', 'do-not-send', 'required_numbers']:
                self.assertNotIn(value, raw)
        with self.assertRaises(ValueError):
            model_messages(case, mode, 'unregistered')

    def test_recorded_outputs_replay_checks_without_claiming_live_effect(self):
        base = Path(__file__).parent
        files = [('comparison-cases.json', 'development-v1-inputs.json', 'development-v1-result.json'),
                 ('comparison-cases.json', 'development-v1-inputs.json', 'development-prompt-v2-result.json'),
                 ('holdout-cases.json', 'holdout-v1-inputs.json', 'holdout-prompt-v2-result.json')]
        for cases_file, inputs_file, results_file in files:
            with self.subTest(record=results_file):
                raw_cases = (base / cases_file).read_bytes()
                dataset = json.loads(raw_cases)
                labels = {c['id']: c for c in dataset['cases']}
                inputs = json.loads((base / 'testdata' / inputs_file).read_text())
                recorded = json.loads((base / 'testdata' / results_file).read_text())
                self.assertTrue(recorded['complete'])
                self.assertEqual(recorded['dataset'], inputs['version'])
                self.assertEqual(recorded['split'], dataset['split'])
                self.assertEqual(recorded['case_sha256'], hashlib.sha256(raw_cases).hexdigest())
                self.assertEqual(len(recorded['rows']), 12)
                pairs = {(c['id'], m['name']): m for c in inputs['cases'] for m in c['modes']}
                seen = set()
                for row in recorded['rows']:
                    key = (row['case_id'], row['mode'])
                    self.assertNotIn(key, seen)
                    seen.add(key)
                    self.assertEqual(row['loaded_bytes'], pairs[key]['used_bytes'])
                    checks = inspect_output(row['raw'], row['at_output_limit'], labels[row['case_id']])
                    for name, value in checks.items():
                        self.assertEqual(row[name], value)
                self.assertEqual(seen, set(pairs))


if __name__ == '__main__':
    unittest.main()
