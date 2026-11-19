# coding: utf-8
"""Export unscored review views; never create completed annotations."""
import argparse
import hashlib
import json
import re
from pathlib import Path


def digest(text):
    return hashlib.sha256(text.encode('utf-8')).hexdigest()


def prepare_packet(dataset, record):
    if dataset['version'] != record['DatasetVersion'] or record['PromptVersion'] != 'runtime-diagnosis-v1' or record['Profile'] != 'qwen3-1.7b' or record['Revision'] != '70d244cc86ccca08cf5af4e1e306ecf908b1ad5e':
        raise ValueError('record identity')
    cases = dataset['cases']
    runs = record['Runs']
    if len(cases) != 8 or len(runs) != len(cases) or len({c['id'] for c in cases}) != len(cases):
        raise ValueError('case set')
    rows = []
    seen = set()
    by_id = {c['id']: c for c in cases}
    for run in runs:
        identity = run['CaseID']
        if identity in seen or identity not in by_id:
            raise ValueError('run identity')
        seen.add(identity)
        case = by_id[identity]
        view = {key: case[key] for key in ('question', 'events', 'allowed_actions')}
        task = run['Input']
        row = run['Generation']
        messages = row['input_messages']
        if task['ID'] != identity or json.loads(task['Input']) != view or task['AllowedSources'] != [e['id'] for e in view['events']] or task['AllowedTools'] not in (None, []):
            raise ValueError('task projection')
        if len(messages) != 2 or messages[0]['Role'] != 'system' or messages[1]['Role'] != 'user' or json.loads(messages[1]['Content']) != view:
            raise ValueError('actual input')
        raw = row['raw']
        if not isinstance(raw, str) or len(raw.encode('utf-8')) > 4096 or not 0 < row['input_tokens'] <= 1024 or not 0 < row['output_tokens'] <= 192 or row['at_output_limit'] != (row['output_tokens'] == 192):
            raise ValueError('record budget')
        judgment = {
            'DatasetVersion': dataset['version'], 'RubricVersion': 'diagnosis-rubric-v1',
            'CaseID': identity, 'InputSHA256': digest(task['Input']),
            'AnswerSHA256': digest(raw), 'Verdict': 'unknown',
        }
        rows.append({
            'CaseID': identity, 'Task': view, 'AnswerRaw': raw,
            'Truncated': row['at_output_limit'], 'JudgmentTemplate': judgment,
            'Reviewer': '', 'ReferenceVersion': '', 'Rationale': '',
            'UnresolvedQuestions': [],
        })
    return {'Schema': 1, 'Purpose': 'independent-development-review-not-completed-labels',
            'DatasetVersion': dataset['version'], 'Cases': rows}


def render_markdown(packet):
    lines = [
        '# 运行诊断原因独立审阅',
        '',
        '本材料保留合成问题、输入事件和模型原文，不展示预期标签或机器分数。它是开发审阅起点，空白记录不是已经完成的标注。',
        '',
        '先各自阅读事件与原答案，再判断原因解释是否成立；完成独立判断后才比较意见。supported表示原因与关键关系有事件支持；unsupported表示原因与事件矛盾或明确误认；依据不够、有歧义或不能确定时保留unknown。引用齐备或建议合法不能代替原因判断。',
        '',
        '填写评审者、参照版本、支持/反驳的事件和理由；保留分歧与尚未解决的问题。建议和引用另行复核，不从这份原因判断推导全部任务通过。哈希只绑定原文，不认证评审者。',
        '',
    ]
    for row in packet['Cases']:
        task = row['Task']
        lines.extend(['## ' + row['CaseID'], '', task['question'], '', '| 事件 | 内容 |', '| --- | --- |'])
        for event in task['events']:
            text = event['text'].replace('|', '\\|').replace('\n', ' ')
            lines.append('| ' + event['id'] + ' | ' + text + ' |')
        lines.extend(['', '建议选项：' + ' / '.join(task['allowed_actions']), '', '模型原文：', ''])
        fence = chr(96) * max(3, max((len(s) for s in re.findall(chr(96) + '+', row['AnswerRaw'])), default=0) + 1)
        lines.extend([fence + 'json', row['AnswerRaw'], fence, '',
                      '- 原因判断：unknown（待独立审阅）。',
                      '- 评审者 / 参照版本：待填写。',
                      '- 支持或反驳事件及理由：待填写。',
                      '- 分歧与未解决问题：待填写。',
                      '- 输入摘要：' + row['JudgmentTemplate']['InputSHA256'],
                      '- 答案摘要：' + row['JudgmentTemplate']['AnswerSHA256'], ''])
    return '\n'.join(lines)


