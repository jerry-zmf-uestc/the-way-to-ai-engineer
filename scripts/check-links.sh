#!/bin/sh
set -eu

ruby -E UTF-8:UTF-8 -e '
  broken = []
  root = Dir.pwd
  files = Dir.glob("**/*.md", File::FNM_DOTMATCH).reject do |path|
    parts = path.split("/")
    parts.any? { |part| %w[.git .specs .tools .astro node_modules dist].include?(part) } ||
      path.start_with?("src/content/chapters/")
  end
  abort "no public Markdown files found" if files.empty?
  files.uniq.sort.each do |file|
    prose = File.read(file).gsub(/```.*?```/m, "").gsub(/`[^`\n]+`/, "")
    prose.scan(/\[[^\]]*\]\(([^)]+)\)/).flatten.each do |link|
      next if link.match?(%r{^(https?://|mailto:|#)})
      relative = link.sub(/#.*$/, "")
      next if relative.empty?
      target = File.expand_path(relative, File.dirname(file))
      if !target.start_with?(root + "/") || target.delete_prefix(root + "/").split("/").any? { |v| %w[.specs .git .tools].include?(v) }
        broken << "#{file}: private target or path outside repository: #{link}"
      elsif !File.exist?(target)
        broken << "#{file}: #{link}"
      end
    end
  end

  abort "broken Markdown links:\n#{broken.join("\n")}" unless broken.empty?
  puts "Markdown links: public relative targets resolve without private files"
'
