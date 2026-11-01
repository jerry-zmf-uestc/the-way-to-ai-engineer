#!/bin/sh
set -eu

ruby -E UTF-8:UTF-8 -e '
  path = "docs/designs/project-plan.md"
  text = File.read(path)
  anchor = %(<a id="chapter-map"></a>)
  abort "chapter map anchor must occur once" unless text.scan(anchor).length == 1
  start = text.index(anchor) + anchor.length
  finish = text.index(/^## 5\./, start)
  abort "chapter map must end before section 5" unless finish
  rows = text[start...finish].lines.grep(/^\| \d+ \|/).map { |line| line.split("|")[1...-1].map(&:strip) }
  abort "chapter rows must have 9 columns" unless rows.all? { |row| row.length == 9 }
  numbers = rows.map { |row| Integer(row[0]) }
  expected = (1..33).to_a
  abort "chapter numbers must be unique and continuous from 1 to 33: #{numbers.inspect}" unless numbers == expected

  allowed = [%w[planned partial available], %w[pending passed failed], %w[pending reviewed needs-revision], %w[pending validated needs-revision]]
  rows.each do |row|
    allowed.each_with_index do |values, offset|
      status = row[4 + offset]
      abort "chapter #{row[0]} has invalid status #{status.inspect} in column #{4 + offset}" unless values.include?(status)
    end
    number = Integer(row[0])
    part, track = number <= 11 ? [1, "golang"] : number <= 21 ? [2, "backend"] : [3, "agent"]
    expected_path = row[2].delete("`")
    abort "chapter #{number} has incorrect Book path" unless expected_path.start_with?(format("book/part%d-%s/", part, track)) && expected_path.end_with?(".md")
    if row[4] == "available" || row[6] == "reviewed"
      abort "chapter #{number} has no book file" unless File.file?(expected_path)
    end
    if row[5] == "passed"
      row[3].split("\u{3001}").each do |topic|
        lab = format("labs/part%d-%s/chapter%02d/%s", part, track, number, topic)
        abort "chapter #{number} has no lab #{topic}" unless File.directory?(lab)
      end
    end
    if row[4] != "planned" || row[5..7].any? { |s| s != "pending" }
      required = []
      required << "code" if row[5] != "pending"
      required << "content" if row[6] != "pending"
      required << "reader" if row[7] != "pending"
      abort "chapter #{number} needs implementation evidence" if required.empty? && row[8] == "\u{2014}"
      required.each do |kind|
        link = row[8][/#{kind}: \[[^\]]+\]\(([^)]+)\)/, 1]
        abort "chapter #{number} missing #{kind} evidence" unless link
        target, anchor = link.split("#", 2)
        full = File.expand_path(target, File.dirname(path))
        abort "chapter #{number} private evidence" if target.split("/").include?(".specs")
        abort "chapter #{number} missing evidence file" unless File.file?(full)
        if anchor
          abort "chapter #{number} missing evidence anchor" unless File.read(full).include?(%(<a id="#{anchor}"></a>))
        end
        if kind == "reader"
          abort "chapter #{number} reader evidence cannot be a script or author review" unless full.end_with?(".md") && anchor && anchor.start_with?("reader-")
        end
      end
    end
  end

  puts "chapter map: 33 unique chapters; paths, four status fields and evidence links valid"
'
