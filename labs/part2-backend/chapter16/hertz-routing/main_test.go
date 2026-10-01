package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRoutes(t *testing.T) {
	router := newRouter().Engine

	tests := []struct {
		name       string
		method     string
		path       string
		headers    []ut.Header
		wantStatus int
		wantBody   string
		wantReqID  string
		wantHeader string
	}{
		{
			name:       "health inherits request id",
			method:     "GET",
			path:       "/healthz",
			headers:    []ut.Header{{Key: "X-Request-ID", Value: "req-123"}},
			wantStatus: consts.StatusOK,
			wantBody:   `"status":"ok"`,
			wantReqID:  `"request_id":"req-123"`,
			wantHeader: "req-123",
		},
		{
			name:       "path parameter",
			method:     "GET",
			path:       "/api/books/bk-42",
			wantStatus: consts.StatusOK,
			wantBody:   `"id":"bk-42"`,
			wantReqID:  `"request_id":"generated-demo-id"`,
			wantHeader: "generated-demo-id",
		},
		{
			name:       "query parameter",
			method:     "GET",
			path:       "/api/search?q=hertz",
			wantStatus: consts.StatusOK,
			wantBody:   `"query":"hertz"`,
			wantReqID:  `"request_id":"generated-demo-id"`,
			wantHeader: "generated-demo-id",
		},
		{
			name:       "missing query",
			method:     "GET",
			path:       "/api/search",
			wantStatus: consts.StatusBadRequest,
			wantBody:   `"error":"q is required"`,
			wantReqID:  "",
			wantHeader: "generated-demo-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := ut.PerformRequest(router, tt.method, tt.path, nil, tt.headers...)
			body := string(resp.Body.Bytes())

			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.wantStatus, body)
			}
			if !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body = %s, want contains %s", body, tt.wantBody)
			}
			if tt.wantReqID != "" && !strings.Contains(body, tt.wantReqID) {
				t.Fatalf("body = %s, want contains %s", body, tt.wantReqID)
			}
			if got := resp.Header().Get("X-Request-ID"); got != tt.wantHeader {
				t.Fatalf("X-Request-ID = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}

func TestRunOutput(t *testing.T) {
	got := captureStdout(t, func() { run(false) })
	want := "hertz routes ready: GET /healthz, GET /api/books/:id, GET /api/search?q=go\n"
	if got != want {
		t.Fatalf("run() output = %q, want %q", got, want)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
