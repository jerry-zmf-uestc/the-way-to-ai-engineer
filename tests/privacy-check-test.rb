#!/usr/bin/env ruby
require 'tmpdir'
require 'fileutils'
require 'open3'

checker = File.expand_path('../scripts/check-privacy-leaks.sh', __dir__)
rg, rg_status = Open3.capture2('/bin/sh', '-c', 'command -v rg')
abort 'install ripgrep to run real privacy regression tests' unless rg_status.success?
Dir.mktmpdir('privacy-check-test-') do |dir|
  FileUtils.mkdir_p(File.join(dir, 'bin'))
  File.write(File.join(dir, 'README.md'), "# Safe example\n")
  out, status = Open3.capture2e({'PATH' => File.join(dir, 'bin')}, '/bin/sh', checker, chdir: dir)
  abort "missing scanner must fail, got #{status.exitstatus}: #{out}" if status.success?
  abort "missing scanner diagnostic absent: #{out}" unless out.include?('ripgrep (rg) is required')
  puts 'PASS: missing scanner fails with actionable diagnostic'
  fake = File.join(dir, 'bin/rg')
  File.write(fake, "#!/bin/sh\nexit 2\n")
  FileUtils.chmod(0755, fake)
  out, status = Open3.capture2e({'PATH' => File.join(dir, 'bin')}, '/bin/sh', checker, chdir: dir)
  abort "scanner error must fail: #{out}" if status.success?
  abort "scanner error diagnostic absent: #{out}" unless out.include?('scanner failed')
  puts 'PASS: scanner execution error cannot report success'
  FileUtils.rm(fake)
  FileUtils.ln_s(rg.strip, fake)
  FileUtils.mkdir_p(File.join(dir, 'node_modules/dependency'))
  dependency_path = File.join('', 'Users', 'dependency', 'build', 'path')
  File.write(File.join(dir, 'node_modules/dependency/generated.js'), dependency_path)
  out, status = Open3.capture2e({'PATH' => File.join(dir, 'bin')}, '/bin/sh', checker, chdir: dir)
  abort "safe source with ignored dependencies failed: #{out}" unless status.success?
  puts 'PASS: real scanner accepts safe source and ignores installed dependencies'
  secret = 'sk-' + 'invalid-test-only-' * 3
  File.write(File.join(dir, 'example.txt'), secret)
  out, status = Open3.capture2e({'PATH' => File.join(dir, 'bin')}, '/bin/sh', checker, chdir: dir)
  abort "secret sample not blocked: #{out}" if status.success?
  abort 'secret value leaked in diagnostic' if out.include?(secret)
  abort 'missing affected filename' unless out.include?('example.txt')
  puts 'PASS: secret-shaped sample blocked without printing its value'
  FileUtils.rm(File.join(dir, 'README.md'))
  FileUtils.rm(File.join(dir, 'example.txt'))
  FileUtils.rm_r(File.join(dir, 'node_modules'))
  FileUtils.rm(fake)
  FileUtils.rmdir(File.join(dir, 'bin'))
  out, status = Open3.capture2e('/bin/sh', checker, chdir: dir)
  abort "empty scan must not pass: #{out}" if status.success?
  abort "empty scan diagnosis missing: #{out}" unless out.include?('no scannable files')
  puts 'PASS: empty scan cannot masquerade as a clean repository'
end
