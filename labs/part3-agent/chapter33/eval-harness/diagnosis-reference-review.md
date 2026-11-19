# 运行诊断证据必要性独立审阅

先只看问题与事件，不看模型答案、Expected或机器分数。判断哪套事件足以支持原因与下一步，是否有其他充分路径；空白不表示无需证据，Reviewed仍为false。

必要性针对具体判断：移除一条事件后，结论是否仍成立？两套事件各自充分时，可以分别记录，不能要求回答把所有替代路径都引用一遍。支持原因与支持下一步可能需要不同事件，分别说明。

各自完成后比较分歧与依据，再审阅模型答案。新标注需要新版本并重新评分，旧运行和旧判断口径保持；不覆盖原标签或把开发问题改称未见验收。

## d01

为什么任务b没有开始？给出下一步，不承诺已入队。

| 事件 | 内容 |
| --- | --- |
| e1 | ready_queue_limit=1; task a is queued. |
| e2 | task b admission returned ErrQueue; its handler calls=0. |

建议选项：wait_or_reduce_batch / raise_model_steps / report_success

- 最小充分事件集合（可记录多套）：待填写。
- 原因 / 下一步各自需要的事件：待填写。
- 非必要或仅补充事件：待填写。
- 必要性、替代关系和歧义说明：待填写。
- 评审者 / 参照版本：待填写。
- 任务视图摘要：1d00036b243db9f0be92d44de979363c9d4168605afbc61ec217399fd05e334d
- 是否完成独立审阅：否。

## d02

为什么这次角色执行被拒绝？怎样安全地继续判断？

| 事件 | 内容 |
| --- | --- |
| e1 | Current task owner=retry-reader, revision=4. |
| e2 | Request from=timeout-reader, revision=3; response=ErrHandoff; tool calls=0. |

建议选项：refresh_state / retry_same_request / create_new_task

- 最小充分事件集合（可记录多套）：待填写。
- 原因 / 下一步各自需要的事件：待填写。
- 非必要或仅补充事件：待填写。
- 必要性、替代关系和歧义说明：待填写。
- 评审者 / 参照版本：待填写。
- 任务视图摘要：c7719f64442715e2fab244926bbe3353b4283d7315ec3db4a072c2866a495138
- 是否完成独立审阅：否。

## d03

工具已返回，但下一轮仍说没有资料。优先检查哪里？

| 事件 | 内容 |
| --- | --- |
| e1 | Assistant proposed tool call id=c1. |
| e2 | Executor returned a valid result for c1. |
| e3 | Next model history contains user and assistant messages, but no tool observation for c1. |

建议选项：inspect_observation_binding / increase_retrieval_count / declare_tool_failed

- 最小充分事件集合（可记录多套）：待填写。
- 原因 / 下一步各自需要的事件：待填写。
- 非必要或仅补充事件：待填写。
- 必要性、替代关系和歧义说明：待填写。
- 评审者 / 参照版本：待填写。
- 任务视图摘要：8ea2fc80df974bbadc5a640f3c5a8ca37d243fc3d0bc9c2b7d2e31dcd1a29836
- 是否完成独立审阅：否。

## d04

子任务completed，为什么不能放进当前报告？

| 事件 | 内容 |
| --- | --- |
| e1 | Child binding parent_goal_revision=1; task revision=1; result=completed. |
| e2 | Current parent goal revision=2; scope and parent ID unchanged. |

建议选项：reject_old_goal_result / accept_any_completed / reset_parent_revision

- 最小充分事件集合（可记录多套）：待填写。
- 原因 / 下一步各自需要的事件：待填写。
- 非必要或仅补充事件：待填写。
- 必要性、替代关系和歧义说明：待填写。
- 评审者 / 参照版本：待填写。
- 任务视图摘要：8864c81db722b5b71d10c52780576bb738cbb7fb027dbf41237fa7066134c791
- 是否完成独立审阅：否。

## d05

是否可以直接重新读取工具并宣称原角色已经完成？

| 事件 | 内容 |
| --- | --- |
| e1 | Store phase=executing; stable role turns=0; RoleResults is empty. |
| e2 | Attempt 1 outcome=returned; bounded raw Fact saved; Used=1. |
| e3 | Original process has terminated. |

建议选项：inspect_commit_and_authority / reread_with_fresh_budget / report_role_completed

- 最小充分事件集合（可记录多套）：待填写。
- 原因 / 下一步各自需要的事件：待填写。
- 非必要或仅补充事件：待填写。
- 必要性、替代关系和歧义说明：待填写。
- 评审者 / 参照版本：待填写。
- 任务视图摘要：d00d17a7856026a06b701b7971a605fe45ef550af08a45fd6ae1b544c264c539
- 是否完成独立审阅：否。

## d06

索引命中了资料，当前回答是否可以引用它？

| 事件 | 内容 |
| --- | --- |
| e1 | Retriever returned document d with index_version=1. |
| e2 | Current authority marks document d revoked; final delivery check denied access. |

建议选项：withhold_revoked_evidence / cite_cached_document / rewrite_permission_flag

- 最小充分事件集合（可记录多套）：待填写。
- 原因 / 下一步各自需要的事件：待填写。
- 非必要或仅补充事件：待填写。
- 必要性、替代关系和歧义说明：待填写。
- 评审者 / 参照版本：待填写。
- 任务视图摘要：6afe234c1af4880fdae6b8974852ef5df5259d0da5e7d9f71456ac753a2f9c44
- 是否完成独立审阅：否。

## d07

能否把两位角色的结果投票为一个超时值？

| 事件 | 内容 |
| --- | --- |
| e1 | Role a reports production timeout=30 seconds, source v1. |
| e2 | Role b reports production timeout=60 seconds, source v2. |
| e3 | Both sources are in allowed scope; which version is current is not established. |

建议选项：resolve_version_or_clarify / majority_vote / choose_last_arrival

- 最小充分事件集合（可记录多套）：待填写。
- 原因 / 下一步各自需要的事件：待填写。
- 非必要或仅补充事件：待填写。
- 必要性、替代关系和歧义说明：待填写。
- 评审者 / 参照版本：待填写。
- 任务视图摘要：139f9d14cd5f8b2baa18c3ca1e114c397a5ab8f62275701328c62d6dedcb5a45
- 是否完成独立审阅：否。

## d08

用户已经取消，为什么名额仍未释放？现在能接纳新任务吗？

| 事件 | 内容 |
| --- | --- |
| e1 | Manager active_limit=1; task a state=canceling. |
| e2 | Worker a received cancellation but has not exited. |
| e3 | New task b returned ErrCapacity. |

建议选项：wait_for_exit_and_fix_downstream / release_slot_immediately / increase_token_budget

- 最小充分事件集合（可记录多套）：待填写。
- 原因 / 下一步各自需要的事件：待填写。
- 非必要或仅补充事件：待填写。
- 必要性、替代关系和歧义说明：待填写。
- 评审者 / 参照版本：待填写。
- 任务视图摘要：45db0b1be923b4dde9fb6d98d23d353bc05a41ee2d8efb3173377987b714e04f
- 是否完成独立审阅：否。

