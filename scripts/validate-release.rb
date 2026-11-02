#!/usr/bin/env ruby
# Export exact candidate bytes before running checks; never stage or commit for the user.
require 'optparse'
require 'open3'
require 'tmpdir'
require 'fileutils'

options = {checks: 'release'}
parser = OptionParser.new do |p|
  p.banner = 'Usage: ruby scripts/validate-release.rb --mode working|index|revision [--ref COMMIT] [--checks docs|validate|release]'
  p.on('--mode MODE', %w[working index revision]) { |v| options[:mode] = v }
  p.on('--ref REF') { |v| options[:ref] = v }
  p.on('--checks CHECKS', %w[docs validate release]) { |v| options[:checks] = v }
end
begin
  parser.parse!
  abort parser.to_s unless options[:mode] && ARGV.empty?
  abort '--ref is required only for revision mode' unless (options[:mode] == 'revision') == !options[:ref].nil?
rescue OptionParser::ParseError => e
  abort e.message
end

def git!(*args)
  out, err, status = Open3.capture3('git', *args)
  abort "snapshot: git #{args.first} failed: #{err}" unless status.success?
  out
end

root = Dir.pwd
abort 'snapshot: run at repository root' unless File.realpath(git!('rev-parse', '--show-toplevel').strip) == File.realpath(root)
entries = {}
identity = options[:mode]
if options[:mode] == 'working'
  git!('ls-files', '-z', '--cached', '--others', '--exclude-standard').split("\0").uniq.each do |path|
    next unless File.exist?(path) || File.symlink?(path) # Unstaged deletion is excluded in working mode only.
    entries[path] = nil
  end
else
  if options[:mode] == 'revision'
    commit = git!('rev-parse', '--verify', '--end-of-options', options[:ref] + '^{commit}').strip
    identity += " #{commit}"
    rows = git!('ls-tree', '-rz', commit).split("\0")
  else
    rows = git!('ls-files', '--stage', '-z').split("\0")
  end
  rows.each do |row|
    metadata, path = row.split("\t", 2)
    mode, kind_or_sha, sha_or_stage = metadata.split(' ')
    abort "snapshot: unsupported file mode #{mode}: #{path}" unless %w[100644 100755].include?(mode)
    abort 'snapshot: resolve index conflicts first' if options[:mode] == 'index' && sha_or_stage != '0'
    sha = options[:mode] == 'index' ? kind_or_sha : sha_or_stage
    entries[path] = [mode, sha]
  end
end
abort 'snapshot: no selected files; select an explicit candidate before publishing' if entries.empty?

entries.each_key do |path|
  parts = path.split('/')
  forbidden = parts.any? { |p| %w[.git .specs .tools .generated .DS_Store].include?(p) || p.start_with?('._') }
  forbidden ||= path.start_with?('site/', 'dist/', 'dist-interview/', '.astro-interview/', '.pages-dist') || (File.basename(path).start_with?('.env') && File.basename(path) != '.env.example')
  abort "snapshot: private or unsafe candidate path: #{path}" if forbidden || path.start_with?('/') || parts.include?('..')
  if options[:mode] == 'working'
    parts.each_index { |i| abort "snapshot: symlink forbidden: #{path}" if File.symlink?(parts[0..i].join('/')) }
    abort "snapshot: not a regular file: #{path}" unless File.file?(path)
  end
end
%w[README.md Makefile labs/go.mod content/book/README.md].each do |path|
  abort "snapshot: missing required file #{path}" unless entries.key?(path)
end
%w[go.mod go.sum go.work].each do |path|
  abort "snapshot: root Go workspace file is forbidden: #{path}" if entries.key?(path)
end

$stdout.sync = true
puts "snapshot: #{identity}; #{entries.size} files; checks=#{options[:checks]}"
puts 'working is a rehearsal, not a fixed-commit publication result' if options[:mode] == 'working'
Dir.mktmpdir('ai-engineer-release-') do |dir|
  entries.each do |path, object|
    target = File.join(dir, path)
    FileUtils.mkdir_p(File.dirname(target))
    if object
      mode, sha = object
      File.binwrite(target, git!('cat-file', 'blob', sha))
      File.chmod(mode == '100755' ? 0755 : 0644, target)
    else
      FileUtils.cp(path, target, preserve: true)
    end
  end
  targets = options[:checks] == 'docs' ? ['validate-docs'] : ['validate']
  targets += %w[security release-readiness] if options[:checks] == 'release'
  targets.each do |target|
    abort "snapshot: #{target} failed; not ready" unless system('make', target, chdir: dir)
  end
end
puts "snapshot: #{options[:checks]} checks passed for #{identity}"
