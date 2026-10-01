# 第 1 章练习代码夹具

以下代码块是 book/part1-golang/chapter01.md 三道动手练的任务起点，供 scripts/verify-book-pilots.rb 提取验证；正文只保留表格描述。

<!-- verify:exercise-hello-1 -->
```go
// 任务：把欢迎语改为 hello, reader，函数签名不变。
// 预期结果：程序输出 hello, reader，测试通过。

// 文件：hello-world/main_test.go
func TestMessage(t *testing.T) {
	// TODO：先把下面两处预期文字改为 hello, reader，运行测试，观察失败。
	if got := message(); got != "hello, go" {
		t.Fatalf("message() = %q, want %q", got, "hello, go")
	}
}

// 文件：hello-world/main.go
func message() string {
	return "hello, go" // TODO：再修改返回值，重新运行程序和测试。
}
```

<!-- verify:exercise-hello-2 -->
```go
// 任务：接着第一题，让 message 接受名字，参考前面的 greet。
// 预期结果：程序输出 hello, gopher，原有测试仍通过。
// 空名字返回 "hello, "，保留末尾空格，暂不增加输入校验。

// 文件：hello-world/main.go
func message() string { // TODO：增加 name string 参数。
	return "hello, reader" // TODO：改为拼接 "hello, " 和 name。
}

func main() {
	fmt.Println(message()) // TODO：传入 "gopher"；漏改调用会报参数不足。
}

// 文件：hello-world/main_test.go
func TestMessage(t *testing.T) {
	if got := message(); got != "hello, reader" { // TODO：这里传入 "reader"。
		t.Fatalf("message() = %q, want %q", got, "hello, reader")
	}
}
```

<!-- verify:exercise-hello-3 -->
```go
// 任务：接着第二题，保留 TestMessage（"reader" → "hello, reader"），新增两个测试。
// 预期结果：三种输入的测试全部通过。
// 文件：hello-world/main_test.go（追加，不覆盖已有测试）

func TestMessageGopher(t *testing.T) {
	// TODO：用 if 比较 message("gopher") 与 "hello, gopher"，不相等时用 t.Fatalf 报错。
	t.Fatal("TODO: 补全 gopher 测试后替换这行")
}

func TestMessageEmpty(t *testing.T) {
	// TODO：检查 message("") 返回 "hello, "；用 %q 在失败信息中显示末尾空格。
	t.Fatal("TODO: 补全空名字测试后替换这行")
}

// 预期字符串直接写出，不另写拼接代码，避免测试与实现一起写错。
// 最后临时让 message 固定返回 "hello, gopher"：测试应失败；恢复后应通过。
```

