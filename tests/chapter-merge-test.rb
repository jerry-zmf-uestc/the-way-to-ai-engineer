#!/usr/bin/env ruby
require 'fileutils'
require 'tmpdir'
require 'open3'
require 'rbconfig'
require 'json'

root = File.expand_path('..', __dir__)
checker = File.join(root, 'scripts/verify-chapter-merge.rb')
cases = {
  'valid layout' => [nil, ->(dir) {}],
  'duplicate directory' => ['duplicate chapter directory', ->(dir) {
    FileUtils.mkdir_p(File.join(dir, 'content/book/part1-golang/chapter02'))
  }],
  'missing topic anchor' => ['topic anchors missing', ->(dir) {
    path = File.join(dir, 'content/book/part1-golang/chapter07.md')
    File.write(path, File.read(path).sub('<a id="reflection"></a>', ''))
  }],
  'duplicate exercise number' => ['exercises must be sequential', ->(dir) {
    path = File.join(dir, 'content/book/part1-golang/chapter02.md')
    File.write(path, File.read(path).sub('2.4 比较普通和严格', '2.3 比较普通和严格'))
  }],
  'missing linked anchor' => ['missing merged chapter anchor', ->(dir) {
    File.write(File.join(dir, 'README.md'), "[topic](content/book/part1-golang/chapter02.md#missing-topic)\n")
  }],
  'duplicate topic region' => ['chapter regions are incomplete', ->(dir) {
    path = File.join(dir, 'tests/fixtures/book-regions.json')
    fixture = JSON.parse(File.read(path))
    regions = fixture.fetch('regions')
    regions << regions.find { |location| location.include?('chapter07.md#') }
    File.write(path, JSON.generate(fixture))
  }]
}
cases.each do |name, (failure, mutate)|
  Dir.mktmpdir('chapter-merge-test-') do |dir|
    FileUtils.mkdir_p(File.join(dir, 'content/book/part1-golang'))
    FileUtils.mkdir_p(File.join(dir, 'tests/fixtures'))
    %w[02 07].each do |chapter|
      path = "content/book/part1-golang/chapter#{chapter}.md"
      FileUtils.cp(File.join(root, path), File.join(dir, path))
    end
    path = 'tests/fixtures/book-regions.json'
    FileUtils.cp(File.join(root, path), File.join(dir, path))
    mutate.call(dir)
    out, err, status = Open3.capture3(RbConfig.ruby, checker, dir)
    if failure
      abort "#{name}: did not reject expected fault: #{out}#{err}" unless !status.success? && err.include?(failure)
    else
      abort "#{name}: rejected valid layout: #{out}#{err}" unless status.success?
    end
    puts "PASS: #{name}"
  end
end
