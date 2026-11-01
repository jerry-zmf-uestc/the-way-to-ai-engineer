# coding: utf-8
import copy
import json
import unittest
from pathlib import Path
from prepare_diagnosis_review import prepare_packet, render_markdown, prepare_reference_packet, render_reference_markdown


class ReviewPacket(unittest.TestCase):
    def setUp(self):
        root = Path(__file__).parent
        self.dataset = json.loads((root / 'diagnosis-cases.json').read_text(encoding='utf-8'))
        self.record = json.loads((root / 'testdata/diagnosis-generation-v1.json').read_text(encoding='utf-8'))

    def test_labels_and_machine_scores_do_not_reach_review(self):
        packet = prepare_packet(self.dataset, self.record)
        self.assertEqual(len(packet['Cases']), 8)
        changed_dataset, changed_record = copy.deepcopy(self.dataset), copy.deepcopy(self.record)
        changed_dataset['cases'][0]['expected']['cause'] = 'changed-hidden-label'
        changed_record['Runs'][0]['Score']['Status'] = 'passed'
        self.assertEqual(packet, prepare_packet(changed_dataset, changed_record))
        for row in packet['Cases']:
            self.assertEqual(set(row['Task']), {'question', 'events', 'allowed_actions'})
            self.assertEqual(row['JudgmentTemplate']['Verdict'], 'unknown')
            self.assertEqual(row['Reviewer'], '')
        self.assertIn(self.record['Runs'][4]['Generation']['raw'], render_markdown(packet))

    def test_changed_input_or_dataset_rejected_and_answer_rebound(self):
        wrong = copy.deepcopy(self.record)
        wrong['Runs'][0]['Input']['Input'] = '{}'
        with self.assertRaises(ValueError):
            prepare_packet(self.dataset, wrong)
        wrong = copy.deepcopy(self.record)
        wrong['DatasetVersion'] = 'old'
        with self.assertRaises(ValueError):
            prepare_packet(self.dataset, wrong)
        changed = copy.deepcopy(self.record)
        changed['Runs'][0]['Generation']['raw'] += ' '
        before = prepare_packet(self.dataset, self.record)['Cases'][0]['JudgmentTemplate']['AnswerSHA256']
        after = prepare_packet(self.dataset, changed)['Cases'][0]['JudgmentTemplate']['AnswerSHA256']
        self.assertNotEqual(before, after)

    def test_generated_starters_match(self):
        root = Path(__file__).parent
        packet = prepare_packet(self.dataset, self.record)
        self.assertEqual((root / 'diagnosis-review.md').read_text(encoding='utf-8'), render_markdown(packet) + '\n')
        self.assertEqual(json.loads((root / 'testdata/diagnosis-review-template.json').read_text(encoding='utf-8')), packet)

    def test_reference_review_does_not_depend_on_model_or_expected(self):
        root = Path(__file__).parent
        packet = prepare_reference_packet(self.dataset)
        changed = copy.deepcopy(self.dataset)
        changed['cases'][0]['expected']['refs'] = ['not-a-reference']
        self.assertEqual(packet, prepare_reference_packet(changed))
        for row in packet['Cases']:
            self.assertFalse(row['Reviewed'])
            self.assertEqual(row['SufficientEventSets'], [])
            self.assertNotIn('AnswerRaw', row)
            self.assertNotIn('Score', row)
        changed['cases'][0]['question'] += ' Changed.'
        self.assertNotEqual(packet['Cases'][0]['TaskSHA256'], prepare_reference_packet(changed)['Cases'][0]['TaskSHA256'])
        self.assertEqual((root / 'diagnosis-reference-review.md').read_text(encoding='utf-8'), render_reference_markdown(packet) + '\n')
        self.assertEqual(json.loads((root / 'testdata/diagnosis-reference-review-template.json').read_text(encoding='utf-8')), packet)


if __name__ == '__main__':
    unittest.main()
