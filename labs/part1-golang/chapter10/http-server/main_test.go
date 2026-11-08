package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewServerHealthz(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantBody    string
		wantContent string
	}{
		{
			name:        "get healthz",
			method:      http.MethodGet,
			path:        "/healthz",
			wantStatus:  http.StatusOK,
			wantBody:    `{"status":"ok"}` + "\n",
			wantContent: "application/json; charset=utf-8",
		},
		{
			name:        "head healthz",
			method:      http.MethodHead,
			path:        "/healthz",
			wantStatus:  http.StatusOK,
			wantBody:    "",
			wantContent: "application/json; charset=utf-8",
		},
		{
			name:       "post healthz",
			method:     http.MethodPost,
			path:       "/healthz",
			wantStatus: http.StatusMethodNotAllowed,
			wantBody:   "method not allowed\n",
		},
		{
			name:       "unknown route",
			method:     http.MethodGet,
			path:       "/missing",
			wantStatus: http.StatusNotFound,
			wantBody:   "404 page not found\n",
		},
	}

	handler := NewServer()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, tt.path, nil)
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

func TestValidateHealthMethod(t *testing.T) {
	t.Parallel()

	if err := validateHealthMethod(http.MethodGet); err != nil {
		t.Fatalf("validateHealthMethod(GET) error = %v, want nil", err)
	}
	if err := validateHealthMethod(http.MethodHead); err != nil {
		t.Fatalf("validateHealthMethod(HEAD) error = %v, want nil", err)
	}
	if err := validateHealthMethod(http.MethodPost); !errors.Is(err, ErrMethodNotAllowed) {
		t.Fatalf("validateHealthMethod(POST) error = %v, want ErrMethodNotAllowed", err)
	}
}

func TestDemoRequest(t *testing.T) {
	t.Parallel()

	got := DemoRequest(NewServer())
	want := `GET /healthz -> 200 {"status":"ok"}`
	if got != want {
		t.Fatalf("DemoRequest() = %q, want %q", got, want)
	}
}

func ExampleDemoRequest() {
	line := DemoRequest(NewServer())
	line = strings.TrimSpace(line)
	fmt.Println(line)
	// Output:
	// GET /healthz -> 200 {"status":"ok"}
}
