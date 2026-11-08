package chain

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRunner_ReverseShutdown(t *testing.T) {
	var calls []string
	r := NewRunner(nil)
	r.Add("product", &fakeServer{name: "product", calls: &calls})
	r.Add("order", &fakeServer{name: "order", calls: &calls})
	r.Add("user", &fakeServer{name: "user", calls: &calls})
	r.Add("gateway", &fakeServer{name: "gateway", calls: &calls})

	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	want := []string{"gateway", "user", "order", "product"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("shutdown order = %v, want %v", calls, want)
	}
}

func TestRunner_ShutdownJoinsErrors(t *testing.T) {
	var calls []string
	productErr := errors.New("product close failed")
	gatewayErr := errors.New("gateway close failed")
	r := NewRunner(nil)
	r.Add("product", &fakeServer{name: "product", calls: &calls, err: productErr})
	r.Add("gateway", &fakeServer{name: "gateway", calls: &calls, err: gatewayErr})

	err := r.Shutdown(context.Background())
	if !errors.Is(err, productErr) || !errors.Is(err, gatewayErr) {
		t.Fatalf("Shutdown() error = %v, want joined product and gateway errors", err)
	}
	if !strings.Contains(err.Error(), "product") || !strings.Contains(err.Error(), "gateway") {
		t.Fatalf("Shutdown() error = %q, want server names in context", err.Error())
	}
}

func TestRunner_AddIgnoresNilServer(t *testing.T) {
	r := NewRunner(nil)
	r.Add("nil", nil)
	if got := r.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
}

type fakeServer struct {
	name  string
	calls *[]string
	err   error
}

func (s *fakeServer) Addr() string { return s.name + "-addr" }

func (s *fakeServer) Shutdown(context.Context) error {
	*s.calls = append(*s.calls, s.name)
	return s.err
}
