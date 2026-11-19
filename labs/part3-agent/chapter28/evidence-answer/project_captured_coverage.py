"""Project fixed captured public inputs to the published coverage protocol."""
import hashlib
import json
from pathlib import Path


def project():
    source = Path(__file__).with_name('testdata') / 'joint-captured-1.7b-v2.json'
    raw = source.read_bytes()
    if hashlib.sha256(raw).hexdigest() != '2e312ac2ce7b56eeb7814148447ebde9a3722f163f0f632b68a38b6fc770f604':
        raise ValueError('captured record version changed')
    records = json.loads(raw)
    if [r['case_id'] for r in records] != ['both-premises', 'policy-only', 'deployment-only']:
        raise ValueError('case identities changed')
    payload = {'Schema': 1, 'DatasetVersion': 'joint-input-capture-v1',
               'RequirementVersion': 'current-timeout-premises-v1',
               'Cases': [], 'Requirements': [], 'Runs': []}
    for r in records:
        submitted = r['submitted']
        batch = r['generation']['batch']
        # These fixed public strings are ASCII without HTML characters; this is
        # the exact Go payload here, not a universal JSON canonicalization rule.
        encoded = json.dumps(submitted, ensure_ascii=False, separators=(',', ':')).encode()
        if r['schema'] != 1 or len(submitted) != 1 or batch['profile'] != 'qwen3-1.7b' or batch['prompt_version'] != 'rag-joint-v2' or hashlib.sha256(encoded).hexdigest() != batch['input_sha256']:
            raise ValueError('submission identity or digest mismatch')
        case_id = r['case_id']
        payload['Cases'].append({'ID': case_id, 'Split': 'development',
                                 'Input': submitted[0]['question'],
                                 'AllowedSources': ['deployment@1', 'policy@1']})
        payload['Requirements'].append({'CaseID': case_id, 'Groups': [
            {'ID': 'current-state', 'Alternatives': ['deployment@1']},
            {'ID': 'applicable-rule', 'Alternatives': ['policy@1']}]})
        payload['Runs'].append({'CaseID': case_id,
                                'RunID': 'captured-1.7b-v2/' + case_id,
                                'CandidatesKnown': False, 'InputKnown': True,
                                'Input': [e['id'] + '@' + e['version'] for e in submitted[0]['evidence']]})
    return payload


if __name__ == '__main__':
    print(json.dumps(project(), ensure_ascii=False))