def prepare_reference_packet(dataset):
    cases = dataset['cases']
    if dataset['version'] != 'runtime-diagnosis-development-v1' or len(cases) != 8 or len({c['id'] for c in cases}) != len(cases):
        raise ValueError('frozen reference scope')
    rows = []
    for case in cases:
        view = {key: case[key] for key in ('question', 'events', 'allowed_actions')}
        rows.append({
            'CaseID': case['id'],
            'Task': view,
            'TaskSHA256': digest(json.dumps(view, sort_keys=True, ensure_ascii=False)),
            'Reviewed': False, 'SufficientEventSets': [], 'OptionalEvents': [],
            'Reviewer': '', 'ReferenceVersion': '', 'Rationale': '',
            'UnresolvedQuestions': [],
        })
    return {'Schema': 1, 'Purpose': 'reference-review-before-model-output',
            'DatasetVersion': dataset['version'], 'Cases': rows}


def render_reference_markdown(packet):
    lines = ['# 运行诊断证据必要性独立审阅', '',
             '先只看问题与事件，不看模型答案、Expected或机器分数。判断哪套事件足以支持原因与下一步，是否有其他充分路径；空白不表示无需证据，Reviewed仍为false。', '',
             '必要性针对具体判断：移除一条事件后，结论是否仍成立？两套事件各自充分时，可以分别记录，不能要求回答把所有替代路径都引用一遍。支持原因与支持下一步可能需要不同事件，分别说明。', '',
             '各自完成后比较分歧与依据，再审阅模型答案。新标注需要新版本并重新评分，旧运行和旧判断口径保持；不覆盖原标签或把开发问题改称未见验收。', '']
    for row in packet['Cases']:
        task = row['Task']
        lines.extend(['## ' + row['CaseID'], '', task['question'], '', '| 事件 | 内容 |', '| --- | --- |'])
        for event in task['events']:
            lines.append('| ' + event['id'] + ' | ' + event['text'].replace('|', '\\|').replace('\n', ' ') + ' |')
        lines.extend(['', '建议选项：' + ' / '.join(task['allowed_actions']), '',
                      '- 最小充分事件集合（可记录多套）：待填写。',
                      '- 原因 / 下一步各自需要的事件：待填写。',
                      '- 非必要或仅补充事件：待填写。',
                      '- 必要性、替代关系和歧义说明：待填写。',
                      '- 评审者 / 参照版本：待填写。',
                      '- 任务视图摘要：' + row['TaskSHA256'],
                      '- 是否完成独立审阅：否。', ''])
    return '\n'.join(lines)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--format', choices=['md', 'json'], default='md')
    parser.add_argument('--scope', choices=['answer', 'reference'], default='answer')
    args = parser.parse_args()
    root = Path(__file__).parent
    dataset = json.loads((root / 'diagnosis-cases.json').read_text(encoding='utf-8'))
    if args.scope == 'reference':
        packet = prepare_reference_packet(dataset)
        rendered = render_reference_markdown(packet)
    else:
        record = json.loads((root / 'testdata/diagnosis-generation-v1.json').read_text(encoding='utf-8'))
        packet = prepare_packet(dataset, record)
        rendered = render_markdown(packet)
    print(rendered if args.format == 'md' else json.dumps(packet, ensure_ascii=False, indent=2))
