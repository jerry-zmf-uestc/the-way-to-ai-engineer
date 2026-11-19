#!/usr/bin/env ruby
require 'tmpdir'
require 'fileutils'
require 'open3'
require 'rbconfig'
root = File.expand_path('..', __dir__)
checker = File.join(root, 'scripts/verify-ci.rb')
Dir.mktmpdir('ci-policy-test-') do |dir|
  FileUtils.cp_r(File.join(root, '.github'), dir)
  file = File.join(dir, '.github/workflows/ci.yml')
  original = File.read(file)
  File.write(file, original.sub(/actions\/checkout@[0-9a-f]{40}/, 'actions/checkout@main'))
  out, status = Open3.capture2e(RbConfig.ruby, checker, chdir: dir)
  abort "mutable action must be rejected: #{out}" unless !status.success? && out.include?('full commit SHA')
  puts 'PASS: mutable action reference rejected'
  [
    ['permission escalation', original.sub('contents: read', 'contents: write'), 'read-only permissions'],
    ['privileged PR event', original.sub('pull_request:', 'pull_request_target:'), 'unsafe or unexpected events'],
    ['persistent checkout credential', original.sub('persist-credentials: false', 'persist-credentials: true'), 'not persist credentials'],
    ['root Go module path', original.sub('go-version-file: labs/go.mod', 'go-version-file: go.mod'), 'labs/go.mod'],
    ['root Go cache path', original.sub('cache-dependency-path: labs/go.sum', 'cache-dependency-path: go.sum'), 'labs/go.sum']
  ].each do |name, body, expected|
    File.write(file, body)
    out, status = Open3.capture2e(RbConfig.ruby, checker, chdir: dir)
    abort "#{name} accepted: #{out}" unless !status.success? && out.include?(expected)
    puts "PASS: #{name} rejected"
  end
  File.write(file, original)
  out, status = Open3.capture2e(RbConfig.ruby, checker, chdir: dir)
  abort "valid CI rejected: #{out}" unless status.success?
  puts 'PASS: configured CI policy accepted'

  pages = File.join(dir, '.github/workflows/pages.yml')
  pages_original = File.read(pages)
  File.write(pages, pages_original.sub(/actions\/deploy-pages@[0-9a-f]{40}/, 'actions/deploy-pages@v4'))
  out, status = Open3.capture2e(RbConfig.ruby, checker, chdir: dir)
  abort "mutable Pages action accepted: #{out}" unless !status.success? && out.include?('full commit SHA')
  puts 'PASS: mutable Pages action reference rejected'

  File.write(pages, pages_original.sub('pages: write', 'pages: read'))
  out, status = Open3.capture2e(RbConfig.ruby, checker, chdir: dir)
  abort "invalid Pages permission accepted: #{out}" unless !status.success? && out.include?('permission escalation')
  puts 'PASS: invalid Pages permission rejected'
end
