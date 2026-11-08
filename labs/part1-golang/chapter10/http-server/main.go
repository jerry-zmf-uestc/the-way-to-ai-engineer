package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

// ErrMethodNotAllowed reports an HTTP method that /healthz does not accept.
var ErrMethodNotAllowed = errors.New("method not allowed")

// HealthResponse is the stable JSON payload returned by /healthz.
type HealthResponse struct {
	Status string `json:"status"`
}

// NewServer builds the minimal HTTP handler for this topic.
func NewServer() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", HealthHandler)
	return mux
}

// HealthHandler serves the stateless health check endpoint.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if err := validateHealthMethod(r.Method); err != nil {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	if err := json.NewEncoder(w).Encode(HealthResponse{Status: "ok"}); err != nil {
		http.Error(w, fmt.Errorf("write health response: %w", err).Error(), http.StatusInternalServerError)
	}
}

func validateHealthMethod(method string) error {
	if method == http.MethodGet || method == http.MethodHead {
		return nil
	}

	return fmt.Errorf("%w: %s", ErrMethodNotAllowed, method)
}

// DemoRequest spins up a loopback httptest.Server so the demo exercises the
// real net/http client + server path instead of the in-memory ResponseRecorder.
func DemoRequest(handler http.Handler) string {
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		return fmt.Sprintf("ERROR: get healthz: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("ERROR: read healthz body: %v", err)
	}

	body := strings.TrimSpace(string(raw))
	return fmt.Sprintf("%s %s -> %d %s", http.MethodGet, "/healthz", resp.StatusCode, body)
}

// DemoRequestListenAndServe 是 DemoRequest 的 "方案 B" 对照实现：使用生产环境更常见的
// http.Server + net.Listener 组合起一个真实监听器，再由 http.Client 主动打请求，
// 最后通过 Shutdown 做优雅关闭。它与 DemoRequest 输出格式保持一致，便于直接对比。
//
// 相较于方案 A (httptest.NewServer)，方案 B 更贴近生产：
//  1. 需要手动管理 net.Listener 生命周期；这里用 "127.0.0.1:0" 让内核分配随机端口，
//     既避免端口冲突，又保证 Listen 返回时端口就已经就绪，因此不需要 time.Sleep 等待。
//  2. 服务端必须放到独立 goroutine 里跑，否则 Serve 会阻塞主流程。
//  3. Serve 正常退出会返回 http.ErrServerClosed，需要用 errors.Is 显式判断，
//     而不是把它当作错误上报。
//  4. 客户端必须显式设置 Timeout；http.DefaultClient 没有超时，是常见的生产坑。
//  5. 关闭顺序：先 srv.Shutdown(ctx) 让在途请求跑完，再让 goroutine 自然退出，
//     不能直接 listener.Close，否则可能丢弃仍在处理的连接。
func DemoRequestListenAndServe(handler http.Handler) string {
	// 1. 起监听器。":0" 让内核选一个可用端口，避免固定端口在自动化场景下打架。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Sprintf("ERROR: listen: %v", err)
	}
	// listener 的所有权会转交给 srv.Serve，正常路径由 Shutdown 释放；只有在下面提前
	// 出错时才需要走 defer 兜底关闭。
	addr := listener.Addr().String()

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second, // 防止 slow-header 之类的低级 DoS
	}

	// 2. 独立 goroutine 跑 server；用带缓冲的 channel 把 Serve 的返回值送回来，
	//    这样主流程能在关闭阶段判断是否有异常退出。
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(listener)
	}()

	// 3. 客户端设置超时，避免请求卡死。
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		_ = srv.Close() // 出错兜底：强关连接，忽略 goroutine 侧错误
		return fmt.Sprintf("ERROR: get healthz: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		_ = srv.Close()
		return fmt.Sprintf("ERROR: read healthz body: %v", err)
	}
	body := strings.TrimSpace(string(raw))
	line := fmt.Sprintf("%s %s -> %d %s", http.MethodGet, "/healthz", resp.StatusCode, body)

	// 4. 优雅关闭。给 Shutdown 一个上下文超时，避免关闭阶段本身卡死。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Sprintf("ERROR: shutdown: %v", err)
	}

	// 5. 等 Serve goroutine 退出。http.ErrServerClosed 是 Shutdown 触发的正常信号，
	//    不算错误；其他任何错误都要如实上报。
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Sprintf("ERROR: serve: %v", err)
	}
	return line
}

func main() {
	handler := NewServer()

	// 方案 A：httptest.NewServer + http.Get。适合 demo / 单测，起停由 httptest 托管。
	fmt.Println("[A httptest]        " + DemoRequest(handler))

	// 方案 B：http.Server + net.Listener + http.Client + Shutdown。贴近生产：手动管
	// 生命周期、显式超时、优雅关闭。两种输出应完全一致。
	fmt.Println("[B ListenAndServe]  " + DemoRequestListenAndServe(handler))
}
