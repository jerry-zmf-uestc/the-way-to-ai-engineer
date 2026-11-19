#!/bin/sh
set -eu

ruby -ryaml -rdate -e '
  files = Dir.glob("labs/**/meta.yml").sort
  abort "no topic metadata found" if files.empty?

  tracks = { 1 => "golang", 2 => "backend", 3 => "agent" }
  statuses = %w[planned drafted validated]
  kinds = %w[original book-port warmup elective reading]

  files.each do |meta_path|
    dir = File.dirname(meta_path)
    %w[README.md meta.yml].each do |name|
      abort "#{dir}: missing #{name}" unless File.file?(File.join(dir, name))
    end

    data = YAML.safe_load(File.read(meta_path), permitted_classes: [Date], aliases: false)
    entries = data.fetch("entrypoints", ["."])
    abort "#{dir}: missing entrypoints" if entries.empty?
    entries.each do |entry|
      abort "#{dir}: unsafe entrypoint" if entry.start_with?("/") || entry.split("/").include?("..")
      abort "#{dir}: entrypoint #{entry} missing main.go" unless File.file?(File.join(dir, entry, "main.go"))
    end
    abort "#{dir}: missing tests" if Dir.glob(File.join(dir, "**/*_test.go")).empty?
    course = data.fetch("course")
    part = Integer(course.fetch("part"))
    chapter = Integer(course.fetch("chapter"))
    slug = data.fetch("topic_slug")
    expected_dir = File.join("labs", "part#{part}-#{tracks.fetch(part)}", format("chapter%02d", chapter), slug)
    abort "#{meta_path}: expected directory #{expected_dir}" unless dir == expected_dir
    abort "#{meta_path}: invalid kind" unless kinds.include?(data.fetch("kind"))
    abort "#{meta_path}: invalid status" unless statuses.include?(data.fetch("status"))

    source = data.fetch("source")
    if data.fetch("kind") == "original"
      abort "#{meta_path}: original topic requires explicit license" if data.fetch("license", "").empty?
      abort "#{meta_path}: original topic cannot hide external source" unless source.nil?
    else
      abort "#{meta_path}: external source metadata required" unless source.is_a?(Hash)
      %w[repository path revision license].each do |key|
        abort "#{meta_path}: empty source.#{key}" if source.fetch(key, "").to_s.strip.empty?
      end
      abort "#{meta_path}: revision must be a full commit SHA" unless source.fetch("revision").match?(/\A[0-9a-f]{40}\z/)
    end

    validation = data.fetch("validation")
    abort "#{meta_path}: pilot must be offline" unless validation.fetch("offline") == true
  end

  puts "topic contracts: #{files.length} topics valid"
'
