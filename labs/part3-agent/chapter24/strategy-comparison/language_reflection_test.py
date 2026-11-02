import json
import hashlib
from pathlib import Path
import unittest
from language_reflection import reflector_view, memo_from_generation, actor_attempt
from self_ask_model import score
from native_reflection import reflection_input, make_memo, select_memo, actor, task_fingerprint

class LanguageFeedbackChecks(unittest.TestCase):
    def test_native_language_record_replays_with_scope_and_actual_messages(self):
        root=Path(__file__).parent
        cases_raw=(root/'followup-independent-cases.json').read_bytes()
        prior_raw=(root/'testdata/native-independent-workflow-v1-result.json').read_bytes()
        saved=json.loads((root/'testdata/native-language-feedback-v1-result.json').read_text())
        self.assertTrue(saved['complete'])
        self.assertEqual(saved['case_sha256'],hashlib.sha256(cases_raw).hexdigest())
        self.assertEqual(saved['prior_sha256'],hashlib.sha256(prior_raw).hexdigest())
        case=next(c for c in json.loads(cases_raw)['cases'] if c['id']=='equally-current-conflict')
        prior=next(r for r in json.loads(prior_raw)['rows'] if r['case_id']==case['id'] and r['mode']=='self-ask')
        self.assertEqual(saved['reflection']['view'],reflection_input(case,prior))
        self.assertEqual(saved['memo'],make_memo(saved['reflection']['raw'],saved['reflection'],saved['scope'],saved['prior_sha256']))
        # The requested plain reflection became a JSON action hint; no authored advice replaces it.
        self.assertEqual(set(json.loads(saved['reflection']['raw'])),{'name','arguments'})
        task={k:case[k] for k in ('id','question','seed','corpus')}
        for row in saved['rows']:
            calls=iter(row['calls'])
            def replay(messages,tools):
                call=next(calls)
                self.assertEqual(messages,call['model_messages'])
                self.assertEqual(tools,call['model_tools'])
                return call['provider_raw'],call
            actual=actor(task,replay,saved['memo'] if row['mode']=='with_advice' else None,saved['scope'])
            for key in ('reads','questions','observations','answer','refused','error','repairs'):
                self.assertEqual(actual[key],row[key])
            self.assertEqual(score(case,actual),row['coverage_pass'])
            with self.assertRaises(StopIteration):next(calls)

    def test_native_reflection_input_has_only_observed_failure(self):
        case={'question':'Compare policy.','required_observed_ids':['north','south'],'corpus':{
            'north':{'text':'Hidden policy is 18 seconds.'}}}
        prior={'observations':[{'document':{'id':'root','text':'Two policies follow.'}}],
               'calls':[{'provider_raw':'actual native proposal','raw':'converted proposal'}],
               'error':'native_tool_count','coverage_pass':False}
        view=reflection_input(case,prior)
        self.assertEqual(view['proposals'],['actual native proposal'])
        for hidden in ('required_observed_ids','corpus','coverage_pass','Hidden policy is 18 seconds.'):
            self.assertNotIn(hidden,json.dumps(view))

    def test_native_advice_scope_attempt_budget_copy_and_no_permission(self):
        task={'id':'hidden-id','question':'Q','seed':'root','corpus':{}}
        scope={'task_fingerprint':task_fingerprint(task),'data_version':'v1','tool_version':'native-v2','model_revision':'model-v1'}
        memo=make_memo('Use one call at a time.',{'at_output_limit':False},scope,'a'*64)
        selected=select_memo(memo,scope,2)
        selected['scope']['data_version']='mutated'
        self.assertEqual(memo['scope'],scope)
        for expected,attempt in [(dict(scope,data_version='v2'),2),(scope,1),(scope,3)]:
            with self.assertRaises(ValueError):select_memo(memo,expected,attempt)
        with self.assertRaises(ValueError):select_memo(dict(memo,tool_permissions=['shell']),scope,2)
        with self.assertRaises(ValueError):select_memo(dict(memo,advice='x'*1025),scope,2)
        for raw,limited in [('',False),('advice',True)]:
            with self.assertRaises(ValueError):make_memo(raw,{'at_output_limit':limited},scope,'a'*64)
        with self.assertRaisesRegex(ValueError,'task_scope_mismatch'):
            actor(dict(task,question='different task'),lambda *_:self.fail('must not invoke model'),memo,scope)
        def complete(messages,tools):
            self.assertIn('unverified_suggestion',json.dumps(messages))
            self.assertNotIn('hidden-id',json.dumps(messages))
            return '<tool_call>{"name":"lookup","arguments":{"question":"q","reference":"outside"}}</tool_call>',{'at_output_limit':False}
        row=actor(task,complete,memo,scope)
        self.assertEqual(row['reads'],0)
        self.assertEqual(row['error'],'unobserved_reference')

    def test_feedback_projection_keeps_gold_out(self):
        case={'question':'What is timeout?','required_terms':['31'],'expected_refuse':False}
        prior={'observations':[{'document':{'text':'directory only'}}],'calls':[{'raw':'bad proposal'}],'error':'answer_shape'}
        view=reflector_view(case,prior)
        self.assertNotIn('required_terms',json.dumps(view))
        self.assertNotIn('31',json.dumps(view))
        self.assertEqual(view['run_error'],'answer_shape')

    def test_incomplete_advice_is_not_replaced_by_author_text(self):
        for raw,limited in [('',False),('suggestion',True),('x'*1025,False)]:
            with self.assertRaises(ValueError):memo_from_generation(raw,{'at_output_limit':limited},'source-sha')

    def test_next_actor_gets_unverified_advice_and_records_real_view(self):
        memo=memo_from_generation('Read the linked policy.',{'at_output_limit':False},'source-sha')
        task={'id':'outside-model','question':'Q','seed':'root','corpus':{}}
        def generate(view):
            self.assertEqual(view['prior_suggestion'],memo)
            self.assertEqual(view['prior_suggestion']['status'],'unverified_suggestion')
            return json.dumps({'action':'ask','question':'q','reference':'unknown'}),{'at_output_limit':False}
        row=actor_attempt(task,generate,memo)
        self.assertEqual(row['error'],'unobserved_reference')
        self.assertEqual(row['reads'],0)
        self.assertEqual(row['calls'][0]['model_view']['prior_suggestion'],memo)

    def test_actual_language_record_replays_without_inference(self):
        root=Path(__file__).parent
        raw=(root/'self-ask-cases.json').read_bytes()
        previous=(root/'testdata/followup-development-v4-result.json').read_bytes()
        saved=json.loads((root/'testdata/language-feedback-v1-result.json').read_text())
        self.assertTrue(saved['complete'])
        self.assertEqual(saved['case_sha256'],hashlib.sha256(raw).hexdigest())
        self.assertEqual(saved['prior_sha256'],hashlib.sha256(previous).hexdigest())
        case=next(c for c in json.loads(raw)['cases'] if c['id']=='team-timeout')
        prior=next(r for r in json.loads(previous)['rows'] if r['case_id']==case['id'] and r['mode']=='self-ask')
        self.assertEqual(saved['reflection']['view'],reflector_view(case,prior))
        self.assertEqual(saved['memo'],memo_from_generation(saved['reflection']['raw'],saved['reflection'],saved['prior_sha256']))
        task={k:case[k] for k in ('id','question','seed','corpus')}
        for row in saved['rows']:
            calls=iter(row['calls'])
            def replay(view):
                call=next(calls)
                self.assertEqual(view,call['model_view'])
                self.assertEqual(call['raw'],call['application_prefix']+call['generated_text'])
                return call['raw'],call
            actual=actor_attempt(task,replay,saved['memo'] if row['mode']=='with_advice' else None)
            for key in ('error','reads','answer','refused','observations','questions'):
                self.assertEqual(actual[key],row[key])
            self.assertEqual(score(case,actual),row['coverage_pass'])
            with self.assertRaises(StopIteration):next(calls)

if __name__=='__main__':unittest.main()
