#!/usr/bin/env ruby
require 'fileutils'
require 'tmpdir'
require 'open3'
require 'rbconfig'

root = File.expand_path('..', __dir__)
checker = File.join(root, 'scripts/verify-docs-layout.rb')
cases = {
  'valid layout' => [nil, ->(dir) {}],
  'obsolete guide directory' => ['obsolete docs path', ->(dir) { FileUtils.mkdir_p(File.join(dir, 'docs/guides')) }],
  'obsolete testing rule' => ['obsolete docs path', ->(dir) { File.write(File.join(dir, 'docs/rules/testing.md'), "# Testing Rules\n") }],
  'extra design document' => ['designs must contain only project-plan.md', ->(dir) { File.write(File.join(dir, 'docs/designs/extra.md'), "# Duplicate design\n") }],
  'missing testing anchor' => ['missing testing-requirements anchor', ->(dir) {
    path = File.join(dir, 'AGENTS.md')
    File.write(path, File.read(path).sub('<a id="testing-requirements"></a>', ''))
  }],
  'unreferenced new rule' => ['unreferenced rule or template', ->(dir) { File.write(File.join(dir, 'docs/rules/extra.md'), "# New rule\n") }],
  'missing template' => ['missing required document', ->(dir) { FileUtils.rm(File.join(dir, 'docs/templates/book-chapter.md')) }],
  'missing preparation anchor' => ['missing running-examples anchor', ->(dir) {
    path = File.join(dir, 'book/introduction.md')
    File.write(path, File.read(path).sub('<a id="running-examples"></a>', ''))
  }],
  'missing variables anchor' => ['missing variables anchor', ->(dir) {
    path = File.join(dir, 'docs/rules/security.md')
    File.write(path, File.read(path).sub('<a id="variables"></a>', ''))
  }]
}
cases.each do |name, (failure, mutate)|
  Dir.mktmpdir('docs-layout-test-') do |dir|
    %w[AGENTS.md docs book].each { |path| FileUtils.cp_r(File.join(root, path), dir) }
    mutate.call(dir)
    out, err, status = Open3.capture3(RbConfig.ruby, checker, dir)
    valid = failure ? (!status.success? && err.include?(failure)) : status.success?
    abort "#{name}: unexpected result: #{out}#{err}" unless valid
    puts "PASS: #{name}"
  end
end
