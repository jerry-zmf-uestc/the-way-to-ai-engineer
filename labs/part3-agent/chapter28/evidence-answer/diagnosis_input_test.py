# coding: utf-8
import json
import unittest
from local_generator import diagnosis_input


class DiagnosisProjection(unittest.TestCase):
    def test_scoring_fields_and_duplicates_rejected(self):
        view = {'question': 'What happened?', 'events': [{'id': 'e1', 'text': 'worker did not exit'}], 'allowed_actions': ['wait']}
        request = {'question': json.dumps(view), 'evidence': []}
        messages = diagnosis_input(request, 'registered instructions')
        self.assertEqual(json.loads(messages[1]['content']), view)
        changed = dict(view, expected={'action': 'wait'})
        with self.assertRaises(ValueError):
            diagnosis_input({'question': json.dumps(changed), 'evidence': []}, 'instructions')
        with self.assertRaises(ValueError):
            diagnosis_input({'question': '{"question":"ignored",' + request['question'][1:], 'evidence': []}, 'instructions')
        with self.assertRaises(ValueError):
            diagnosis_input(dict(request, evidence=[{'id': 'hidden'}]), 'instructions')


if __name__ == '__main__':
    unittest.main()
