package summary

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/config"
)

func TestFetchSuccess(t *testing.T) {
	t.Parallel()

	var gotTrace string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTrace = r.Header.Get("X-Trace-ID")
		_, _ = io.WriteString(w, `{"status":200,"orders":2,"total_cents":2497,"trace":"Frontend→Hertz→UserSvc→OrderSvc→ProductSvc","attempts":1}`)
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	result, err := Fetch(context.Background(), config.Default(), addr)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Status != http.StatusOK || result.Orders != 2 || result.TotalCents != 2497 || result.Attempts != 1 {
		t.Fatalf("result = %+v", result)
	}
	if gotTrace != DemoTrace {
		t.Fatalf("trace header = %q, want %q", gotTrace, DemoTrace)
	}
}
