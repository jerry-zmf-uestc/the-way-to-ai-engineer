import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from case_registry import inspect_registry, load_json


class RegistryChecks(unittest.TestCase):
    def test_current_public_definitions_are_pinned_without_qualification(self):
        _, registry = load_json(Path(__file__).with_name('case-registry.json'))
        report = inspect_registry(Path(__file__).resolve().parents[4], registry)
        self.assertEqual((report['dataset_count'], report['definition_count']), (16, 87))
        self.assertEqual(report['definition_counts_by_declared_kind'], {'model-input': 84, 'scoring-fixture': 3})
        self.assertIsNone(report['qualified_model_case_count'])
        self.assertTrue(all(row['qualification'] == 'review_pending' for row in report['datasets']))
        self.assertEqual(sum(row['definition_count'] for row in report['datasets'] if row['current_use'] == 'regression'), 24)
        self.assertTrue(any(row['source_split_counts'] == {'unspecified': 6} and 'holdout' in row['path'] for row in report['datasets']))

    def test_mutated_bytes_versions_ids_and_declarations_refuse(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / 'labs/part3-agent/example/cases.json'
            path.parent.mkdir(parents=True)
            data = json.dumps({'version': 'v1', 'cases': [{'id': 'a'}]}).encode()
            path.write_bytes(data)
            entry = {'path': 'labs/part3-agent/example/cases.json', 'sha256': hashlib.sha256(data).hexdigest(), 'version': 'v1', 'version_key': 'version', 'cases_key': 'cases', 'id_key': 'id', 'case_ids': ['a'], 'kind': 'model-input', 'family': 'example', 'current_use': 'review_pending', 'qualification': 'review_pending', 'use_evidence': []}
            base = {'schema': 2, 'version': 'course-case-registry-v2', 'datasets': [entry]}
            self.assertEqual(inspect_registry(root, base)['datasets'][0]['source_split_counts'], {'unspecified': 1})
            for key, value in [('version', 'v2'), ('case_ids', ['b']), ('cases_key', 'other'), ('qualification', 'qualified'), ('path', '../private.json'), ('sha256', '0' * 64)]:
                bad = copy.deepcopy(base)
                bad['datasets'][0][key] = value
                with self.assertRaises((ValueError, OSError)):
                    inspect_registry(root, bad)
            bad = copy.deepcopy(base)
            bad['datasets'].append(copy.deepcopy(entry))
            with self.assertRaises(ValueError):
                inspect_registry(root, bad)
            duplicate = json.dumps({'version': 'v1', 'cases': [{'id': 'a'}, {'id': 'a'}]}).encode()
            path.write_bytes(duplicate)
            bad = copy.deepcopy(base)
            bad['datasets'][0]['sha256'] = hashlib.sha256(duplicate).hexdigest()
            bad['datasets'][0]['case_ids'] = ['a', 'a']
            with self.assertRaises(ValueError):
                inspect_registry(root, bad)
            path.write_bytes(data + b' ')
            with self.assertRaises(ValueError):
                inspect_registry(root, base)
            path.unlink()
            outside = root / 'outside.json'
            outside.write_bytes(data)
            path.symlink_to(outside)
            with self.assertRaises(ValueError):
                inspect_registry(root, base)

    def test_current_use_needs_pinned_complete_record(self):
        _, registry = load_json(Path(__file__).with_name('case-registry.json'))
        root = Path(__file__).resolve().parents[4]
        row = next(e for e in registry['datasets'] if e['current_use'] == 'regression')
        for mutate in ('empty', 'hash', 'layout', 'state', 'escape'):
            bad = copy.deepcopy(registry)
            e = next(e for e in bad['datasets'] if e['path'] == row['path'])
            if mutate == 'empty': e['use_evidence'] = []
            if mutate == 'hash': e['use_evidence'][0]['sha256'] = '0' * 64
            if mutate == 'layout': e['use_evidence'][0]['id_key'] = 'unregistered'
            if mutate == 'state': e['current_use'] = 'qualified'
            if mutate == 'escape': e['use_evidence'][0]['path'] = '../private.json'
            with self.subTest(mutate=mutate), self.assertRaises((ValueError, OSError)):
                inspect_registry(root, bad)

    def test_missing_or_foreign_record_case_refuses_even_with_new_hash(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'labs/part3-agent/example/cases.json'
            source.parent.mkdir(parents=True)
            source.write_text(json.dumps({'version': 'v1', 'cases': [{'id': 'a'}, {'id': 'b'}]}))
            record = source.with_name('record.json')
            entry = {'path': 'labs/part3-agent/example/cases.json', 'sha256': hashlib.sha256(source.read_bytes()).hexdigest(), 'version': 'v1', 'version_key': 'version', 'cases_key': 'cases', 'id_key': 'id', 'case_ids': ['a', 'b'], 'kind': 'model-input', 'family': 'example', 'current_use': 'regression', 'qualification': 'review_pending', 'use_evidence': []}
            registry = {'schema': 2, 'version': 'course-case-registry-v2', 'datasets': [entry]}
            for ids in (['a', 'b', 'a'], ['a'], ['a', 'foreign']):
                record.write_text(json.dumps({'Rows': [{'CaseID': i} for i in ids]}))
                entry['use_evidence'] = [{'path': 'labs/part3-agent/example/record.json', 'sha256': hashlib.sha256(record.read_bytes()).hexdigest(), 'id_key': 'Rows.CaseID'}]
                if set(ids) == {'a', 'b'}:
                    self.assertEqual(inspect_registry(root, registry)['definition_count'], 2)
                else:
                    with self.assertRaises(ValueError):
                        inspect_registry(root, registry)

    def test_duplicate_json_fields_and_case_ids_refuse(self):
        with tempfile.TemporaryDirectory() as directory:
            file = Path(directory) / 'duplicate.json'
            file.write_text('{"version":"v1","version":"v2"}')
            with self.assertRaises(ValueError):
                load_json(file)


if __name__ == '__main__':
    unittest.main()
