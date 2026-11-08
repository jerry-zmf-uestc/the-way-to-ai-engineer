"""Read pinned public definitions; consistency is not case qualification."""
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import sys


def unique_fields(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate_field')
        result[key] = value
    return result


def load_json(path):
    with path.open('rb') as source:
        data = source.read((512 << 10) + 1)
    if len(data) > 512 << 10:
        raise ValueError('file_bound')
    return data, json.loads(data.decode('utf-8'), object_pairs_hook=unique_fields)


def inspect_registry(root, registry):
    if not isinstance(registry, dict) or set(registry) != {'schema', 'version', 'datasets'} or type(registry['schema']) is not int or registry['schema'] != 2 or registry['version'] != 'course-case-registry-v2':
        raise ValueError('registry_shape')
    entries = registry['datasets']
    if not isinstance(entries, list) or not 0 < len(entries) <= 20:
        raise ValueError('dataset_bound')
    root = root.resolve()
    paths, versions, rows, counts = set(), set(), [], {}
    fields = {'path', 'sha256', 'version', 'version_key', 'cases_key', 'id_key', 'case_ids', 'kind', 'family', 'current_use', 'qualification', 'use_evidence'}
    for entry in entries:
        if not isinstance(entry, dict) or set(entry) != fields or any(not isinstance(entry[k], str) or not entry[k] or len(entry[k]) > 256 for k in fields - {'id_key', 'case_ids', 'use_evidence'}):
            raise ValueError('entry_shape')
        name = entry['path']
        relative = PurePosixPath(name)
        if relative.is_absolute() or '..' in relative.parts or relative.as_posix() != name or not name.startswith('labs/part3-agent/') or relative.suffix != '.json' or name in paths or entry['version'] in versions:
            raise ValueError('source_identity')
        path = (root / name).resolve()
        path.relative_to(root / 'labs/part3-agent')
        if not path.is_file():
            raise ValueError('source_file')
        paths.add(name)
        versions.add(entry['version'])
        if not re.fullmatch('[0-9a-f]{64}', entry['sha256']) or entry['kind'] not in ('model-input', 'contract-case', 'scoring-fixture') or entry['current_use'] not in ('review_pending', 'regression') or entry['qualification'] != 'review_pending':
            raise ValueError('declaration')
        data, source = load_json(path)
        if hashlib.sha256(data).hexdigest() != entry['sha256'] or not isinstance(source, dict) or entry['version_key'] not in ('version', 'Version') or source.get(entry['version_key']) != entry['version']:
            raise ValueError('source_drift')
        key = entry['cases_key']
        cases = source.get(key)
        if key == 'history':
            if entry['id_key'] is not None or not isinstance(cases, list) or not cases or entry['case_ids'] != ['single-history-task']:
                raise ValueError('history_layout')
            ids, splits = ['single-history-task'], [source.get('split')]
        else:
            if key not in ('cases', 'Cases') or entry['id_key'] not in ('id', 'ID', 'scenario', 'Input.ID') or not isinstance(cases, list) or not 0 < len(cases) <= 128 or any(not isinstance(c, dict) for c in cases):
                raise ValueError('case_layout')
            if entry['id_key'] == 'Input.ID':
                if any(not isinstance(c.get('Input'), dict) for c in cases):
                    raise ValueError('input_layout')
                ids = [c['Input'].get('ID') for c in cases]
            else:
                ids = [c.get(entry['id_key']) for c in cases]
            splits = [c.get('split', c.get('Split', source.get('split'))) for c in cases]
        valid_ids = all(isinstance(i, str) and i and len(i) <= 128 for i in ids)
        valid_splits = all(s is None or isinstance(s, str) and s and len(s) <= 32 for s in splits)
        if not valid_ids or len(set(ids)) != len(ids) or ids != entry['case_ids'] or not valid_splits:
            raise ValueError('case_identity')
        use_evidence = entry['use_evidence']
        if not isinstance(use_evidence, list) or len(use_evidence) > 4 or (entry['current_use'] == 'regression') != bool(use_evidence):
            raise ValueError('use_evidence_required')
        evidence_paths = set()
        for evidence in use_evidence:
            if not isinstance(evidence, dict) or set(evidence) != {'path', 'sha256', 'id_key'} or evidence['id_key'] not in ('Rows.CaseID', 'rows.case_id'):
                raise ValueError('use_evidence_shape')
            name = evidence['path']
            if not isinstance(name, str) or len(name) > 256:
                raise ValueError('use_evidence_path')
            relative = PurePosixPath(name)
            if relative.is_absolute() or '..' in relative.parts or relative.as_posix() != name or not name.startswith('labs/part3-agent/') or relative.suffix != '.json' or name in evidence_paths:
                raise ValueError('use_evidence_path')
            evidence_paths.add(name)
            evidence_path = (root / name).resolve()
            evidence_path.relative_to(root / 'labs/part3-agent')
            if not evidence_path.is_file():
                raise ValueError('use_evidence_file')
            raw_record, record = load_json(evidence_path)
            if not isinstance(evidence['sha256'], str) or not re.fullmatch('[0-9a-f]{64}', evidence['sha256']) or hashlib.sha256(raw_record).hexdigest() != evidence['sha256'] or not isinstance(record, dict):
                raise ValueError('use_evidence_drift')
            row_key, id_key = evidence['id_key'].split('.')
            recorded = record.get(row_key)
            if not isinstance(recorded, list) or not 0 < len(recorded) <= 128 or any(not isinstance(row, dict) or not isinstance(row.get(id_key), str) for row in recorded) or {row[id_key] for row in recorded} != set(ids):
                raise ValueError('use_evidence_cases')
            if 'case_sha256' in record and record['case_sha256'] != entry['sha256']:
                raise ValueError('use_evidence_source')
        split_counts = {}
        for split in splits:
            label = split if split is not None else 'unspecified'
            split_counts[label] = split_counts.get(label, 0) + 1
        counts[entry['kind']] = counts.get(entry['kind'], 0) + len(ids)
        rows.append({'path': entry['path'], 'version': entry['version'], 'sha256': entry['sha256'], 'case_ids': ids, 'definition_count': len(ids), 'declared_kind': entry['kind'], 'declared_family': entry['family'], 'source_split_counts': split_counts, 'current_use': entry['current_use'], 'use_evidence': use_evidence, 'qualification': 'review_pending'})
    return {'schema': 2, 'registry_version': registry['version'], 'dataset_count': len(rows), 'definition_count': sum(counts.values()), 'definition_counts_by_declared_kind': counts, 'qualified_model_case_count': None, 'datasets': rows}


if __name__ == '__main__':
    try:
        if len(sys.argv) != 1:
            raise ValueError('arguments')
        _, registry = load_json(Path(__file__).with_name('case-registry.json'))
        report = inspect_registry(Path(__file__).resolve().parents[4], registry)
        print(json.dumps(report, ensure_ascii=False, allow_nan=False))
    except (ValueError, OSError, TypeError, KeyError):
        print('case registry: invalid registry or source', file=sys.stderr)
        sys.exit(2)
