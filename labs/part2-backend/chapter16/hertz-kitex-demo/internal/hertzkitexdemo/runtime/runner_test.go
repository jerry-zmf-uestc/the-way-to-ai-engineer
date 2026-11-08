package runtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
)

type fakeServer struct {
	name  string
	calls *[]string
	err   error
}

func (f fakeServer) Addr() string { return f.name }

func (f fakeServer) Shutdown(context.Context) error {
	*f.calls = append(*f.calls, f.name)
	return f.err
}

func TestRunnerShutdownReverseOrder(t *testing.T) {
	t.Parallel()

	var calls []string
	runner := NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)))
	runner.Add("product", fakeServer{name: "product", calls: &calls})
	runner.Add("order", fakeServer{name: "order", calls: &calls})
	runner.Add("user", fakeServer{name: "user", calls: &calls})

	if err := runner.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	want := []string{"user", "order", "product"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("shutdown calls = %v, want %v", calls, want)
	}
}

func TestRunnerShutdownJoinsErrors(t *testing.T) {
	t.Parallel()

	var calls []string
	errBoom := errors.New("boom")
	runner := NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)))
	runner.Add("gateway", fakeServer{name: "gateway", calls: &calls, err: errBoom})

	err := runner.Shutdown(context.Background())
	if err == nil {
		t.Fatal("Shutdown() error = nil, want joined error")
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("Shutdown() error = %v, want wrapped boom", err)
	}
}
