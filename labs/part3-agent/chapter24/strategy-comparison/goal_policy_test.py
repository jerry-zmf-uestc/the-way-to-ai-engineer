import unittest
from bounded_search import Limits, search


class GoalPolicyChecks(unittest.TestCase):
    def test_low_scored_completed_child_is_checked_before_pruning(self):
        def propose(path):
            return ['dead', 'answer'] if path[-1] == 'root' else []
        def score(path):
            return 9 if path[-1] == 'dead' else 0
        verify = lambda path: path[-1] == 'answer'
        limits = Limits(width=1, nodes=2)
        retained = search('root', propose, score, verify, limits)
        generated = search('root', propose, score, verify, limits, goal_policy='generated')
        self.assertEqual(retained.reason, 'exhausted')
        self.assertEqual(generated.reason, 'solved')
        self.assertEqual(generated.path, ('root', 'answer'))
        self.assertEqual((generated.generated, generated.evaluated), (2, 1))

    def test_goal_respects_node_budget_and_does_not_expand_or_score(self):
        seen = []
        def verify(path):
            seen.append(path[-1])
            return path[-1] == 'answer'
        def score(path):
            self.assertNotEqual(path[-1], 'answer')
            return 0
        result = search('root', lambda path: ['dead', 'answer'], score, verify,
                        Limits(width=1, nodes=1), goal_policy='generated')
        self.assertEqual(result.reason, 'node_budget')
        self.assertEqual(seen, ['root', 'dead'])
        root = search('answer', lambda path: self.fail('expanded goal'), score, verify,
                      goal_policy='generated')
        self.assertEqual((root.generated, root.evaluated, root.expanded), (0, 0, 0))

    def test_late_cancel_and_invalid_goal_result_cannot_solve(self):
        stopped = False
        def verify(path):
            nonlocal stopped
            if len(path) > 1:
                stopped = True
                return True
            return False
        result = search('root', lambda path: ['answer'], lambda path: self.fail('late goal evaluated'),
                        verify, cancelled=lambda: stopped, goal_policy='generated')
        self.assertEqual(result.reason, 'cancelled')
        self.assertIsNone(result.path)
        with self.assertRaisesRegex(ValueError, 'invalid goal result'):
            search('root', lambda path: ['answer'], lambda path: 0,
                   lambda path: False if len(path) == 1 else 1, goal_policy='generated')
        with self.assertRaisesRegex(ValueError, 'invalid goal policy'):
            search('root', lambda path: [], lambda path: 0, lambda path: False, goal_policy='unknown')

    def test_cycle_still_consumes_generation_without_verification(self):
        verified = []
        def verify(path):
            verified.append(path)
            return False
        result = search('root', lambda path: ['root'], lambda path: self.fail('cycle scored'),
                        verify, goal_policy='generated')
        self.assertEqual((result.reason, result.generated, result.evaluated), ('exhausted', 1, 0))
        self.assertEqual(verified, [('root',)])


if __name__ == '__main__':
    unittest.main()
