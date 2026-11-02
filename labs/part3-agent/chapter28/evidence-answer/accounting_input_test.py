# coding: utf-8
import json
import unittest
from local_generator import accounting_input


class AccountingProjection(unittest.TestCase):
    def test_only_question_and_bounded_facts_reach_model(self):
        view = {'question': 'Remaining synthetic budget?', 'facts': [{'ID': 'limit', 'Value': 100}]}
        request = {'question': json.dumps(view), 'evidence': []}
        messages = accounting_input(request, 'registered instructions')
        self.assertEqual(json.loads(messages[1]['content']), view)
        for changed in [dict(view, expected={'value': 50}),
                        dict(view, facts=[{'ID': 'limit', 'Value': True}]),
                        dict(view, facts=[{'ID': 'limit', 'Value': 100}, {'ID': 'limit', 'Value': 100}])]:
            with self.assertRaises(ValueError):
                accounting_input({'question': json.dumps(changed), 'evidence': []}, 'instructions')
        with self.assertRaises(ValueError):
            accounting_input({'question': '{"question":"ignored",' + request['question'][1:], 'evidence': []}, 'instructions')


if __name__ == '__main__':
    unittest.main()
