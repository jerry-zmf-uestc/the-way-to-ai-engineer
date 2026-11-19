package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEchoAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		wantStatus  int
		wantBody    string
		wantContent string
	}{
		{
			name:        "valid echo request",
			method:      http.MethodPost,
			path:        "/api/echo",
			body:        `{"message":"hello, json api"}`,
			wantStatus:  http.StatusOK,
			wantBody:    `{"echo":"hello, json api","length":15}` + "\n",
			wantContent: "application/json; charset=utf-8",
		},
		{
			name:       "reject get",
			method:     http.MethodGet,
			path:       "/api/echo",
			wantStatus: http.StatusMethodNotAllowed,
			wantBody:   "method not allowed\n",
		},
		{
			name:       "reject malformed json",
			method:     http.MethodPost,
			path:       "/api/echo",
			body:       `{"message":`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid json\n",
		},
		{
			name:       "reject empty message",
			method:     http.MethodPost,
			path:       "/api/echo",
			body:       `{"message":"   "}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "message is required\n",
		},
		{
			name:       "unknown route",
			method:     http.MethodPost,
			path:       "/api/missing",
			body:       `{"message":"hello"}`,
			wantStatus: http.StatusNotFound,
			wantBody:   "404 page not found\n",
		},
	}

	handler := NewServer()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if rec.Body.String() != tt.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantContent != "" && rec.Header().Get("Content-Type") != tt.wantContent {
				t.Fatalf("content-type = %q, want %q", rec.Header().Get("Content-Type"), tt.wantContent)
			}
		})
	}
}

func TestValidateEchoMessage(t *testing.T) {
	t.Parallel()

	if err := validateEchoMessage("hello"); err != nil {
		t.Fatalf("validateEchoMessage(%q) error = %v, want nil", "hello", err)
	}
	if err := validateEchoMessage("   "); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("validateEchoMessage(empty) error = %v, want ErrEmptyMessage", err)
	}
}

func TestDemoRequest(t *testing.T) {
	t.Parallel()

	got := DemoRequest(NewServer())
	want := `POST /api/echo -> 200 {"echo":"hello, json api","length":15}`
	if got != want {
		t.Fatalf("DemoRequest() = %q, want %q", got, want)
	}
}

func ExampleDemoRequest() {
	line := DemoRequest(NewServer())
	line = strings.TrimSpace(line)
	fmt.Println(line)
	// Output:
	// POST /api/echo -> 200 {"echo":"hello, json api","length":15}
}
