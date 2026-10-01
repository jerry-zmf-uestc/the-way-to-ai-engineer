#!/usr/bin/env ruby
require 'tmpdir'
require 'open3'
require 'rbconfig'

checker = File.expand_path('../scripts/security.rb', __dir__)
Dir.mktmpdir('security-command-test-') do |dir|
  File.symlink('/usr/bin/uname', File.join(dir, 'uname')) # System Ruby invokes uname during boot on macOS.
  out, status = Open3.capture2e({'PATH' => dir}, RbConfig.ruby, checker, chdir: dir)
  abort "security must fail when scanners are absent: #{out}" if status.success?
  abort "missing scanner must be actionable: #{out}" unless out.include?('missing security tool')
  puts 'PASS: missing security tools block security checks'
  # External executables are the seam: a wrong build version must be rejected
  # before any scanner is allowed to run.
  File.write(File.join(dir, 'gitleaks'), "#!/bin/sh\necho scanner-must-not-run\n")
  File.chmod(0755, File.join(dir, 'gitleaks'))
  File.write(File.join(dir, 'go'), "#!/bin/sh\nprintf 'mod github.com/zricethezav/gitleaks/v8 v0.0.0\\n'\n")
  File.chmod(0755, File.join(dir, 'go'))
  out, status = Open3.capture2e({'PATH' => dir}, RbConfig.ruby, checker, chdir: dir)
  abort "wrong tool version accepted: #{out}" unless !status.success? && out.include?('version mismatch')
  abort 'scanner ran before version validation' if out.include?('scanner-must-not-run')
  puts 'PASS: wrong scanner build version rejected before execution'
end
