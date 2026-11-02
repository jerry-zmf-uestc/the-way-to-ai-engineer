#!/usr/bin/env ruby
# 从正文提取样章代码，在临时副本验证示例、故障、修复与练习可完成性。
require 'tmpdir'
require 'fileutils'
require 'open3'

ROOT = File.expand_path('..', __dir__)
ENVIRONMENT = { 'GOTOOLCHAIN' => 'local', 'GOPROXY' => 'off', 'GOSUMDB' => 'off' }.freeze
blocks = {}
%w[chapter01 chapter02].each do |chapter|
  text = File.read(File.join(ROOT, "content/book/part1-golang/#{chapter}.md"))
  text += File.read(File.join(ROOT, 'tests/fixtures/chapter01-exercises.md')) if chapter == 'chapter01'
  text.scan(/<!-- verify:([\w-]+) -->\s*```go\n(.*?)\n```/m).each do |id, code|
    abort "duplicate block: #{id}" if blocks.key?(id)
    blocks[id] = code + "\n"
  end
end
used = []
snippet = lambda do |id|
  used << id
  blocks.fetch(id)
end

def run!(dir, *command, output: nil, failure: nil)
  stdout, stderr, status = Open3.capture3(ENVIRONMENT, *command, chdir: dir)
  combined = stdout + stderr
  if failure
    abort "expected failure: #{command.join(' ')}\n#{combined}" if status.success? || !combined.match?(failure)
  else
    abort "command failed: #{command.join(' ')}\n#{combined}" unless status.success?
    abort "output mismatch: #{command.join(' ')}\n#{combined}" if output && stdout != output
  end
  combined
end

