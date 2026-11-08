#!/usr/bin/env ruby
require 'json'
abort 'Usage: ruby scripts/verify-release-readiness.rb [--strict]' unless ARGV.empty? || ARGV == ['--strict']
required = %w[licensing third-party-review security-reporting community-contact github-checks reader-trial]
rows = JSON.parse(File.read('docs/release-readiness.json')).fetch('checks')
abort 'release checklist: missing, unexpected or duplicate checks' unless rows.map { |r| r.fetch('id') }.sort == required.sort
rows.each do |row|
  abort "release checklist: invalid status #{row['id']}" unless %w[pending confirmed].include?(row.fetch('status'))
  evidence = row.fetch('evidence')
  abort "release checklist: missing evidence #{row['id']}" unless evidence.is_a?(Array) && !evidence.empty?
  evidence.each do |path|
    abort 'release checklist: private or unsafe evidence' if path.start_with?('/') || path.split('/').any? { |p| %w[.. .specs .git .tools].include?(p) }
    abort "release checklist: missing evidence file #{path}" unless File.file?(path)
  end
  abort 'release checklist: explanation required' if row.fetch('note').strip.empty?
end
pending = rows.select { |r| r['status'] != 'confirmed' }.map { |r| r['id'] }
abort "release blocked: #{pending.join(', ')}" if ARGV == ['--strict'] && !pending.empty?
puts "release checklist: valid structure; #{pending.size} pending human/remote checks"
