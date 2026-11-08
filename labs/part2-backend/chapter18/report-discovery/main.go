package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"time"
)

type observation struct {
	target, served    string
	success, failures int
}

func instance(name string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, name)
	}))
}

func lookup(ctx context.Context, client *http.Client, target, address string) (observation, error) {
	m := observation{target: target}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address+"/exports/export-1", nil)
	if err != nil {
		return m, err
	}
	resp, err := client.Do(req)
	if err != nil {
		m.failures++
		return m, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16))
	if err != nil {
		return m, err
	}
	if resp.StatusCode != http.StatusOK {
		return m, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	m.served, m.success = string(body), 1
	return m, nil
}

func exercise() ([]observation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a, b := instance("A"), instance("B")
	defer a.Close()
	defer b.Close()

	// shortcut: an explicit in-process snapshot stands in for a registry watch.
	ready := map[string]string{"A": a.URL, "B": b.URL}
	pinned := ready["A"]
	before, err := lookup(ctx, a.Client(), "A", pinned)
	if err != nil {
		return nil, err
	}
	delete(ready, "A")
	a.Close()

	stale, err := lookup(ctx, b.Client(), "A", pinned)
	if err != nil {
		return nil, err
	}
	// Refresh the destination for the next request.
	for name, address := range ready {
		fresh, err := lookup(ctx, b.Client(), name, address)
		return []observation{before, stale, fresh}, err
	}
	return nil, fmt.Errorf("no ready instances")
}

func main() {
	results, err := exercise()
	if err != nil {
		log.Fatal(err)
	}
	for i, m := range results {
		fmt.Printf("phase=%s target=%s served=%q success=%d failures=%d\n",
			[]string{"before", "pinned", "refreshed"}[i], m.target, m.served, m.success, m.failures)
	}
}
