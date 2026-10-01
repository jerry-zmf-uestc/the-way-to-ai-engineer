#!/usr/bin/env ruby
require 'tmpdir'
require 'fileutils'
require 'open3'
require 'rbconfig'

checker = File.expand_path('../scripts/validate-release.rb', __dir__)
Dir.mktmpdir('release-snapshot-test-') do |dir|
  def command!(dir, *args)
    out, status = Open3.capture2e(*args, chdir: dir)
    abort "fixture command failed: #{out}" unless status.success?
    out
  end
  command!(dir, 'git', 'init', '-q')
  FileUtils.mkdir_p(File.join(dir, 'book'))
  FileUtils.mkdir_p(File.join(dir, 'labs'))
  File.write(File.join(dir, '.gitignore'), ".specs/\n")
  File.write(File.join(dir, 'README.md'), "staged\n")
  File.write(File.join(dir, 'labs/go.mod'), "module fixture/labs\n")
  File.write(File.join(dir, 'book/README.md'), "# Book\n")
  File.write(File.join(dir, 'Makefile'), "validate-docs:\n\t@test ! -e .specs\n\t@test ! -e .git\n\t@test \"$$(cat README.md)\" = staged\n")
  command!(dir, 'git', 'add', '.')
  File.write(File.join(dir, 'README.md'), "unstaged\n")
  FileUtils.mkdir_p(File.join(dir, '.specs'))
  File.write(File.join(dir, '.specs/private.md'), 'private')
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--mode', 'index', '--checks', 'docs', chdir: dir)
  abort "index must use staged bytes, excluding private files: #{out}" unless status.success?
  abort "index identity missing: #{out}" unless out.include?('snapshot: index')
  puts 'PASS: index exports staged bytes, not worktree or private files'
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--mode', 'working', '--checks', 'docs', chdir: dir)
  abort "working must see edited bytes and fail fixture check: #{out}" if status.success?
  puts 'PASS: working mode does not disguise unstaged changes'
  command!(dir, 'git', '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture baseline')
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--mode', 'revision', '--ref', 'HEAD', '--checks', 'docs', chdir: dir)
  abort "revision must ignore worktree: #{out}" unless status.success?
  puts 'PASS: revision exports a resolved commit'
  command!(dir, 'git', 'add', '-f', '.specs/private.md')
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--mode', 'index', '--checks', 'docs', chdir: dir)
  abort "tracked private file must be rejected: #{out}" unless !status.success? && out.include?('private or unsafe candidate')
  puts 'PASS: force-added private files are rejected, not silently omitted'
  command!(dir, 'git', 'rm', '--cached', '-q', '.specs/private.md', 'labs/go.mod')
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--mode', 'index', '--checks', 'docs', chdir: dir)
  abort "missing required file must fail: #{out}" unless !status.success? && out.include?('missing required file labs/go.mod')
  puts 'PASS: missing module blocks snapshot checks'
  command!(dir, 'git', 'add', 'labs/go.mod')
  File.write(File.join(dir, 'go.mod'), "module forbidden-root\n")
  command!(dir, 'git', 'add', 'go.mod')
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--mode', 'index', '--checks', 'docs', chdir: dir)
  abort "root module must fail: #{out}" unless !status.success? && out.include?('root Go workspace file is forbidden: go.mod')
  puts 'PASS: root Go module cannot blur the labs boundary'
  command!(dir, 'git', 'rm', '--cached', '-q', 'go.mod')
  FileUtils.rm(File.join(dir, 'go.mod'))
  File.symlink('README.md', File.join(dir, 'link'))
  command!(dir, 'git', 'add', 'link')
  out, status = Open3.capture2e(RbConfig.ruby, checker, '--mode', 'index', '--checks', 'docs', chdir: dir)
  abort "symlink must fail: #{out}" unless !status.success? && out.include?('unsupported file mode')
  puts 'PASS: symlinks cannot escape the snapshot boundary'
end
