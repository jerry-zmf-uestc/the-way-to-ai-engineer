package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrUnknownTool      = errors.New("tool calling: unknown tool")
	ErrInvalidArguments = errors.New("tool calling: invalid arguments")
	ErrDuplicateTool    = errors.New("tool calling: tool already registered")
	ErrEmptyToolName    = errors.New("tool calling: tool name is empty")
	ErrNonFiniteResult  = errors.New("calculator: non-finite result")
)

// ArgType 是本示例支持的参数类型子集，对齐 JSON Schema 的基本标量。
type ArgType string

const (
	ArgString ArgType = "string"
	ArgNumber ArgType = "number"
)

// Schema 描述一个工具的参数形状：声明了哪些参数、各自类型、哪些必填。
// 未在 Properties 中声明的参数一律拒绝（相当于 additionalProperties: false）。
type Schema struct {
	Properties map[string]ArgType
	Required   []string
}

// Tool 是注册表中的最小工具单元：名字、描述、参数 schema 与真正干活的 handler。
// handler 只在参数通过 schema 校验之后才会被调用。
type Tool struct {
	Name        string
	Description string
	Parameters  Schema
	Handler     func(ctx context.Context, args map[string]any) (string, error)
}

// Registry 是本实验的工具清单与参数校验入口。先注册，运行期间只读；
// 不支持并发注册。模型提议的工具名和参数，
// 必须先通过这里的查找与校验，handler 才有机会运行。
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) error {
	if strings.TrimSpace(tool.Name) == "" {
		return ErrEmptyToolName
	}
	if !definitionFits(tool) {
		return ErrToolInputLimit
	}
	if tool.Handler == nil {
		return fmt.Errorf("tool calling: tool %q: handler is nil", tool.Name)
	}
	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateTool, tool.Name)
	}
	// 复制定义，注册以后调用方不能通过共享map悄悄改变参数契约。
	properties := make(map[string]ArgType, len(tool.Parameters.Properties))
	for name, kind := range tool.Parameters.Properties {
		if kind != ArgString && kind != ArgNumber {
			return fmt.Errorf("%w: unsupported type", ErrInvalidArguments)
		}
		properties[name] = kind
	}
	for _, name := range tool.Parameters.Required {
		if _, ok := properties[name]; !ok {
			return fmt.Errorf("%w: undeclared required argument", ErrInvalidArguments)
		}
	}
	tool.Parameters = Schema{Properties: properties, Required: append([]string(nil), tool.Parameters.Required...)}
	r.tools[tool.Name] = tool
	return nil
}

// Call 是唯一执行入口：未知工具直接拒绝，参数不符合 schema 不执行 handler。
func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := payloadFits(name, args); err != nil {
		return "", err
	}
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownTool, name)
	}
	if err := validateArgs(tool, args); err != nil {
		return "", err
	}
	if err := encodedArgsFit(args); err != nil {
		return "", err
	}
	result, err := tool.Handler(ctx, args)
	if stop := ctx.Err(); stop != nil {
		return "", stop
	}
	if err != nil {
		return "", err
	}
	if !resultFits(result) {
		return "", ErrToolResult
	}
	return result, nil
}

// Names 返回已注册工具名（排序保证输出稳定），供拼进 system prompt 展示给模型。
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validateArgs(tool Tool, args map[string]any) error {
	if args == nil {
		args = map[string]any{}
	}
	for _, required := range tool.Parameters.Required {
		if _, ok := args[required]; !ok {
			return fmt.Errorf("%w: tool %q: missing required argument %q", ErrInvalidArguments, tool.Name, required)
		}
	}
	for name, value := range args {
		want, declared := tool.Parameters.Properties[name]
		if !declared {
			return fmt.Errorf("%w: tool %q: unexpected argument %q", ErrInvalidArguments, tool.Name, name)
		}
		if !matchesType(value, want) {
			return fmt.Errorf("%w: tool %q: argument %q: expected %s, got %s",
				ErrInvalidArguments, tool.Name, name, want, typeName(value))
		}
	}
	return nil
}

func matchesType(value any, want ArgType) bool {
	switch want {
	case ArgString:
		_, ok := value.(string)
		return ok
	case ArgNumber:
		// JSON 解码出的数字是 float64；直接构造的调用方可能给 int，都接受。
		switch number := value.(type) {
		case float64:
			return !math.IsNaN(number) && !math.IsInf(number, 0)
		case int:
			return true
		}
		return false
	}
	return false
}

func typeName(value any) string {
	switch number := value.(type) {
	case string:
		return "string"
	case float64:
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return "non-finite number"
		}
		return "number"
	case int:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	}
	return fmt.Sprintf("%T", value)
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

func formatFinite(value float64) (string, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", ErrNonFiniteResult
	}
	return strconv.FormatFloat(value, 'g', -1, 64), nil
}

// calculatorTool 是确定性的四则运算 fake：不依赖网络和模型。
func calculatorTool() Tool {
	return Tool{
		Name:        "calculator",
		Description: "基本四则运算：add、subtract、multiply、divide",
		Parameters: Schema{
			Properties: map[string]ArgType{
				"op": ArgString,
				"a":  ArgNumber,
				"b":  ArgNumber,
			},
			Required: []string{"op", "a", "b"},
		},
		Handler: func(_ context.Context, args map[string]any) (string, error) {
			a := toFloat(args["a"])
			b := toFloat(args["b"])
			// schema 保证参数类型正确，但语义校验（除零、未知运算）仍是 handler 的责任。
			switch args["op"].(string) {
			case "add":
				return formatFinite(a + b)
			case "subtract":
				return formatFinite(a - b)
			case "multiply":
				return formatFinite(a * b)
			case "divide":
				if b == 0 {
					return "", errors.New("calculator: division by zero")
				}
				return formatFinite(a / b)
			}
			return "", fmt.Errorf("calculator: unknown op %q", args["op"])
		},
	}
}

// getTimeTool 通过注入时钟保持确定性：测试和 demo 传入固定时间。
func getTimeTool(now func() time.Time) Tool {
	return Tool{
		Name:        "get_time",
		Description: "返回当前 UTC 时间（RFC3339）",
		Parameters:  Schema{Properties: map[string]ArgType{}},
		Handler: func(_ context.Context, _ map[string]any) (string, error) {
			return now().UTC().Format(time.RFC3339), nil
		},
	}
}

func runDemo(ctx context.Context) (string, error) {
	registry := NewRegistry()
	fixed := func() time.Time { return time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC) }
	for _, tool := range []Tool{calculatorTool(), getTimeTool(fixed)} {
		if err := registry.Register(tool); err != nil {
			return "", err
		}
	}

	product, err := registry.Call(ctx, "calculator", map[string]any{"op": "multiply", "a": 6, "b": 7})
	if err != nil {
		return "", err
	}
	clock, err := registry.Call(ctx, "get_time", nil)
	if err != nil {
		return "", err
	}

	// 模型提议了一个注册表里没有的工具：必须被拒绝，而不是"想办法执行"。
	_, unknownErr := registry.Call(ctx, "delete_database", nil)
	if unknownErr == nil {
		return "", errors.New("tool calling: unknown tool was not rejected")
	}

	return fmt.Sprintf("registered=%v calculator=%s get_time=%s rejected=%q",
		registry.Names(), product, clock, unknownErr.Error()), nil
}

func main() {
	out, err := runDemo(context.Background())
	if err != nil {
		fmt.Println("run demo:", err)
		return
	}
	fmt.Println(out)
}
