#!/usr/bin/env ruby
require 'yaml'

workflows = Dir.glob('.github/workflows/*.yml').sort
abort 'CI: ci.yml and pages.yml are required' unless workflows.map { |path| File.basename(path) }.sort == %w[ci.yml pages.yml]

workflows.each do |path|
  workflow = YAML.safe_load(File.read(path), aliases: false)
  name = File.basename(path)
  abort "#{name}: unsafe or unexpected events" unless workflow.fetch('on').keys.sort == %w[pull_request push workflow_dispatch]
  abort "#{name}: contents read-only permissions required" unless workflow.fetch('permissions') == {'contents' => 'read'}

  workflow.fetch('jobs').each do |job_name, job|
    if job.key?('permissions')
      allowed = name == 'pages.yml' && job_name == 'deploy' &&
        job['permissions'] == {'pages' => 'write', 'id-token' => 'write'}
      abort "#{name}: unexpected job permission escalation" unless allowed
    end
    abort "#{name}: timeout required for #{job_name}" unless job.fetch('timeout-minutes', 0).between?(1, 30)
    job.fetch('steps').each do |step|
      next unless step['uses']
      abort "#{name}: third-party actions must use full commit SHA" unless step['uses'].match?(/\A[\w-]+\/[\w-]+@[0-9a-f]{40}\z/)
      if step['uses'].start_with?('actions/checkout@')
        abort "#{name}: checkout must not persist credentials" unless step.fetch('with').fetch('persist-credentials') == false
      end
      if step['uses'].start_with?('actions/setup-go@')
        settings = step.fetch('with')
        abort "#{name}: Go version must come from labs/go.mod" unless settings['go-version-file'] == 'labs/go.mod'
        abort "#{name}: Go cache must follow labs/go.sum" unless settings['cache'] == true && settings['cache-dependency-path'] == 'labs/go.sum'
      end
    end
  end
end
Dir.glob('.github/ISSUE_TEMPLATE/*.yml').each do |path|
  form = YAML.safe_load(File.read(path), aliases: false)
  %w[name description body].each { |key| abort "#{path}: missing #{key}" unless form.key?(key) }
  ids = form['body'].map { |item| item['id'] }.compact
  abort "#{path}: duplicate field id" unless ids.uniq == ids
end
puts 'CI policy: pinned actions, bounded jobs, read-only validation and scoped Pages deployment valid (remote execution not proven)'
