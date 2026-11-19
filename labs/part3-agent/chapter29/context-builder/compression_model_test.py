"""Offline projection and truncation checks; no model/runtime dependency."""
import json
import hashlib
import os
from pathlib import Path
import subprocess
import sys
import unittest
from compression_model import execute, views


class CompressionChecks(unittest.TestCase):
    def test_projection_and_raw_retention(self):
        case = {'snapshot_version': 'v2', 'history': [{'id':str(n)} for n in range(5)],
                'authoritative_snapshot': {'hidden':'gold'}}
        seen = []
        def generate(view):
            seen.append(view)
            return '```invalid```', {'at_output_limit':False}
        judged = []
        rows = execute(case, generate, lambda raw, mode: judged.append(raw) or 'invalid_shape')
        self.assertEqual(judged, ['```invalid```','```invalid```'])
        self.assertEqual(len(seen[0]['history']),5)
        self.assertEqual(len(seen[1]['history']),3)
        self.assertNotIn('authoritative_snapshot', json.dumps(seen))
        self.assertTrue(all(r['raw']=='```invalid```' for r in rows))

    def test_truncation_never_enters_validator(self):
        case = {'snapshot_version':'v2','history':[]}
        rows = execute(case, lambda _: ('{}', {'at_output_limit':True}),
                       lambda *_: self.fail('truncated candidate judged'))
        self.assertTrue(all(r['validation'] is None and r['review']=='output_limit' for r in rows))

    def test_explicit_gate(self):
        env = dict(os.environ)
        env.pop('CONTEXT_COMPRESSION_E2E',None)
        process = subprocess.run([sys.executable,str(Path(__file__).with_name('compression_model.py'))],
                                 env=env,capture_output=True,text=True,timeout=5)
        self.assertNotEqual(process.returncode,0)
        self.assertIn('CONTEXT_COMPRESSION_E2E=1', process.stderr)

    def test_frozen_generation_records(self):
        root = Path(__file__).parent
        case_raw = (root/'compression-case.json').read_bytes()
        case = json.loads(case_raw)
        report = json.loads((root/'testdata/compression-development-v1-result.json').read_text())
        self.assertTrue(report['complete'])
        self.assertEqual(report['case_sha256'], hashlib.sha256(case_raw).hexdigest())
        self.assertEqual(report['split'], 'development')
        self.assertEqual(len(report['rows']), 2)
        for row, (mode, view) in zip(report['rows'], views(case)):
            self.assertEqual(row['mode'], mode)
            self.assertEqual(row['view'], view)
            usage = row['usage']
            self.assertEqual(row['raw'], usage['application_prefix']+usage['generated_text'])
            self.assertTrue(0 < usage['input_tokens'] <= 1536)
            self.assertTrue(0 < usage['output_tokens'] < 320)
            self.assertFalse(usage['at_output_limit'])
            self.assertEqual(row['validation'], 'stage=invalid_shape accepted=false published_facts=0')
            self.assertEqual((root/f'testdata/compression-development-v1-result-{mode}.txt').read_text(), row['raw'])


if __name__=='__main__':
    unittest.main()
