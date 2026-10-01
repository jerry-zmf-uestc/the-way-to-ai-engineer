#!/usr/bin/env ruby
require 'tmpdir'
require 'fileutils'
require 'open3'
checker = File.expand_path('../scripts/check-links.sh', __dir__)
Dir.mktmpdir('public-links-test-') do |dir|
  FileUtils.mkdir_p(File.join(dir, '.specs'))
  File.write(File.join(dir, '.specs/private.md'), "# Private\n")
  File.write(File.join(dir, 'README.md'), "[private](.specs/private.md)\n")
  out, status = Open3.capture2e('/bin/sh', checker, chdir: dir)
  abort "private dependency accepted: #{out}" if status.success?
  abort "private dependency not diagnosed: #{out}" unless out.include?('private target')
  puts 'PASS: private link rejected even when local file exists'
  File.write(File.join(dir, 'README.md'), "# Public\n")
  File.write(File.join(dir, '.specs/private.md'), "[missing](missing.md)\n")
  FileUtils.mkdir_p(File.join(dir, 'node_modules/dependency'))
  File.write(File.join(dir, 'node_modules/dependency/README.md'), "[missing](missing.md)\n")
  FileUtils.mkdir_p(File.join(dir, 'src/content/chapters'))
  File.write(File.join(dir, 'src/content/chapters/generated.md'), "[deployed](/course/page/)\n")
  out, status = Open3.capture2e('/bin/sh', checker, chdir: dir)
  abort "private/generated/dependency files must not affect public checks: #{out}" unless status.success?
  puts 'PASS: private, generated, and dependency files are outside public source validation'
  File.write(File.join(dir, 'README.md'), "[missing](missing.md)\n")
  out, status = Open3.capture2e('/bin/sh', checker, chdir: dir)
  abort "public broken link accepted: #{out}" if status.success?
  puts 'PASS: missing public target fails'
end
