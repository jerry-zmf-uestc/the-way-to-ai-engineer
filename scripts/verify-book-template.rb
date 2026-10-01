#!/usr/bin/env ruby
require 'pathname'

# Structural checks only. A passing result does not certify teaching quality.
# All pages use the new skeleton: opening sentence, topic sections, pitfall
# list, table-form exercises and links.
root = File.expand_path(ARGV.fetch(0, '..'), ARGV.empty? ? __dir__ : Dir.pwd)
Dir.chdir(root)
pages = Dir.glob('book/**/chapter*.md').sort + ['book/part3-agent/chapter28/rag.md']
abort 'expected 21 teaching pages' unless pages.uniq.length == 21
total = 0
pages.each do |path|
  text = File.read(path)
  prose = text.gsub(/^```[^\n]*\n.*?^```\s*$/m, '')
  abort "#{path}: expected one title" unless prose.scan(/^# /).length == 1
  %w[踩坑清单 动手练 链接].each do |title|
    abort "#{path}: missing #{title}" unless prose.match?(/^## #{title}$/)
  end
  opening_block = prose.split(/^## /, 2).first.to_s
  opening = opening_block.lines.drop(1).join
  abort "#{path}: empty opening" if opening.strip.empty?
  abort "#{path}: core summary block no longer allowed" if opening_block.include?('本章核心')
  %w[本章目标 本章小结 完成检查 示例中的分工].each do |banned|
    abort "#{path}: boilerplate section #{banned}" if prose.match?(/^\#{2,3} #{banned}$/)
  end
  abort "#{path}: boilerplate phrase" if prose.include?('先选正常和边界输入预测结果')
  refs = prose.split(/^## 链接\s*$/, 2).last.to_s.split(/^## /, 2).first
  abort "#{path}: missing official reference" unless refs.match?(%r{https://(?:go\.dev|pkg\.go\.dev|www\.cloudwego\.io)/})
  hands_on = prose.split(/^## 动手练\s*$/, 2).last.to_s.split(/^## /, 2).first
  abort "#{path}: missing exercise table" unless hands_on.match?(/^\| --- /)
  # Exercises live in the hands-on table; no page keeps TODO exercise blocks.
  go_blocks = text.scan(/^```(?:go|thrift)\n(.*?)^```\s*$/m).flatten.select { |block| block.include?('// 任务：') }
  shell_blocks = text.scan(/^```(?:bash|sh|console)\n(.*?)^```\s*$/m).flatten.select { |block| block.include?('# 任务：') }
  blocks = go_blocks + shell_blocks
  abort "#{path}: exercise code blocks no longer allowed, use the table" unless blocks.empty?
  total += blocks.length
  abort "#{path}: maintenance text in reader page" if prose.match?(/source revision|迁移审查|meta\.yml|正文尚待|等待确认/)
  footer = text.strip.lines.last
  abort "#{path}: missing footer navigation" unless footer.start_with?('[返回全书目录]')
  links = footer.scan(/\[[^\]]+\]\(([^)]+)\)/).flatten
  links.each do |link|
    target = File.expand_path(link.split('#', 2).first, File.dirname(path))
    abort "#{path}: broken footer link #{link}" unless File.file?(target)
  end
end
abort "expected 0 exercises, got #{total}" unless total == 0
puts "book structure: #{pages.length} teaching pages on the new skeleton; exercises as tables; pitfalls, references and navigation present (manual review still required)"
