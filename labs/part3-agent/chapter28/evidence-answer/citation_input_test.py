import json
import unittest
from local_generator import joint_input_v3


class CitationInputChecks(unittest.TestCase):
    def test_projection_uses_output_identity_without_scoring_fields(self):
        request = {'question': 'current timeout?', 'evidence': [
            {'id': 'policy', 'version': '1', 'text': 'conditional rule', 'tenant': 'private'}],
            'expected': 'hidden answer', 'case_id': 'private-case'}
        before = json.loads(json.dumps(request))
        messages = joint_input_v3(request, 'fixed instructions')
        self.assertEqual(messages[0], {'role': 'system', 'content': 'fixed instructions'})
        self.assertEqual(json.loads(messages[1]['content']), {
            'question': 'current timeout?', 'evidence': [{'evidence_id': 'policy', 'version': '1', 'text': 'conditional rule'}]})
        self.assertEqual(request, before)
        self.assertNotIn('hidden answer', json.dumps(messages))
        self.assertNotIn('private', json.dumps(messages))


if __name__ == '__main__':
    unittest.main()
