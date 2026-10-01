#!/usr/bin/env ruby
require 'json'
require 'pathname'

# An optional root lets regression checks run on disposable copies.
root = File.expand_path(ARGV.fetch(0, '..'), ARGV.empty? ? __dir__ : Dir.pwd)
Dir.chdir(root)
topics = {
  2 => %w[basic-types strings-runes collections struct-json],
  7 => %w[reflection generics]
}
anchors = {}
regions = []
topics.each do |chapter, ids|
  directory = format('book/part1-golang/chapter%02d', chapter)
  abort "duplicate chapter directory: #{directory}" if File.exist?(directory)
  path = directory + '.md'
  text = File.read(path)
  prose = text.gsub(/```.*?```/m, '')
  abort "#{path}: expected one title" unless prose.scan(/^# /).length == 1
  levels = prose.scan(/^(\#{1,6}) /).flatten.map(&:length)
  abort "#{path}: skipped heading level" unless levels.each_cons(2).all? { |a,b| b <= a + 1 }
  actual_ids = prose.scan(/<a id="([^"]+)"><\/a>/).flatten
  expected_ids = ids + ids.map { |id| id + '-exercises' }
  abort "#{path}: topic anchors missing, duplicated or out of order" unless actual_ids == expected_ids
  anchors[File.expand_path(path)] = actual_ids
  regions.concat(ids.map { |id| "#{path}##{id}" })
  expected = (1..ids.length).to_a
  numbers = prose.scan(/^\| (?:<a id="[^"]+"><\/a>)?#{chapter}\.(\d+) /).flatten.map(&:to_i)
  abort "#{path}: exercises must be sequential" unless numbers == expected
end
mapped = JSON.parse(File.read('tests/fixtures/book-regions.json')).fetch('regions')
actual_regions = mapped.select { |location| anchors.key?(File.expand_path(location.split('#', 2).first)) }
abort 'merged chapter regions are incomplete or repeated' unless actual_regions.sort == regions.sort

links_checked = 0
Dir.glob('**/*.md').uniq.each do |path|
  prose = File.read(path).gsub(/```.*?```/m, '')
  prose.scan(/\[[^\]]*\]\(([^)]+)\)/).flatten.each do |link|
    next if link.match?(%r{^(https?://|mailto:)})
    relative, anchor = link.split('#', 2)
    target = relative.empty? ? File.expand_path(path) : File.expand_path(relative, File.dirname(path))
    next unless anchors.key?(target) && anchor
    abort "#{path}: missing merged chapter anchor #{link}" unless anchors.fetch(target).include?(anchor)
    links_checked += 1
  end
end
puts "merged chapters: 2 single files, 6 topics, 6 sequential exercises; #{links_checked} anchor links valid"
