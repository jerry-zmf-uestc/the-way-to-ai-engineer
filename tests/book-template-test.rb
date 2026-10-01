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
