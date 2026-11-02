#!/usr/bin/env ruby
require 'json'
require 'open3'
require 'fileutils'

tools = JSON.parse(File.read(File.join(__dir__, 'security-tools.json')))
abort 'Usage: ruby scripts/security.rb [--install | --history]' unless ARGV.empty? || [%w[--install], %w[--history]].include?(ARGV)
if ARGV == ['--install']
  directory = File.expand_path('.tools/bin')
  FileUtils.mkdir_p(directory)
  env = {'GOBIN' => directory, 'GOTOOLCHAIN' => 'local', 'GOPROXY' => 'https://proxy.golang.org,direct', 'GOSUMDB' => 'sum.golang.org'}
  tools.each_value do |tool|
    abort "tool installation failed: #{tool['package']}" unless system(env, 'go', 'install', "#{tool.fetch('package')}@#{tool.fetch('version')}")
  end
  puts 'security tools installed in .tools/bin; explicitly add that directory to PATH'
  exit
end

binaries = {}
tools.each do |name, tool|
  binary = ENV.fetch('PATH', '').split(File::PATH_SEPARATOR).map { |p| File.join(p, name) }.find { |p| File.file?(p) && File.executable?(p) }
  abort "missing security tool #{name}; run make tools-install and add .tools/bin to PATH" unless binary
  begin
    info, status = Open3.capture2e('go', 'version', '-m', binary)
  rescue Errno::ENOENT
    abort 'Go is required to verify security tool build versions'
  end
  expected = ['mod', tool.fetch('module'), tool.fetch('version')]
  abort "security tool version mismatch: #{name}; reinstall pinned tools" unless status.success? && info.lines.any? { |l| l.split.first(3) == expected }
  binaries[name] = binary
end

if ARGV == ['--history']
  abort 'history check requires an existing Git commit; initial candidates need directory scanning' unless system('git', 'rev-parse', '--verify', 'HEAD', out: File::NULL, err: File::NULL)
  command = [binaries.fetch('gitleaks'), 'git', '--redact', '--no-banner', '--config', '.gitleaks.toml', '--log-opts=--all', '.']
  abort 'history security check failed' unless system(*command)
else
  puts 'security: scanning directory; querying Go vulnerability database (network required)'
  leaks_ok = system(binaries.fetch('gitleaks'), 'dir', '--redact', '--no-banner', '--config', '.gitleaks.toml', '.')
  vulnerabilities_ok = system({'GOTOOLCHAIN' => 'local', 'GOPROXY' => 'off', 'GOSUMDB' => 'off', 'GOWORK' => 'off'}, binaries.fetch('govulncheck'), './...', chdir: 'labs')
  abort 'security check failed; see redacted diagnostics above' unless leaks_ok && vulnerabilities_ok
end
puts 'security checks passed for the selected scope; not a guarantee of absence of vulnerabilities'
