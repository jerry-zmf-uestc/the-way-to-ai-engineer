package obs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
)

// ctxKey 是 trace-id 在 context.Context 里的唯一 key；用私有类型防止外部
// 通过 string 撞 key。
type ctxKey struct{}

// NewLogger 用给定 io.Writer 与 level 构造 slog.JSONHandler。
// 生产环境传 os.Stderr；单测传 *bytes.Buffer 便于断言字段。
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(h)
}

// WithTraceID 把 trace-id 存入 context，供下游 handler / kitex client middleware 读取。
// 空字符串按未设置处理，保持 TraceID 返回 ("", false)。
func WithTraceID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// TraceID 从 context 读取 trace-id。第二返回值区分 "未设置" 与 "空值"。
func TraceID(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(ctxKey{}).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// NewTraceID 生成 16-byte（32 hex 字符）随机 trace-id；不依赖 uuid，
// 只走 crypto/rand → hex.EncodeToString。
func NewTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read 在正常运行时不会失败；出错说明系统熵源出问题，
		// bounded demo 里直接 panic 是合理选择。
		panic("obs: read random bytes: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
