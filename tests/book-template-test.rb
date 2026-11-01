#!/usr/bin/env ruby
require 'fileutils'
require 'tmpdir'
require 'open3'
require 'rbconfig'

root = File.expand_path('..', __dir__)
checker = File.join(root, 'scripts/verify-book-template.rb')
ch02 = 'book/part1-golang/chapter02.md' # 多主题章
ch04 = 'book/part1-golang/chapter04.md' # 单主题新骨架样例
cases = {
  'valid book' => [nil, nil, nil],
  'sample missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/building/chapter24-decision-loop.md'],
  'sample missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/building/chapter24-decision-loop.md'],
  'sample missing references' => ['missing 参考文档', ['## 参考文档', '## 其他'], 'book/part3-agent/building/chapter24-decision-loop.md'],
  'sample rejects legacy section' => ['legacy fixed section 动手练', ['## 小结', "## 动手练\n\n## 小结"], 'book/part3-agent/building/chapter24-decision-loop.md'],
  'Tool missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/building/chapter23-actions.md'],
  'Runtime missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/runtime/chapter29-execution.md'],
  'Model rejects legacy section' => ['legacy fixed section 动手练', ['## 小结', "## 动手练\n\n## 小结"], 'book/part3-agent/building/chapter22-architecture.md'],
  'Session missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/building/chapter26-context.md'],
  'Context missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/building/chapter26-context.md'],
  'RAG missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/building/chapter27-rag.md'],
  'Eval missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/governance/chapter35-evaluation.md'],
  'ReAct missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/building/chapter24-decision-loop.md'],
  'Planning missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/building/chapter25-planning.md'],
  'Checkpoint missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/runtime/chapter29-execution.md'],
  'Trace missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/governance/chapter34-observability.md'],
  'MCP missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/building/chapter23-actions.md'],
  'Harness missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/building/chapter22-architecture.md'],
  'Skills missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/building/chapter28-skills.md'],
  'Workspace missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/runtime/chapter30-state.md'],
  'Memory missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/runtime/chapter30-state.md'],
  'Memory missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/runtime/chapter30-state.md'],
  'Delegation missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/runtime/chapter31-collaboration.md'],
  'Async missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/runtime/chapter32-async-tasks.md'],
  'Async missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/runtime/chapter32-async-tasks.md'],
  'Streaming missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/runtime/chapter33-serving.md'],
  'Streaming missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/runtime/chapter33-serving.md'],
  'Serving missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/runtime/chapter33-serving.md'],
  'Serving missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/runtime/chapter33-serving.md'],
  'Safety missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/governance/chapter36-security.md'],
  'Safety missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/governance/chapter36-security.md'],
  'Cost missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/governance/chapter37-cost.md'],
  'Cost missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/governance/chapter37-cost.md'],
  'HITL missing lab entry' => ['missing top lab entry', ['配套实验：', '实验：'], 'book/part3-agent/building/chapter23-actions.md'],
  'HITL missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/building/chapter23-actions.md'],
  'DAG missing summary' => ['missing 小结', ['## 小结', '## 其他'], 'book/part3-agent/runtime/chapter31-collaboration.md'],
  'missing pitfall list' => ['missing 踩坑清单', ['## 踩坑清单', '## 其他'], ch04],
  'missing links section' => ['missing 链接', ['## 链接', '## 其他'], ch04],
  'boilerplate section in new skeleton' => ['boilerplate section 本章小结', ['## 动手练', "## 本章小结\n\n兜底内容\n\n## 动手练"], ch04],
  'empty opening' => ['empty opening', [%r{Go 的函数设计极度克制.*?(?=## 4\.1)}m, '', :gsub], ch04],
  'core summary block banned' => ['core summary block no longer allowed', ['## 4.1 函数声明', "本章核心：\n\n> **测试用摘要。**\n\n## 4.1 函数声明"], ch04],
  'missing exercise table' => ['missing exercise table', ['| --- | --- | --- | --- |', '|---|---|---|---|'], ch04],
  'missing official source' => ['missing official reference', [%r{https://(?:go\.dev|pkg\.go\.dev)/}, 'https://example.invalid/', :gsub], ch02],
  'exercise code block rejected' => ['exercise code blocks no longer allowed', ['## 链接', "```go\n// 任务：x\n// 预期结果：y\n// 文件：z\n// TODO: t\n```\n\n## 链接"], ch04],
  'broken navigation' => ['broken footer link', ['[下一篇：控制流与边界分支](chapter03.md)', '[下一篇：控制流与边界分支](missing.md)'], ch02]
}
cases.each do |name, (failure, replacement, target)|
  Dir.mktmpdir('book-template-test-') do |dir|
    FileUtils.cp_r(File.join(root, 'book'), dir)
    if replacement
      path = File.join(dir, target)
      original = File.read(path)
      mutated = replacement.last == :gsub ? original.gsub(replacement[0], replacement[1]) : original.sub(*replacement)
      abort "#{name}: mutation did not apply" if original == mutated
      File.write(path, mutated)
    end
    out, err, status = Open3.capture3(RbConfig.ruby, checker, dir)
    if failure
      abort "#{name}: wrong result: #{out}#{err}" unless !status.success? && err.include?(failure)
    else
      abort "#{name}: rejected valid book: #{out}#{err}" unless status.success?
    end
    puts "PASS: #{name}"
  end
end

Dir.mktmpdir('book-template-missing-new-page-') do |dir|
  FileUtils.cp_r(File.join(root, 'book'), dir)
  FileUtils.rm(File.join(dir, 'book/part3-agent/runtime/chapter33-serving.md'))
  out, err, status = Open3.capture3(RbConfig.ruby, checker, dir)
  abort "missing new page accepted: #{out}#{err}" unless !status.success? && err.include?('expected 32 teaching pages')
  puts 'PASS: missing new teaching page rejected'
end