Dir.mktmpdir('book-pilots-007-') do |dir|
  FileUtils.cp(File.join(ROOT, 'labs/go.mod'), File.join(dir, 'go.mod'))
  labs = {
    'hello-world' => 'chapter01/hello-world',
    'arrays-slices' => 'chapter02/arrays-slices',
    'maps' => 'chapter02/maps'
  }
  labs.each { |name, path| FileUtils.cp_r(File.join(ROOT, "labs/part1-golang/#{path}"), File.join(dir, name)) }
  full = lambda do |id, expected = nil, failure = nil|
    File.write(File.join(dir, 'inspect.go'), snippet.call(id))
    run!(dir, 'go', 'run', 'inspect.go', output: expected, failure: failure)
  end
  originals = %w[hello-world arrays-slices maps].to_h do |name|
    [name, %w[main.go main_test.go].to_h { |file| [file, File.read(File.join(dir, name, file))] }]
  end
  restore = lambda do |name|
    originals.fetch(name).each { |file, body| File.write(File.join(dir, name, file), body) }
  end
  write_lab = lambda { |name, file, body| File.write(File.join(dir, name, file), body) }

  abort 'hello source mismatch' unless snippet.call('hello') == originals['hello-world']['main.go']
  abort 'hello test mismatch' unless snippet.call('hello-test') == originals['hello-world']['main_test.go']
  {
    'window' => 'arrays-slices', 'append-copy' => 'arrays-slices',
    'count-words' => 'maps', 'lookup' => 'maps', 'unique-words' => 'maps'
  }.each do |id, name|
    abort "source excerpt mismatch: #{id}" unless originals[name]['main.go'].include?(snippet.call(id).strip)
  end
  run!(dir, 'go', 'run', './hello-world', output: "hello, go\n")
  run!(dir, 'go', 'test', './hello-world', '-count=1')
  run!(dir, 'go', 'build', '-o', File.join(dir, 'hello-bin'), './hello-world')
  run!(dir, File.join(dir, 'hello-bin'), output: "hello, go\n")
  full.call('greet', "hello, gopher\n")

  main = originals['hello-world']['main.go']
  test = originals['hello-world']['main_test.go']
  write_lab.call('hello-world', 'main.go', main.sub('package main', 'package hello'))
  run!(dir, 'go', 'run', './hello-world', failure: /found packages/)
  write_lab.call('hello-world', 'main_test.go', test.sub('package main', 'package hello'))
  run!(dir, 'go', 'run', './hello-world', failure: /not a main package/)
  restore.call('hello-world')
  run!(dir, 'go', 'test', './hello-world', '-count=1')
  run!(dir, 'go', 'run', './hello-world', output: "hello, go\n")
  write_lab.call('hello-world', 'main.go', main.sub('hello, go', 'hello, reader'))
  run!(dir, 'go', 'run', './hello-world', output: "hello, reader\n")
  run!(dir, 'go', 'test', './hello-world', '-count=1', failure: /want "hello, go"/)
  write_lab.call('hello-world', 'main_test.go', test.gsub('hello, go', 'hello, reader'))
  run!(dir, 'go', 'test', './hello-world', '-count=1')

  # 直接提取正文练习，按文件注释放回临时副本，不把摘录当作独立程序。
  exercise_parts = lambda do |id, expected_files|
    sections = snippet.call(id).split(%r{^// 文件：hello-world/(main(?:_test)?\.go)[^\n]*\n})
    sections.shift # 任务和预期注释不属于任何文件。
    pairs = sections.each_slice(2).to_a
    abort "exercise file boundaries mismatch: #{id}" unless pairs.map(&:first).sort == expected_files.sort
    pairs.to_h
  end
  code_only = lambda do |body|
    body.lines.map { |line| line.sub(%r{//.*$}, '').strip }.reject(&:empty?).join("\n")
  end
  exercise1 = exercise_parts.call('exercise-hello-1', %w[main.go main_test.go])
  exercise2 = exercise_parts.call('exercise-hello-2', %w[main.go main_test.go])
  exercise3 = exercise_parts.call('exercise-hello-3', %w[main_test.go])
  exercise1.each do |file, body|
    abort "exercise 1 differs from Lab: #{file}" unless code_only.call(originals['hello-world'][file]).include?(code_only.call(body))
  end

  # 练习 1：先改断言，观察红，再改行为观察绿。
  restore.call('hello-world')
  exercise_main = main.sub(/func message\(\) string \{.*?^\}/m, exercise1.fetch('main.go').strip)
  exercise_test = "package main\nimport \"testing\"\n" + exercise1.fetch('main_test.go')
  write_lab.call('hello-world', 'main.go', exercise_main)
  write_lab.call('hello-world', 'main_test.go', exercise_test)
  run!(dir, 'go', 'test', './hello-world', '-count=1')
  completed_test = exercise_test.gsub('hello, go', 'hello, reader')
  write_lab.call('hello-world', 'main_test.go', completed_test)
  run!(dir, 'go', 'test', './hello-world', '-count=1', failure: /want "hello, reader"/)
  completed_main = exercise_main.sub('return "hello, go"', 'return "hello, reader"')
  write_lab.call('hello-world', 'main.go', completed_main)
  run!(dir, 'go', 'test', './hello-world', '-count=1')
  run!(dir, 'go', 'run', './hello-world', output: "hello, reader\n")

  # 练习 2：签名变化必须同步调用方与旧测试。
  { 'main.go' => completed_main, 'main_test.go' => completed_test }.each do |file, body|
    abort "exercise 2 does not continue exercise 1: #{file}" unless code_only.call(body).include?(code_only.call(exercise2.fetch(file)))
  end
  exercise_main = "package main\nimport \"fmt\"\n" + exercise2.fetch('main.go')
  exercise_test = "package main\nimport \"testing\"\n" + exercise2.fetch('main_test.go')
  parameterized = exercise_main.sub('func message() string', 'func message(name string) string')
                              .sub('return "hello, reader"', 'return "hello, " + name')
  write_lab.call('hello-world', 'main.go', parameterized)
  write_lab.call('hello-world', 'main_test.go', exercise_test)
  run!(dir, 'go', 'test', './hello-world', '-count=1', failure: /not enough arguments/)
  parameterized = parameterized.sub('fmt.Println(message())', 'fmt.Println(message("gopher"))')
  write_lab.call('hello-world', 'main.go', parameterized)
  updated_test = exercise_test.sub('message();', 'message("reader");')
  write_lab.call('hello-world', 'main_test.go', updated_test)
  run!(dir, 'go', 'test', './hello-world', '-count=1')
  run!(dir, 'go', 'run', './hello-world', output: "hello, gopher\n")

  # 练习 3：正文 TODO 未完成时明确失败，补全后覆盖三种输入。
  updated_test += "\n" + exercise3.fetch('main_test.go')
  write_lab.call('hello-world', 'main_test.go', updated_test)
  run!(dir, 'go', 'test', './hello-world', '-count=1', failure: /TODO:/)
  updated_test = updated_test.sub('t.Fatal("TODO: 补全 gopher 测试后替换这行")',
                                 'if got := message("gopher"); got != "hello, gopher" { t.Fatalf("got %q", got) }')
                             .sub('t.Fatal("TODO: 补全空名字测试后替换这行")',
                                  'if got := message(""); got != "hello, " { t.Fatalf("got %q", got) }')
  write_lab.call('hello-world', 'main_test.go', updated_test)
  run!(dir, 'go', 'test', './hello-world', '-v', '-count=1')
  write_lab.call('hello-world', 'main.go', parameterized.sub('return "hello, " + name', 'return "hello, gopher"'))
  run!(dir, 'go', 'test', './hello-world', '-count=1', failure: /FAIL/)
  write_lab.call('hello-world', 'main.go', parameterized)
  run!(dir, 'go', 'test', './hello-world', '-count=1')
  puts 'Chapter 1: source parity, run/test/build, failures/fixes, three exercises passed'

  full.call('array', "[1 2 3]\n[9 2 3]\n3\n")
  full.call('append', "[9 2] [9 2 3]\n2 3 3\n[9 2 3] [7 2 3 4]\n4 true\n")
  run!(dir, 'go', 'run', './arrays-slices', output: "base: [1 2 3]\nextended: [1 2 3 4 5]\nmiddle: [2 3]\n")
  run!(dir, 'go', 'test', './arrays-slices', '-count=1')
  run!(dir, 'go', 'run', './maps', output: "map[go:2 learn:1]\n2 true\n[go learn]\n")
  run!(dir, 'go', 'test', './maps', '-count=1')
  full.call('alias', "[1 99 3]\n")
  fixed_alias = snippet.call('alias').sub('alias := base[1:]', snippet.call('alias-fix').strip)
  File.write(File.join(dir, 'inspect.go'), fixed_alias)
  run!(dir, 'go', 'run', 'inspect.go', output: "[1 2 3]\n")
  full.call('map-zero', "0 0\n0 true\n0 false\nfalse\n")
  full.call('nil-map', nil, /assignment to entry in nil map/)
  File.write(File.join(dir, 'inspect.go'), snippet.call('nil-map').sub('var counts map[string]int', 'counts := make(map[string]int)'))
  run!(dir, 'go', 'run', 'inspect.go', output: '')
  File.write(File.join(dir, 'inspect.go'), snippet.call('append').sub('t := append(s, 3)', "s[2] = 3\n\tt := append(s, 3)"))
  run!(dir, 'go', 'run', 'inspect.go', failure: /index out of range/)
  full.call('append', "[9 2] [9 2 3]\n2 3 3\n[9 2 3] [7 2 3 4]\n4 true\n")

  # 第 2 章练习 2.7：长度、容量、元素隔离和窗口边界。
  array_main = originals['arrays-slices']['main.go'].sub('fmt.Println("base:", values)', <<~GO.strip)
    fmt.Println(len(values), cap(values), len(extended), cap(extended), len(middle), cap(middle))
    extended[0] = 99
    fmt.Println("after copy:", values)
    middle[0] = 88
    fmt.Println("after window:", values)
    fmt.Println("base:", values)
  GO
  write_lab.call('arrays-slices', 'main.go', array_main)
  output = run!(dir, 'go', 'run', './arrays-slices')
  abort 'exercise 2.7 observation mismatch' unless output.start_with?("3 3 5 5 2 2\nafter copy: [1 2 3]\nafter window: [1 88 3]\n")
  array_tests = originals['arrays-slices']['main_test.go'] + <<~GO

    func TestBookIsolationAndBounds(t *testing.T) {
      base := []int{1, 2, 3}
      out := appendCopy(base, 4, 5)
      out[0] = 99
      if base[0] != 1 { t.Fatal("copy aliases input") }
      w := window(base, 1, 3)
      w[0] = 88
      if base[1] != 88 { t.Fatal("window should share") }
      for _, bounds := range [][2]int{{-1, 2}, {0, 4}, {3, 1}} {
        if window(base, bounds[0], bounds[1]) != nil { t.Fatal("invalid window") }
      }
      if len(window(base, 2, 2)) != 0 { t.Fatal("empty window") }
      if len(appendCopy(nil)) != 0 { t.Fatal("empty appendCopy") }
    }
  GO
  write_lab.call('arrays-slices', 'main_test.go', array_tests)
  run!(dir, 'go', 'test', './arrays-slices', '-count=1')

  # 练习 2.8：先验证新增用例能发现缺失的大小写归一化。
  map_tests = originals['maps']['main_test.go'] + <<~GO

    func TestBookNormalizedWords(t *testing.T) {
      got := countWords([]string{"Go", "go", "LEARN", ""})
      if !reflect.DeepEqual(got, map[string]int{"go": 2, "learn": 1}) { t.Fatalf("got %v", got) }
      if len(countWords(nil)) != 0 { t.Fatal("nil input") }
    }
  GO
  write_lab.call('maps', 'main_test.go', map_tests)
  run!(dir, 'go', 'test', './maps', '-count=1', failure: /FAIL/)
  map_main = originals['maps']['main.go'].sub('import "fmt"', "import (\n\"fmt\"\n\"strings\"\n)")
  map_main = map_main.sub('counts[word]++', "word = strings.ToLower(word)\n\t\tcounts[word]++")
  write_lab.call('maps', 'main.go', map_main)
  run!(dir, 'go', 'test', './maps', '-count=1')
  write_lab.call('maps', 'main.go', map_main.sub('words := []string{"go", "learn", "go", ""}', 'words := []string{"Go", "go", "LEARN", ""}'))
  run!(dir, 'go', 'run', './maps', output: "map[go:2 learn:1]\n2 true\n[Go go LEARN]\n")

  # 练习 2.9：维护者参考实现，验证题目中的合并、nil 和不修改输入约定。
  map_main += <<~GO

    func mergeCounts(a, b map[string]int) map[string]int {
      merged := make(map[string]int)
      for key, value := range a { merged[key] = value }
      for key, value := range b { merged[key] += value }
      return merged
    }
  GO
  map_tests += <<~GO

    func TestBookMergeCounts(t *testing.T) {
      a, b := map[string]int{"go": 2}, map[string]int{"go": 1, "learn": 1}
      out := mergeCounts(a, b)
      if !reflect.DeepEqual(out, map[string]int{"go": 3, "learn": 1}) { t.Fatalf("got %v", out) }
      out["go"] = 99
      if a["go"] != 2 || b["go"] != 1 { t.Fatal("inputs mutated") }
      empty := mergeCounts(nil, nil)
      if empty == nil || len(empty) != 0 { t.Fatal("must return writable empty map") }
      empty["go"] = 1
      if !reflect.DeepEqual(mergeCounts(a, nil), a) || !reflect.DeepEqual(mergeCounts(nil, b), b) { t.Fatal("one nil input") }
    }
  GO
  write_lab.call('maps', 'main.go', map_main)
  write_lab.call('maps', 'main_test.go', map_tests)
  run!(dir, 'go', 'test', './maps', '-count=1')
  puts 'Chapter 2: source parity, array/slice/map outputs, failures/fixes, three exercises passed'
  abort "unverified blocks: #{blocks.keys - used}" unless (blocks.keys - used).empty?
  run!(dir, 'go', 'test', './hello-world', './arrays-slices', './maps', '-race', '-count=1')
  puts "All #{blocks.length} marked Go blocks checked; exercise variants race tests passed; temporary copies removed on exit"
end
