#!/usr/bin/env ruby
require 'tmpdir'
require 'fileutils'
require 'open3'
require 'rbconfig'
require 'json'
checker = File.expand_path('../scripts/verify-release-readiness.rb', __dir__)
ids = %w[licensing third-party-review security-reporting community-contact github-checks reader-trial]
Dir.mktmpdir('readiness-test-') do |dir|
  FileUtils.mkdir_p(File.join(dir, 'docs'))
  File.write(File.join(dir, 'evidence.md'), '# Explicit fixture evidence')
  path = File.join(dir, 'docs/release-readiness.json')
  rows = ids.map { |id| {'id'=>id, 'status'=>'pending', 'evidence'=>['evidence.md'], 'note'=>'not performed'} }
  File.write(path, JSON.generate({'checks'=>rows}))
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--strict', chdir: dir)
  abort "pending checks must block release: #{out}" unless !status.success? && out.include?('release blocked:')
  puts 'PASS: pending checks block release'
  out, status = Open3.capture2e(RbConfig.ruby, checker, chdir: dir)
  abort "pending is valid during development: #{out}" unless status.success?
  puts 'PASS: development may retain explicit pending checks'
  rows.each { |row| row['status'] = 'confirmed'; row['note'] = 'fixture review completed' }
  File.write(path, JSON.generate({'checks'=>rows}))
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--strict', chdir: dir)
  abort "complete fixture rejected: #{out}" unless status.success?
  puts 'PASS: complete checklist accepted'
  rows.pop
  File.write(path, JSON.generate({'checks'=>rows}))
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--strict', chdir: dir)
  abort "removed requirement accepted: #{out}" if status.success?
  puts 'PASS: deleting a required check cannot bypass the gate'
end
