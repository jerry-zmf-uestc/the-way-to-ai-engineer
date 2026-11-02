package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestServerProcess(t *testing.T) {
	if os.Getenv("MCP_SERVER_PROCESS") != "1" {
		t.Skip("local helper")
	}
	profile := os.Getenv("MCP_SERVER_PROFILE")
	if profile == "" {
		profile = "primary"
	}
	if err := serveProfile(os.Stdin, os.Stdout, profile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
func process(t *testing.T, approved map[string]bool) *Client {
	return processForServer(t, "docs", "primary", approved)
}

func processForServer(t *testing.T, alias, profile string, approved map[string]bool) *Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestServerProcess$")
	cmd.Env = append(os.Environ(), "MCP_SERVER_PROCESS=1", "MCP_SERVER_PROFILE="+profile)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(input, output, alias, approved)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.Close()
		if err := cmd.Wait(); err != nil {
			t.Error(err)
		}
		cancel()
	})
	return c
}
func TestActualStdioHandshakeDiscoveryCallAndToolFailure(t *testing.T) {
	c := process(t, map[string]bool{"read_doc": true})
	ctx := context.Background()
	if _, err := c.Discover(ctx); !errors.Is(err, ErrProtocol) {
		t.Fatal("discovery before initialization")
	}
	if err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	names, err := c.Discover(ctx)
	if err != nil || len(names) != 1 || names[0] != "docs::read_doc" {
		t.Fatalf("names=%v err=%v", names, err)
	}
	text, err := c.Call(ctx, names[0], map[string]any{"id": "doc-A"})
	if err != nil || text != "Timeout is 30 seconds." {
		t.Fatalf("text=%q err=%v", text, err)
	}
	if _, err := c.Call(ctx, names[0], map[string]any{"id": "missing"}); !errors.Is(err, ErrTool) {
		t.Fatal("tool isError became success")
	}
	if _, err := c.Call(ctx, names[0], map[string]any{"id": 1}); !errors.Is(err, ErrProtocol) {
		t.Fatal("bad args sent")
	}
}
func TestDiscoveryDoesNotAuthorize(t *testing.T) {
	c := process(t, map[string]bool{})
	ctx := context.Background()
	if err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	names, err := c.Discover(ctx)
	if err != nil || len(names) != 0 {
		t.Fatal("unapproved tool exposed")
	}
	before := c.seq
	if _, err := c.Call(ctx, "docs::read_doc", map[string]any{"id": "doc-A"}); !errors.Is(err, ErrDenied) || c.seq != before {
		t.Fatal("discovered capability widened permission")
	}
}
func TestCancellationClosesBlockingSession(t *testing.T) {
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	defer serverInput.Close()
	defer serverOutput.Close()
	c, err := NewClient(clientInput, clientOutput, "docs", nil)
	if err != nil {
		t.Fatal(err)
	}
	readDone := make(chan struct{})
	go func() { buffer := make([]byte, 4096); serverInput.Read(buffer); close(readDone) }()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := c.exchange(ctx, "initialize", map[string]any{}); done <- err }()
	<-readDone
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !c.closed {
			t.Fatal("cancellation lost")
		}
	case <-time.After(time.Second):
		t.Fatal("stdio read remained blocked")
	}
}
func TestWrongResponseIDFailsClosed(t *testing.T) {
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	defer serverInput.Close()
	defer serverOutput.Close()
	c, _ := NewClient(clientInput, clientOutput, "docs", nil)
	defer c.Close()
	go func() {
		buffer := make([]byte, 4096)
		serverInput.Read(buffer)
		fmt.Fprintln(serverOutput, `{"jsonrpc":"2.0","id":99,"result":{}}`)
	}()
	if _, err := c.exchange(context.Background(), "initialize", map[string]any{}); !errors.Is(err, ErrProtocol) || !c.closed {
		t.Fatal("wrong ID accepted")
	}
}

func TestCancellationAlsoInterruptsBlockedWrite(t *testing.T) {
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	defer serverInput.Close()
	defer serverOutput.Close()
	c, _ := NewClient(clientInput, clientOutput, "docs", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := c.exchange(ctx, "initialize", map[string]any{}); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked write ignored timeout")
	}
}

func TestInitializedNotificationHasWriteDeadline(t *testing.T) {
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	defer serverInput.Close()
	defer serverOutput.Close()
	c, _ := NewClient(clientInput, clientOutput, "docs", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := c.notify(ctx, message{JSONRPC: "2.0", Method: "notifications/initialized"}); !errors.Is(err, context.DeadlineExceeded) || !c.closed {
		t.Fatal("notification write ignored deadline")
	}
}
