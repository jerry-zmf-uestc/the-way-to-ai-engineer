import json
import hashlib
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from self_ask_model import execute, score, v5_model_view, v6_model_view, registered_cases, question_reference
from local_strategies import model_lock, load_model
from native_self_ask import parse_native, native_input, adapt_generate, workflow_generate


class FollowupChecks(unittest.TestCase):
    def test_native_query_binding_original_record_replays(self):
        base = Path(__file__).parent
        data, dataset = registered_cases('development-v1')
        record = json.loads((base / 'testdata/native-query-binding-v3-result.json').read_text())
        self.assertTrue(record['complete'])
        self.assertEqual(record['prompt_version'], 'native-self-ask-v3')
        self.assertEqual(record['case_sha256'], hashlib.sha256(data).hexdigest())
        self.assertEqual(len(record['rows']), 8)
        for saved in record['rows']:
            case = next(c for c in dataset['cases'] if c['id'] == saved['case_id'])
            task = {k: case[k] for k in ('id', 'question', 'seed', 'corpus')}
            calls = iter(saved['calls'])
            def replay(view, mode):
                call = next(calls)
                self.assertEqual(view, call['view'])
                messages, tools = native_input(view, 'v3')
                self.assertEqual(messages, call['model_messages'])
                self.assertEqual(tools, call['model_tools'])
                return call['raw'], call
            row = execute(task, saved['mode'], replay, query_lookup=True,
                          repair_protocol=True, expose_budget=True)
            for key in ('answer', 'refused', 'error', 'reads', 'questions', 'observations', 'repairs'):
                self.assertEqual(row[key], saved[key])
            self.assertEqual(score(case, row), saved['coverage_pass'])
            with self.assertRaises(StopIteration):
                next(calls)

    def test_question_selects_only_discovered_matching_evidence(self):
        corpus = {
            'root': {'id': 'root', 'text': 'Cedar service directory.', 'references': ['a-timeout', 'b-retry']},
            'a-timeout': {'id': 'a-timeout', 'text': 'Cedar request timeout is 30 seconds.', 'references': []},
            'b-retry': {'id': 'b-retry', 'text': 'Cedar retry count is 2.', 'references': []},
            'hidden': {'id': 'hidden', 'text': 'Cedar retry count is 99.', 'references': []},
        }
        remaining = ['a-timeout', 'b-retry']
        self.assertEqual(question_reference('request timeout seconds?', remaining, corpus), 'a-timeout')
        self.assertEqual(question_reference('retry count?', remaining, corpus), 'b-retry')
        with self.assertRaisesRegex(ValueError, 'query_no_match'):
            question_reference('unrelated fee?', remaining, corpus)
        task = {'id': 'query', 'question': 'Cedar timeout and retry?', 'seed': 'root', 'corpus': corpus}
        requests = iter([
            {'action': 'ask', 'question': 'Cedar service directory?'},
            {'action': 'ask', 'question': 'retry count?'},
            {'action': 'refuse', 'reason': 'insufficient_evidence'},
        ])
        def generate(view, mode):
            return json.dumps(next(requests)), {'at_output_limit': False}
        row = execute(task, 'self-ask', generate, query_lookup=True)
        self.assertIsNone(row['error'])
        self.assertEqual([o['document']['id'] for o in row['observations']], ['root', 'b-retry'])
        self.assertEqual([q['binding'] for q in row['questions']], ['application_query_match'] * 2)

    def test_native_query_schema_and_history_do_not_claim_model_reference(self):
        view = {'question': 'Cedar?', 'available_references': ['retry'], 'queries_remaining': 2,
                'observations': [{'follow_up': 'Cedar directory?', 'document': {
                    'id': 'root', 'text': 'Cedar directory.', 'references': ['retry']}}]}
        messages, tools = native_input(view, 'v3')
        self.assertEqual(tools[0]['function']['parameters']['required'], ['question'])
        self.assertEqual(messages[2]['tool_calls'][0]['function']['arguments'], {'question': 'Cedar directory?'})
        task = {'id': 'bad', 'question': 'q', 'seed': 'root', 'corpus': {}}
        row = execute(task, 'self-ask', lambda *_: (json.dumps({'action': 'ask', 'question': 'q', 'reference': 'root'}), {'at_output_limit': False}), query_lookup=True)
        self.assertEqual(row['error'], 'query_reference_not_allowed')
        self.assertEqual(row['reads'], 0)

    def test_first_unused_task_workflow_record_replays_with_origins(self):
        base = Path(__file__).parent
        raw_cases, dataset = registered_cases('independent-v1')
        record=json.loads((base/'testdata/native-independent-workflow-v1-result.json').read_text())
        self.assertTrue(record['complete'])
        self.assertEqual(record['case_sha256'],hashlib.sha256(raw_cases).hexdigest())
        self.assertEqual(record['scorer_version'],'term-quote-observed-ids-v3')
        self.assertEqual(len(record['rows']),12)
        for saved in record['rows']:
            case=next(x for x in dataset['cases'] if x['id']==saved['case_id'])
            calls=iter(saved['calls'])
            def replay(view,mode):
                call=next(calls)
                self.assertEqual(view,call['view'])
                def complete(messages,tools):
                    self.assertEqual(call['origin'],'model')
                    self.assertEqual(messages,call['model_messages'])
                    self.assertEqual(tools,call['model_tools'])
                    return call['provider_raw'],call
                raw,meta=(workflow_generate(view,complete) if mode=='fixed-workflow' else adapt_generate(view,complete,'v2'))
                self.assertEqual(raw,call['raw'])
                self.assertEqual(meta['origin'],call['origin'])
                if meta['origin']=='application_workflow':
                    self.assertNotIn('provider_raw',meta)
                    self.assertEqual(meta['input_tokens'],0)
                return raw,meta
            row=execute({k:case[k] for k in ('id','question','seed','corpus')},saved['mode'],replay,
                        repair_protocol=True,expose_budget=True)
            for key in ('reads','questions','observations','answer','refused','error','repairs'):
                self.assertEqual(row[key],saved[key])
            self.assertEqual(score(case,row),saved['coverage_pass'])
            with self.assertRaises(StopIteration):next(calls)

    def test_workflow_follows_discovered_order_and_only_synthesis_uses_model(self):
        case = {'id':'hidden-workflow', 'question':'Read relevant facts.', 'seed':'root', 'corpus':{
            'root':{'id':'root','text':'Two branches follow.','references':['z-first','a-second']},
            'z-first':{'id':'z-first','text':'First branch is 31.','references':['nested']},
            'a-second':{'id':'a-second','text':'Second branch is 42.','references':[]},
            'nested':{'id':'nested','text':'Later branch.','references':[]}}}
        invocations = []
        def complete(messages, tools):
            self.assertEqual(tools, [])
            self.assertNotIn('hidden-workflow',json.dumps(messages))
            self.assertNotIn('Later branch.',json.dumps(messages))
            invocations.append(messages)
            return json.dumps({'action':'answer','answer':'31 and 42','citations':[
                {'id':'z-first','quote':'First branch is 31.'},
                {'id':'a-second','quote':'Second branch is 42.'}]}), {'at_output_limit':False}
        row = execute(case,'fixed-workflow',lambda view,_:workflow_generate(view,complete),
                      repair_protocol=True,expose_budget=True)
        self.assertIsNone(row['error'])
        self.assertEqual([q['reference'] for q in row['questions']],['root','z-first','a-second'])
        self.assertEqual(len(invocations),1)
        self.assertEqual([c['origin'] for c in row['calls']],['application_workflow']*3+['model'])
        self.assertTrue(score(dict(case,required_terms=['31','42'],expected_refuse=False),row))

    def test_conflict_refusal_requires_both_actual_documents(self):
        case = {'expected_refuse':True,'required_terms':[], 'required_observed_ids':['north','south']}
        row = {'error':None,'refused':True,'answer':'','reads':1,'citations':[],
               'observations':[{'document':{'id':'directory'}}]}
        self.assertFalse(score(case,row))
        row['observations'].extend([{'document':{'id':'north'}},{'document':{'id':'south'}}])
        row['reads']=3
        self.assertTrue(score(case,row))

    def test_datasets_are_registered_and_scoring_fields_stay_outside_inputs(self):
        for name in ('development-v1','independent-v1'):
            raw, dataset = registered_cases(name)
            self.assertEqual(len(dataset['cases']),4)
            for case in dataset['cases']:
                view={'question':case['question'],'available_references':[case['seed']],
                      'observations':[],'queries_remaining':3}
                messages, tools = native_input(view,'v2')
                for hidden in ('required_terms','required_observed_ids','expected_refuse','family','corpus'):
                    self.assertNotIn(hidden,json.dumps(messages+tools))
        with self.assertRaisesRegex(ValueError,'unregistered dataset'):
            registered_cases('../other.json')

    def test_native_v2_quoted_tool_tags_remain_final_data(self):
        quote = 'The literal <tool_call>31</tool_call> is document text.'
        raw = json.dumps({'action': 'answer', 'answer': '31', 'citations': [{'id': 'doc', 'quote': quote}]})
        self.assertEqual(parse_native(raw, 'v2')['citations'][0]['quote'], quote)
        query = '<tool_call>'+json.dumps({'name':'lookup','arguments':{
            'question':'What does literal <tool_call></tool_call> mean?', 'reference':'doc'}})+'</tool_call>'
        self.assertEqual(parse_native(query, 'v2')['question'], 'What does literal <tool_call></tool_call> mean?')

    def test_native_cli_requires_explicit_gate(self):
        with tempfile.TemporaryDirectory() as directory:
            env = dict(os.environ)
            env.pop('LOCAL_STRATEGY_E2E', None)
            result = subprocess.run([sys.executable, str(Path(__file__).with_name('native_self_ask.py')),
                                     '--assets', directory, '--output', str(Path(directory)/'result.json')],
                                    env=env, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 2)
            self.assertFalse((Path(directory)/'result.json').exists())
    def test_native_v2_final_json_has_no_tool_effect_or_prose_trigger(self):
        case = {'id': 'c', 'question': 'q', 'seed': 'root', 'corpus': {
            'root': {'id': 'root', 'text': 'Timeout is 31 seconds.', 'references': []}}}
        replies = iter(['<tool_call>{"name":"lookup","arguments":{"question":"What is timeout?","reference":"root"}}</tool_call>',
                        'The timeout is 31 seconds.',
                        '{"action":"answer","answer":"31 seconds","citations":[{"id":"root","quote":"Timeout is 31 seconds."}]}'])
        def complete(messages, tools):
            self.assertNotIn('answer', [x['function']['name'] for x in tools])
            return next(replies), {'at_output_limit': False}
        row = execute(case, 'self-ask', lambda view, _: adapt_generate(view, complete, 'v2'),
                      repair_protocol=True, expose_budget=True)
        self.assertIsNone(row['error'])
        self.assertEqual(row['reads'], 1)
        self.assertEqual(row['repairs'], [{'code': 'json_format'}])
        self.assertTrue(score(dict(case, required_terms=['31'], expected_refuse=False), row))
        raw = '{"action":"ask","question":"q","reference":"root"}'
        row = execute(case, 'self-ask', lambda view, _: adapt_generate(view,
            lambda *_: (raw, {'at_output_limit': False}), 'v2'), repair_protocol=True, expose_budget=True)
        self.assertEqual(row['reads'], 0)
        self.assertEqual(row['error'], 'native_final_action')

    def test_native_actual_records_replay_without_weights(self):
        base = Path(__file__).parent
        cases = json.loads((base/'self-ask-cases.json').read_text())['cases']
        for size, protocol in (('1.7b','v1'),('1.7b','v2'),('0.6b','v2')):
            record = json.loads((base/'testdata'/('native-followup-'+size+'-'+protocol+'-result.json')).read_text())
            self.assertTrue(record['complete'])
            self.assertEqual(record['model_revision'], model_lock('qwen3-'+size)['revision'])
            self.assertEqual(record['prompt_version'], 'native-self-ask-'+protocol)
            self.assertEqual(record['case_sha256'], hashlib.sha256((base/'self-ask-cases.json').read_bytes()).hexdigest())
            self.assertEqual(len(record['rows']),8)
            for saved in record['rows']:
                case = next(c for c in cases if c['id'] == saved['case_id'])
                calls = iter(saved['calls'])
                def complete(messages, tools):
                    call = next(calls)
                    self.assertEqual(messages, call['model_messages'])
                    self.assertEqual(tools, call['model_tools'])
                    return call['provider_raw'], call
                row = execute({k: case[k] for k in ('id','question','seed','corpus')}, saved['mode'],
                              lambda view, _: adapt_generate(view, complete, protocol), repair_protocol=True, expose_budget=True)
                for key in ('reads','questions','observations','answer','refused','error','repairs'):
                    self.assertEqual(row[key], saved[key])
                self.assertEqual(score(case,row),saved['coverage_pass'])
                with self.assertRaises(StopIteration):next(calls)

    def test_native_two_hop_messages_preserve_actual_questions_and_results(self):
        case = {'id': 'hidden-native-case', 'question': 'What is the service timeout?', 'seed': 'directory',
                'corpus': {'directory': {'id': 'directory', 'text': 'Quartz config follows.', 'references': ['config']},
                           'config': {'id': 'config', 'text': 'Quartz timeout is 31 seconds.', 'references': []}}}
        proposed = [
            {'name': 'lookup', 'arguments': {'question': 'Which service and configuration?', 'reference': 'directory'}},
            {'name': 'lookup', 'arguments': {'question': 'What is Quartz timeout?', 'reference': 'config'}},
            {'name': 'answer', 'arguments': {'answer': '31 seconds', 'citations': [{'id': 'config', 'quote': 'Quartz timeout is 31 seconds.'}]}},
        ]
        calls = []
        def complete(messages, tools):
            self.assertNotIn('hidden-native-case', json.dumps(messages))
            self.assertNotIn('required_terms', json.dumps(messages))
            if calls:
                self.assertEqual(messages[2]['role'], 'assistant')
                self.assertEqual(messages[2]['tool_calls'][0]['function']['arguments']['reference'], 'directory')
                self.assertEqual(messages[3]['role'], 'tool')
                self.assertIn('Quartz', messages[3]['content'])
            if len(calls) == 2:
                self.assertEqual([x['role'] for x in messages[2:6]], ['assistant', 'tool', 'assistant', 'tool'])
                self.assertIn('31', messages[5]['content'])
                self.assertNotIn('lookup', [x['function']['name'] for x in tools])
            raw = '<tool_call>' + json.dumps(proposed[len(calls)]) + '</tool_call>'
            calls.append(raw)
            return raw, {'at_output_limit': False}
        row = execute(case, 'self-ask', lambda view, _: adapt_generate(view, complete),
                      repair_protocol=True, expose_budget=True)
        self.assertIsNone(row['error'])
        self.assertEqual(row['reads'], 2)
        self.assertTrue(score(dict(case, required_terms=['31'], expected_refuse=False), row))
        self.assertEqual([c['provider_raw'] for c in row['calls']], calls)
        self.assertEqual(row['calls'][0]['controller_payload']['reference'], 'directory')

    def test_native_bad_envelopes_and_unknown_functions_never_execute(self):
        case = {'id': 'c', 'question': 'q', 'seed': 'root', 'corpus': {}}
        good = '<tool_call>{"name":"lookup","arguments":{"question":"q","reference":"root"}}</tool_call>'
        for raw in (good+good, 'prose '+good,
                    '<tool_call>{"name":"shell","arguments":{}}</tool_call>',
                    '<tool_call>{"name":"lookup","name":"answer","arguments":{}}</tool_call>',
                    '<tool_call>{"name":"lookup","arguments":{"action":"answer"}}</tool_call>'):
            with self.assertRaises(ValueError):
                parse_native(raw)
            row = execute(case, 'self-ask', lambda view, _: adapt_generate(view,
                lambda *_: (raw, {'at_output_limit': False})), repair_protocol=True, expose_budget=True)
            self.assertEqual(row['reads'], 0)
            self.assertEqual(len(row['calls']), 1)
            self.assertEqual(row['repairs'], [])
            self.assertEqual(row['calls'][0]['provider_raw'], raw)
            self.assertIsNotNone(row['error'])

    def test_native_lookup_omitted_when_quota_exhausted_and_truncation_not_parsed(self):
        view = {'question': 'q', 'available_references': ['next'], 'queries_remaining': 0,
                'observations': []}
        messages, tools = native_input(view)
        self.assertNotIn('lookup', [x['function']['name'] for x in tools])
        raw = '<tool_call>{"name":"refuse","arguments":{"reason":"insufficient_evidence"}}</tool_call>'
        payload, record = adapt_generate(view, lambda *_: (raw, {'at_output_limit': True}))
        self.assertEqual(payload, raw)
        self.assertEqual(record['adapter_error'], 'truncated_proposal')
        self.assertIsNone(record['controller_payload'])

    def test_model_selection_is_registered_and_default_lock_unchanged(self):
        self.assertEqual(model_lock('qwen3-0.6b')['revision'], 'c1899de289a04d12100db370d81485cdf75e47ca')
        larger = model_lock('qwen3-1.7b')
        self.assertEqual(larger['revision'], '70d244cc86ccca08cf5af4e1e306ecf908b1ad5e')
        self.assertEqual(sum(x['path'].endswith('.safetensors') for x in larger['files']), 2)
        for name in ('../model-lock.json', '/tmp/lock.json', 'remote-model'):
            with self.assertRaisesRegex(ValueError, 'unregistered model'):
                load_model('/nonexistent', name)

    def test_v6_legal_actions_depend_on_both_quota_and_unread_ids(self):
        for quota, ids, can_ask in [(2, ['next'], True), (0, ['next'], False), (2, [], False)]:
            view = {'question': 'What is timeout?', 'available_references': ids,
                    'queries_remaining': quota, 'observations': [{'follow_up': 'Which service?',
                    'document': {'id': 'root', 'text': 'Quartz config follows.', 'references': ['next']}}]}
            result = v6_model_view(view)
            self.assertEqual('ask' in result['allowed_actions'], can_ask)
            self.assertEqual(result['allowed_actions'][:2], ['answer', 'refuse'])
            self.assertEqual(result['read_documents'], [{'id': 'root', 'text': 'Quartz config follows.'}])
            self.assertEqual(view['observations'][0]['document']['references'], ['next'])

    def test_v5_projection_keeps_only_observed_facts_and_remaining_budget(self):
        view = {'question': 'What is the timeout?', 'available_references': ['config'],
                'queries_remaining': 2, 'case_id': 'hidden-case', 'required_terms': ['31'],
                'corpus': {'hidden': 'gold'}, 'protocol_feedback': {'code': 'answer_shape'},
                'observations': [{'follow_up': 'Which service?', 'document': {
                    'id': 'directory', 'text': 'Quartz has a configuration.',
                    'references': ['config'], 'scoring_label': 'hidden-label'}}]}
        projected = v5_model_view(view)
        self.assertEqual(projected['read_documents'], [{'id': 'directory',
            'text': 'Quartz has a configuration.', 'references': ['config'], 'follow_up': 'Which service?'}])
        self.assertEqual(projected['queries_remaining'], 2)
        self.assertEqual(projected['unread_document_ids'], ['config'])
        for hidden in ('hidden-case', 'required_terms', 'hidden-label', 'corpus', '31'):
            self.assertNotIn(hidden, json.dumps(projected))
        projected['read_documents'][0]['references'].clear()
        self.assertEqual(view['observations'][0]['document']['references'], ['config'])

    def test_v5_query_budget_is_not_reset_by_protocol_repair(self):
        case = {'id': 'c', 'question': 'What is timeout?', 'seed': 'root', 'corpus': {
            'root': {'id': 'root', 'text': 'The configuration follows.', 'references': ['config']},
            'config': {'id': 'config', 'text': 'Timeout is 31 seconds.', 'references': []}}}
        replies = [
            {'action': 'ask', 'question': 'Which configuration?', 'reference': 'root'},
            {'action': 'answer', 'answer': 'missing citations'},
            {'action': 'ask', 'question': 'What is timeout?', 'reference': 'config'},
            {'action': 'answer', 'answer': '31 seconds', 'citations': [{'id': 'config', 'quote': 'Timeout is 31 seconds.'}]},
        ]
        budgets = []
        def generate(view, mode):
            budgets.append(v5_model_view(view)['queries_remaining'])
            return json.dumps(replies[len(budgets)-1]), {'at_output_limit': False}
        row = execute(case, 'self-ask', generate, repair_protocol=True, expose_budget=True)
        self.assertIsNone(row['error'])
        self.assertEqual(budgets, [3, 2, 2, 1])
        self.assertEqual(row['reads'], 2)
        budgets.clear()
        row = execute(case, 'single-query', generate, repair_protocol=True, expose_budget=True)
        self.assertEqual(budgets, [1, 0, 0])
        self.assertEqual(row['error'], 'query_budget')
        self.assertEqual(row['reads'], 1)

    def test_cli_requires_explicit_model_gate(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'result.json'
            env = dict(os.environ)
            env.pop('LOCAL_STRATEGY_E2E', None)
            result = subprocess.run([sys.executable, str(Path(__file__).with_name('self_ask_model.py')),
                                     '--assets', directory, '--output', str(output)],
                                    env=env, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 2)
            self.assertFalse(output.exists())

    def test_followup_uses_intermediate_answer_and_labels_remain_outside(self):
        case = {'id': 'hidden-case-id', 'question': 'What is service timeout?', 'seed': 'directory',
                'corpus': {'directory': {'id': 'directory', 'text': 'Service Quartz, see config.', 'references': ['config']},
                           'config': {'id': 'config', 'text': 'Timeout is 31 seconds.', 'references': []}}}
        calls = 0
        def generate(view, mode):
            nonlocal calls
            self.assertNotIn('hidden-case-id', json.dumps(view))
            self.assertNotIn('required_terms', json.dumps(view))
            if calls == 0:
                reply = {'action': 'ask', 'question': 'Which service configuration is relevant?', 'reference': 'directory'}
            elif calls == 1:
                self.assertEqual(view['available_references'], ['config'])
                self.assertIn('Quartz', view['observations'][0]['document']['text'])
                reply = {'action': 'ask', 'question': 'What is Quartz timeout?', 'reference': 'config'}
            else:
                reply = {'action': 'answer', 'answer': '31 seconds', 'citations': [{'id': 'config', 'quote': 'Timeout is 31 seconds.'}]}
            calls += 1
            return json.dumps(reply), {'at_output_limit': False}
        row = execute(case, 'self-ask', generate)
        self.assertIsNone(row['error'])
        self.assertEqual(row['reads'], 2)
        self.assertEqual(len(row['questions']), 2)
        self.assertTrue(score(dict(case, required_terms=['31'], expected_refuse=False), row))
        self.assertEqual(row['semantic_quality'], 'unreviewed')
        unrelated = dict(row, citations=[{'id': 'directory', 'quote': 'Service Quartz, see config.'}])
        self.assertFalse(score(dict(case, required_terms=['31'], expected_refuse=False), unrelated))

    def test_unseen_reference_and_unobserved_citation_are_rejected(self):
        case = {'id': 'c', 'question': 'question', 'seed': 'directory', 'corpus': {}}
        for reply, expected in [({'action': 'ask', 'question': 'q', 'reference': 'unseen'}, 'unobserved_reference'),
                                ({'action': 'answer', 'answer': 'invented', 'citations': [{'id': 'unseen', 'quote': 'invented'}]}, 'unobserved_citation')]:
            row = execute(case, 'self-ask', lambda view, mode: (json.dumps(reply), {'at_output_limit': False}))
            self.assertEqual(row['error'], expected)
            self.assertEqual(row['reads'], 0)

    def test_sole_known_binding_is_explicit_and_ambiguous_choice_rejected(self):
        case = {'id': 'c', 'question': 'question', 'seed': 'root', 'corpus': {'root': {'id': 'root', 'text': 'related', 'references': ['a', 'b']}}}
        proposal = {'action': 'ask', 'question': 'Which source supplies the answer?'}
        row = execute(case, 'self-ask', lambda view, mode: (json.dumps(proposal), {'at_output_limit': False}), sole_reference=True)
        self.assertEqual(row['reads'], 1)
        self.assertEqual(row['questions'][0]['binding'], 'sole_available_reference')
        self.assertEqual(row['error'], 'unobserved_reference')

    def test_protocol_feedback_preserves_observations_and_shared_budget(self):
        case = {'id':'hidden-gold', 'question':'What is service timeout?', 'seed':'root',
                'corpus':{'root':{'id':'root','text':'Quartz config is next.','references':['next']},
                          'next':{'id':'next','text':'Quartz timeout is 31 seconds.','references':[]}}}
        replies = [
            {'action':'ask','question':'Which configuration?','reference':'root'},
            {'action':'answer','answer':'unsupported early answer'},
            {'action':'ask','question':'What is Quartz timeout?','reference':'next'},
            {'action':'answer','answer':'31 seconds','citations':[{'id':'next','quote':'Quartz timeout is 31 seconds.'}]},
        ]
        count = 0
        def generate(view, mode):
            nonlocal count
            self.assertNotIn('hidden-gold', json.dumps(view))
            if count == 2:
                self.assertEqual(view['protocol_feedback'], {'code':'answer_shape'})
                self.assertEqual(view['available_references'], ['next'])
                self.assertEqual(view['observations'][0]['document']['id'], 'root')
            if count == 3:
                self.assertNotIn('protocol_feedback', view)
            reply = replies[count]
            count += 1
            return json.dumps(reply), {'at_output_limit':False}
        row = execute(case, 'self-ask', generate, repair_protocol=True)
        self.assertIsNone(row['error'])
        self.assertEqual(row['reads'], 2)
        self.assertEqual(len(row['calls']), 4)
        self.assertEqual(row['repairs'], [{'code':'answer_shape'}])
        self.assertTrue(score(dict(case, required_terms=['31'], expected_refuse=False), row))

        count = 0
        row = execute(case, 'single-query', generate, repair_protocol=True)
        self.assertEqual(row['error'], 'query_budget')
        self.assertEqual(row['reads'], 1)
        self.assertEqual(len(row['calls']), 3)

    def test_repair_does_not_guess_sources_or_repeat_forever(self):
        case = {'id':'c','question':'q','seed':'root','corpus':{}}
        malformed = lambda *_: ('{"action":"answer","answer":"x"}', {'at_output_limit':False})
        row = execute(case,'self-ask',malformed,repair_protocol=True)
        self.assertEqual(row['error'],'answer_shape')
        self.assertEqual(len(row['calls']),2)
        self.assertEqual(len(row['repairs']),1)
        for raw, limited, error in [
            ('{"action":"ask","question":"q","reference":"unseen"}', False, 'unobserved_reference'),
            ('{"action":"answer","answer":"x","citations":[{"id":"unseen","quote":"x"}]}', False, 'unobserved_citation'),
            ('{}', True, 'truncated_proposal'),
        ]:
            row = execute(case,'self-ask',lambda *_:(raw,{'at_output_limit':limited}),repair_protocol=True)
            self.assertEqual(row['error'],error)
            self.assertEqual(len(row['calls']),1)
            self.assertEqual(row['repairs'],[])
            self.assertEqual(row['reads'],0)

    def test_recorded_questions_and_failures_replay_without_inference(self):
        base = Path(__file__).parent
        raw_cases = (base / 'self-ask-cases.json').read_bytes()
        dataset = json.loads(raw_cases)
        for version in ('v1', 'v2', 'v3', 'v4', 'v5', 'v6'):
            record = json.loads((base / 'testdata' / ('followup-development-' + version + '-result.json')).read_text())
            self.assertTrue(record['complete'])
            self.assertEqual(record['case_sha256'], hashlib.sha256(raw_cases).hexdigest())
            self.assertEqual(len(record['rows']), 8)
            for case in dataset['cases']:
                task = {k: case[k] for k in ('id', 'question', 'seed', 'corpus')}
                for mode in ('single-query', 'self-ask'):
                    saved = next(x for x in record['rows'] if x['case_id'] == case['id'] and x['mode'] == mode)
                    calls = iter(saved['calls'])
                    def replay(view, mode):
                        call = next(calls)
                        self.assertEqual(view, call['view'])
                        self.assertEqual(call['raw'], call['application_prefix'] + call['generated_text'])
                        if version == 'v5':
                            self.assertEqual(v5_model_view(view), call['model_view'])
                        if version == 'v6':
                            self.assertEqual(v6_model_view(view), call['model_view'])
                        return call['raw'], call
                    row = execute(task, mode, replay, sole_reference=version in ('v3', 'v4', 'v5', 'v6'),
                                  repair_protocol=version in ('v4', 'v5', 'v6'), expose_budget=version in ('v5', 'v6'))
                    for key in ('answer', 'refused', 'error', 'reads', 'questions', 'observations'):
                        self.assertEqual(row[key], saved[key])
                    self.assertEqual(row['repairs'], saved.get('repairs', []))
                    self.assertEqual(score(case, row), saved['coverage_pass'])
                    with self.assertRaises(StopIteration):
                        next(calls)

    def test_paired_model_records_keep_same_tasks_and_control(self):
        base = Path(__file__).parent
        raw_cases = (base / 'self-ask-cases.json').read_bytes()
        cases = json.loads(raw_cases)['cases']
        for name in ('qwen3-0.6b', 'qwen3-1.7b'):
            size = name.removeprefix('qwen3-')
            record = json.loads((base / 'testdata' / ('followup-capacity-' + size + '-v6-result.json')).read_text())
            self.assertTrue(record['complete'])
            self.assertEqual(record['model_name'], name)
            self.assertEqual(record['model_revision'], model_lock(name)['revision'])
            self.assertEqual(record['prompt_version'], 'self-ask-v6')
            self.assertEqual(record['case_sha256'], hashlib.sha256(raw_cases).hexdigest())
            self.assertEqual(record['runtime'], {'device': 'cpu', 'threads': 1, 'dtype': 'torch.float32'})
            self.assertEqual(len(record['rows']), 8)
            for saved in record['rows']:
                case = next(x for x in cases if x['id'] == saved['case_id'])
                task = {k: case[k] for k in ('id', 'question', 'seed', 'corpus')}
                calls = iter(saved['calls'])
                def replay(view, mode):
                    call = next(calls)
                    self.assertEqual(view, call['view'])
                    self.assertEqual(v6_model_view(view), call['model_view'])
                    self.assertEqual(call['raw'], call['application_prefix'] + call['generated_text'])
                    return call['raw'], call
                row = execute(task, saved['mode'], replay, sole_reference=True,
                              repair_protocol=True, expose_budget=True)
                for key in ('answer', 'refused', 'error', 'reads', 'questions', 'observations', 'repairs'):
                    self.assertEqual(row[key], saved[key])
                self.assertEqual(score(case, row), saved['coverage_pass'])
                with self.assertRaises(StopIteration):
                    next(calls)


if __name__ == '__main__':
    unittest.main()
