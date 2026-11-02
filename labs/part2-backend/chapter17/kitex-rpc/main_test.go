package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter17/kitex-rpc/kitex_gen/api"
)

func TestGeneratedTypes(t *testing.T) {
	t.Parallel()

	echoReq := &api.EchoRequest{Message: "hello"}
	echoResp := &api.EchoResponse{Message: echoReq.GetMessage(), Length: 5}
	addReq := &api.AddRequest{First: 512, Second: 512}
	addResp := &api.AddResponse{Sum: addReq.GetFirst() + addReq.GetSecond()}

	if echoResp.GetMessage() != "hello" {
		t.Fatalf("echo response message = %q, want %q", echoResp.GetMessage(), "hello")
	}
	if echoResp.GetLength() != 5 {
		t.Fatalf("echo response length = %d, want 5", echoResp.GetLength())
	}
	if addResp.GetSum() != 1024 {
		t.Fatalf("add response sum = %d, want 1024", addResp.GetSum())
	}
}

func TestEcho(t *testing.T) {
	t.Parallel()

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name        string
		ctx         context.Context
		req         *api.EchoRequest
		wantMessage string
		wantLength  int32
		wantErr     error
	}{
		{
			name:        "trims ascii message and counts runes",
			ctx:         context.Background(),
			req:         &api.EchoRequest{Message: " hello kitex "},
			wantMessage: "hello kitex",
			wantLength:  11,
		},
		{
			name:        "counts unicode runes",
			ctx:         context.Background(),
			req:         &api.EchoRequest{Message: " 你好 kitex "},
			wantMessage: "你好 kitex",
			wantLength:  8,
		},
		{
			name:    "rejects nil request",
			ctx:     context.Background(),
			req:     nil,
			wantErr: ErrNilRequest,
		},
		{
			name:    "rejects empty message",
			ctx:     context.Background(),
			req:     &api.EchoRequest{Message: "  \t\n  "},
			wantErr: ErrEmptyMessage,
		},
		{
			name:    "preserves canceled context",
			ctx:     canceledCtx,
			req:     &api.EchoRequest{Message: "hello"},
			wantErr: context.Canceled,
		},
	}

	service := EchoServiceImpl{}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := service.Echo(tt.ctx, tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Echo() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Echo() error = %v, want nil", err)
			}
			if got.GetMessage() != tt.wantMessage {
				t.Fatalf("Echo() message = %q, want %q", got.GetMessage(), tt.wantMessage)
			}
			if got.GetLength() != tt.wantLength {
				t.Fatalf("Echo() length = %d, want %d", got.GetLength(), tt.wantLength)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	t.Parallel()

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		req     *api.AddRequest
		wantSum int64
		wantErr error
	}{
		{
			name:    "adds positive numbers",
			ctx:     context.Background(),
			req:     &api.AddRequest{First: 512, Second: 512},
			wantSum: 1024,
		},
		{
			name:    "adds negative numbers",
			ctx:     context.Background(),
			req:     &api.AddRequest{First: -10, Second: 3},
			wantSum: -7,
		},
		{
			name:    "adds zero values",
			ctx:     context.Background(),
			req:     &api.AddRequest{},
			wantSum: 0,
		},
		{
			name:    "rejects nil request",
			ctx:     context.Background(),
			req:     nil,
			wantErr: ErrNilRequest,
		},
		{
			name:    "preserves canceled context",
			ctx:     canceledCtx,
			req:     &api.AddRequest{First: 1, Second: 2},
			wantErr: context.Canceled,
		},
	}

	service := EchoServiceImpl{}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := service.Add(tt.ctx, tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Add() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Add() error = %v, want nil", err)
			}
			if got.GetSum() != tt.wantSum {
				t.Fatalf("Add() sum = %d, want %d", got.GetSum(), tt.wantSum)
			}
		})
	}
}

func TestDemo(t *testing.T) {
	t.Parallel()

	got := Demo()
	want := "Echo -> hello kitex (11), Add -> 1024"
	if got != want {
		t.Fatalf("Demo() = %q, want %q", got, want)
	}
}

func ExampleDemo() {
	fmt.Println(Demo())
	// Output:
	// Echo -> hello kitex (11), Add -> 1024
}
