package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func discoverServer(t *testing.T, c *Client) {
	t.Helper()
	if err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestActualTwoServerRoutingKeepsDestinationAndSession(t *testing.T) {
	primary := processForServer(t, "primary", "primary", map[string]bool{"read_doc": true})
	archive := processForServer(t, "archive", "archive", map[string]bool{"read_doc": true})
	discoverServer(t, primary)
	discoverServer(t, archive)
	router, err := NewToolRouter(primary, archive)
	if err != nil {
		t.Fatal(err)
	}
	names := router.Names()
	if !reflect.DeepEqual(names, []string{"archive::read_doc", "primary::read_doc"}) {
		t.Fatal(names)
	}
	names[0] = "untrusted::read_doc"
	if router.Names()[0] != "archive::read_doc" {
		t.Fatal("caller changed routing snapshot")
	}
	ctx := context.Background()
	before := archive.seq
	text, err := router.Call(ctx, "primary::read_doc", map[string]any{"id": "doc-A"})
	if err != nil || text != "Timeout is 30 seconds." || archive.seq != before {
		t.Fatal(text, err, "wrong destination")
	}
	before = primary.seq
	text, err = router.Call(ctx, "archive::read_doc", map[string]any{"id": "doc-A"})
	if err != nil || text != "Archive timeout is 45 seconds." || primary.seq != before {
		t.Fatal(text, err, "wrong destination")
	}
	if _, err := NewToolRouter(primary, primary); !errors.Is(err, ErrProtocol) {
		t.Fatal("duplicate alias accepted")
	}
	if _, err := router.Call(ctx, "archive::read_doc", map[string]any{"id": "missing"}); !errors.Is(err, ErrTool) || primary.seq != before {
		t.Fatal("tool failure fell back to other server", err)
	}
	archive.input.Close()
	if _, err := router.Call(ctx, "archive::read_doc", map[string]any{"id": "doc-A"}); err == nil || !archive.closed || primary.seq != before {
		t.Fatal("transport failure silently rerouted", err)
	}
	if _, err := router.Call(ctx, "archive::read_doc", map[string]any{"id": "doc-A"}); !errors.Is(err, ErrDenied) || primary.seq != before {
		t.Fatal("closed session silently rerouted", err)
	}
	if text, err := router.Call(ctx, "primary::read_doc", map[string]any{"id": "doc-A"}); err != nil || text != "Timeout is 30 seconds." {
		t.Fatal("unrelated session lost", err)
	}
}

func TestActualTwoServerDiscoveryDoesNotSharePermission(t *testing.T) {
	primary := processForServer(t, "primary", "primary", map[string]bool{"read_doc": true})
	archive := processForServer(t, "archive", "archive", nil)
	discoverServer(t, primary)
	discoverServer(t, archive)
	router, err := NewToolRouter(primary, archive)
	if err != nil || !reflect.DeepEqual(router.Names(), []string{"primary::read_doc"}) {
		t.Fatal(err, "unapproved capability published")
	}
	a, b := primary.seq, archive.seq
	for _, name := range []string{"read_doc", "archive::read_doc", "other::read_doc", "primary::read_doc::extra"} {
		if _, err := router.Call(context.Background(), name, map[string]any{"id": "doc-A"}); !errors.Is(err, ErrDenied) || primary.seq != a || archive.seq != b {
			t.Fatal("ambiguous or denied route sent RPC", name, err)
		}
	}
}
