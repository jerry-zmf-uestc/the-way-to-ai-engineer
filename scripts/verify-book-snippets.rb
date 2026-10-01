#!/usr/bin/env ruby
require 'tmpdir'
require 'open3'
require 'fileutils'
root = File.expand_path('..', __dir__)
Dir.chdir(root)
def run_case(name, code, test, should_pass: true, error: nil)
  Dir.mktmpdir('book-snippets-') do |dir|
    File.write(File.join(dir, 'go.mod'), "module bookcheck\n\ngo 1.26\n")
    File.write(File.join(dir, 'main.go'), code)
    File.write(File.join(dir, 'main_test.go'), test)
    out, err, status = Open3.capture3(
      {'GOTOOLCHAIN'=>'local', 'GOPROXY'=>'off', 'GOSUMDB'=>'off', 'GOWORK'=>'off'},
      'go', 'test', './...', '-count=1', '-timeout=30s', chdir: dir)
    valid = should_pass ? status.success? : (!status.success? && (out+err).include?(error))
    abort "#{name}: unexpected result\n#{out}#{err}" unless valid
    puts "PASS: #{name}"
  end
end
control = File.read('book/part1-golang/chapter03.md')
good = control.scan(/```go\n(.*?)```/m).flatten.first(2).join("\n")
bad = control.split('## 踩坑清单',2).last[/```go\n(.*?)```/m,1]
control_test = <<~'GO'
package main
import "testing"
func TestBoundaries(t *testing.T) {
  tests := []struct{ score int; want string }{{-1,"invalid"},{0,"retry"},{59,"retry"},{60,"pass"},{89,"pass"},{90,"excellent"},{99,"excellent"},{100,"excellent"},{101,"invalid"}}
  for _,tt := range tests {
    if got := labelScore(tt.score); got != tt.want { t.Fatalf("score %d: got %s want %s",tt.score,got,tt.want) }
  }
  for n,want := range map[int]int{-1:0,0:0,1:0,10:30} { if got:=sumEven(n);got!=want {t.Fatalf("sumEven(%d)=%d",n,got)} }
}
GO
run_case('control-flow corrected boundaries', "package main\n"+good, control_test)
broken_control = "package main\n"+bad+"\n"+good.split('func sumEven',2).last.prepend('func sumEven')
run_case('late range check fails at 101', broken_control, control_test, should_pass: false, error: 'score 101')
