#!/usr/bin/env ruby
require 'tmpdir'
require 'fileutils'
require 'open3'
require 'yaml'
root = File.expand_path('..', __dir__)
checker = File.join(root, 'scripts/validate-topics.sh')
Dir.mktmpdir('topic-metadata-test-') do |dir|
  relative = 'labs/part1-golang/chapter01'
  FileUtils.mkdir_p(File.join(dir, relative))
  FileUtils.cp_r(File.join(root, relative, 'hello-world'), File.join(dir, relative))
  path = File.join(dir, relative, 'hello-world/meta.yml')
  original = YAML.safe_load(File.read(path), aliases: false)
  cases = [
    ['original metadata', original, nil],
    ['missing original license', original.merge('license'=>''), 'explicit license'],
    ['external content without source', original.merge('kind'=>'book-port'), 'source metadata required'],
    ['external content incomplete source', original.merge('kind'=>'book-port', 'source'=>{'repository'=>'example/public'}), 'empty source.path']
  ]
  cases.each do |name, metadata, expected|
    File.write(path, YAML.dump(metadata))
    out, status = Open3.capture2e('/bin/sh', checker, chdir: dir)
    valid = expected ? (!status.success? && out.include?(expected)) : status.success?
    abort "#{name}: #{out}" unless valid
    puts "PASS: #{name}"
  end
end
