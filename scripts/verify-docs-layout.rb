#!/usr/bin/env ruby
require 'pathname'

root = File.expand_path(ARGV.fetch(0, '..'), ARGV.empty? ? __dir__ : Dir.pwd)
Dir.chdir(root)
obsolete = %w[docs/guides docs/rules/workspace.md docs/rules/spec-driven-workflow.md docs/rules/variables.md docs/rules/testing.md]
obsolete.each { |p| abort "obsolete docs path: #{p}" if File.exist?(p) }
required = %w[docs/designs/project-plan.md docs/release-readiness.json docs/rules/book-chapter-contract.md docs/rules/topic-contract.md docs/rules/security.md docs/templates/book-chapter.md docs/templates/book-chapter-review.md]
required.each { |p| abort "missing required document: #{p}" unless File.file?(p) }
design_entries = Dir.children('docs/designs').sort
abort "designs must contain only project-plan.md: #{design_entries.join(', ')}" unless design_entries == ['project-plan.md']
agents = File.read('AGENTS.md').gsub(/```.*?```/m, '')
abort 'AGENTS.md: missing testing-requirements anchor' unless agents.scan('<a id="testing-requirements"></a>').length == 1
linked = agents.scan(/\[[^\]]+\]\(([^)]+)\)/).flatten.reject { |l| l.match?(%r{^https?://}) }.map { |l| File.expand_path(l.split('#', 2).first) }
rules = Dir.glob('docs/rules/**/*.md')
templates = Dir.glob('docs/templates/**/*.md')
(rules + templates).each do |p|
  abort "AGENTS.md: unreferenced rule or template: #{p}" unless linked.include?(File.expand_path(p))
end
intro = File.read('book/introduction.md')
abort 'introduction: missing running-examples anchor' unless intro.scan('<a id="running-examples"></a>').length == 1
abort 'security: missing variables anchor' unless File.read('docs/rules/security.md').include?('<a id="variables"></a>')
puts "docs layout: one consolidated design, #{rules.length} rules and #{templates.length} templates; AGENTS references valid"
