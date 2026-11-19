#!/usr/bin/env ruby
require 'tmpdir'
require 'fileutils'
require 'open3'
root = File.expand_path('..', __dir__)
checker = File.join(root, 'scripts/validate-chapter-map.sh')
Dir.mktmpdir('chapter-evidence-test-') do |dir|
  %w[content labs docs Makefile tests].each { |path| FileUtils.cp_r(File.join(root, path), dir) }
  path = File.join(dir, 'docs/designs/project-plan.md')
  original = File.read(path)
  out, status = Open3.capture2e('/bin/sh', checker, chdir: dir)
  abort "valid map with separate unit table: #{out}" unless status.success?
  puts 'PASS: unit-number table does not replace chapter-state map'
  [
    ['missing chapter-map anchor', original.sub('<a id="chapter-map"></a>', ''), 'chapter map anchor must occur once'],
    ['missing legacy chapter row', original.sub(/^\| 33 \| Agent治理 · Eval、Trace[^\n]*\n/, ''), 'chapter numbers must be unique and continuous'],
    ['missing evidence type', original.sub('code: [checks]', '[checks]'), 'missing code evidence'],
    ['invalid review anchor', original.sub('#author-review)', '#not-a-review)'), 'missing evidence anchor'],
    ['script posing as reader evidence', original.sub('reviewed | pending |', 'reviewed | validated |').sub('code: [checks]', 'reader: [tests](../../Makefile); code: [checks]'), 'reader evidence cannot be a script']
  ].each do |name, text, error|
    File.write(path, text)
    out, status = Open3.capture2e('/bin/sh', checker, chdir: dir)
    abort "#{name}: #{out}" unless !status.success? && out.include?(error)
    puts "PASS: #{name} rejected"
  end
end
