import unittest
from project_captured_coverage import project


class ProjectionTest(unittest.TestCase):
    def test_fixed_actual_inputs_keep_labels_separate(self):
        value = project()
        self.assertEqual(value['Schema'], 1)
        self.assertEqual(len(value['Cases']), 3)
        self.assertEqual(len(value['Requirements']), 3)
        self.assertTrue(all(not r['CandidatesKnown'] for r in value['Runs']))
        self.assertEqual([r['Input'] for r in value['Runs']],
                         [['deployment@1', 'policy@1'], ['policy@1'], ['deployment@1']])
        self.assertTrue(all(r['InputKnown'] for r in value['Runs']))
        self.assertTrue(all('Groups' not in c for c in value['Cases']))


if __name__ == '__main__':
    unittest.main()
