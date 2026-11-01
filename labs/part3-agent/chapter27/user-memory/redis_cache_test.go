package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func redisCacheKey(scope Scope, key string) string {
	return "memory:v1:" + base64.RawURLEncoding.EncodeToString([]byte(scope.Tenant)) + ":" + base64.RawURLEncoding.EncodeToString([]byte(scope.User)) + ":" + base64.RawURLEncoding.EncodeToString([]byte(key))
}

func isolatedRedis(t *testing.T) (func(context.Context, ...string) (string, error), func()) {
	t.Helper()
	server, cli := os.Getenv("MEMORY_REDIS_SERVER"), os.Getenv("MEMORY_REDIS_CLI")
	if !filepath.IsAbs(server) || !filepath.IsAbs(cli) {
		t.Fatal("trusted absolute Redis binary paths required")
	}
	for _, binary := range []string{server, cli} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		output, err := exec.CommandContext(ctx, binary, "--version").Output()
		cancel()
		fields := strings.Fields(string(output))
		matched := (binary == server && len(fields) >= 3 && fields[2] == "v=7.2.11") || (binary == cli && len(fields) == 2 && fields[1] == "7.2.11")
		if err != nil || !matched {
			t.Fatal("prepared Redis version mismatch")
		}
	}
	root, err := os.MkdirTemp("/tmp", "memory-redis-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	socket := filepath.Join(root, "cache.sock")
	log, err := os.OpenFile(filepath.Join(root, "redis.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	cmd := exec.Command(server, "--port", "0", "--unixsocket", socket, "--unixsocketperm", "600", "--save", "", "--appendonly", "no", "--daemonize", "no", "--maxmemory", "16mb", "--maxmemory-policy", "noeviction", "--maxclients", "8")
	cmd.Dir, cmd.Stdout, cmd.Stderr = root, log, log
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	if err := cmd.Start(); err != nil {
		t.Fatal("private Redis startup failed", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cmd.Process.Signal(os.Interrupt)
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("Redis exit: %v", err)
				}
			case <-time.After(3 * time.Second):
				cmd.Process.Kill()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("Redis cleanup wait timed out")
				}
			}
		})
	}
	t.Cleanup(stop)
	command := func(ctx context.Context, args ...string) (string, error) {
		child, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		request := exec.CommandContext(child, cli, append([]string{"-s", socket, "--raw"}, args...)...)
		request.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
		output, err := request.Output()
		if child.Err() != nil {
			return "", child.Err()
		}
		if err != nil || len(output) > 4096 {
			return "", errors.New("private Redis command failed")
		}
		return strings.TrimSuffix(string(output), "\n"), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if response, err := command(ctx, "PING"); err == nil && response == "PONG" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("private Redis not ready")
		case <-ticker.C:
		}
	}
	return command, stop
}

func TestActualRedisStaleCacheRevalidation(t *testing.T) {
	if os.Getenv("MEMORY_REDIS_E2E") != "1" {
		t.Skip("explicit prepared local Redis only")
	}
	redisRevalidationContract(t, NewFake(), "Fake")
}

func TestActualRedisPostgresRevalidation(t *testing.T) {
	if os.Getenv("MEMORY_REDIS_E2E") != "1" || os.Getenv("MEMORY_E2E") != "1" {
		t.Skip("explicit Redis and isolated PostgreSQL only")
	}
	if os.Getenv("MEMORY_TEST_DATABASE") != "1" {
		t.Fatal("fixture database confirmation required")
	}
	db, err := sql.Open("pgx", os.Getenv("MEMORY_DSN"))
	if err != nil {
		t.Fatal("database configuration failed")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var name string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); err != nil || name != "memory_lab" {
		t.Fatal("isolated memory_lab required")
	}
	store := SQLStore{db}
	if err := store.Init(ctx); err != nil {
		t.Fatal("fixture schema setup failed")
	}
	redisRevalidationContract(t, store, "PostgreSQL")
}

func redisRevalidationContract(t *testing.T, backend Backend, backendName string) {
	t.Helper()
	command, stop := isolatedRedis(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	scope := Scope{"redis-" + t.Name(), "reader"}
	memory := Memory{backend, func() time.Time { return now }}
	consent := Consent{Scope: scope, Key: "response-style", Operation: "write", Value: "concise", Allowed: true, ExpiresAt: now.Add(time.Minute)}
	first, err := memory.Write(ctx, scope, consent.Key, "concise", "confirmation-1", 0, time.Hour, consent)
	if err != nil {
		t.Fatal(err)
	}
	lateRead, err := memory.Store.Peek(ctx, scope, consent.Key)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(first)
	key := redisCacheKey(scope, consent.Key)
	if result, err := command(ctx, "SET", key, string(payload), "PX", "300000"); err != nil || result != "OK" {
		t.Fatal("cache fill failed", err)
	}
	consent.Value = "detailed"
	current, err := memory.Write(ctx, scope, consent.Key, "detailed", "confirmation-2", first.Revision, time.Hour, consent)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := command(ctx, "DEL", key); err != nil || result != "1" {
		t.Fatal("cache invalidation failed", err)
	}
	payload, _ = json.Marshal(lateRead)
	if result, err := command(ctx, "SET", key, string(payload), "PX", "300000"); err != nil || result != "OK" {
		t.Fatal("late fill failed", err)
	}
	raw, err := command(ctx, "GET", key)
	var cached Record
	if err != nil || json.Unmarshal([]byte(raw), &cached) != nil || cached.Value != "concise" || cached.Revision != 1 {
		t.Fatal("late fill was not actually present in Redis", err)
	}
	remaining, err := command(ctx, "PTTL", key)
	ttl, parseErr := strconv.Atoi(remaining)
	if err != nil || parseErr != nil || ttl <= 0 || ttl > 300000 {
		t.Fatal("cache TTL not valid", err, parseErr)
	}
	resolved, err := memory.Resolve(ctx, scope, []string{cached.Key})
	if err != nil || len(resolved) != 1 || resolved[0].Key != current.Key || resolved[0].Value != current.Value || resolved[0].Source != current.Source || resolved[0].Revision != current.Revision || !resolved[0].ExpiresAt.Equal(current.ExpiresAt) || resolved[0].Deleted {
		t.Fatal("stale Redis content reached input", resolved, err)
	}
	input, err := memory.Input(ctx, scope, []string{cached.Key})
	if err != nil || len(input) != 1 || input[0].Value != "detailed" || input[0].Revision != current.Revision {
		t.Fatal("current source did not reach selected input", input, err)
	}
	for _, other := range []Scope{{scope.Tenant, "other"}, {"other", scope.User}} {
		if raw, err := command(ctx, "GET", redisCacheKey(other, cached.Key)); err != nil || raw != "" {
			t.Fatal("cache namespace leak", err)
		}
		resolved, err := memory.Resolve(ctx, other, []string{cached.Key})
		if err != nil || len(resolved) != 0 {
			t.Fatal("candidate bypassed source scope", resolved, err)
		}
	}
	consent.Operation = "delete"
	if _, err := memory.Delete(ctx, scope, consent.Key, current.Revision, consent); err != nil {
		t.Fatal(err)
	}
	if raw, err := command(ctx, "GET", key); err != nil || raw == "" {
		t.Fatal("test no longer holds a stale cache", err)
	}
	if rows, err := memory.Resolve(ctx, scope, []string{cached.Key}); err != nil || len(rows) != 0 {
		t.Fatal("Redis revived deleted memory", rows, err)
	}
	if input, err := memory.Input(ctx, scope, []string{cached.Key}); err != nil || len(input) != 0 {
		t.Fatal("deleted source reached selected input", input, err)
	}
	consent.Key, consent.Value, consent.Operation = "locale", "en", "write"
	if _, err := memory.Write(ctx, scope, "locale", "en", "confirmed-locale", 0, time.Hour, consent); err != nil {
		t.Fatal(err)
	}
	stop()
	if _, err := command(ctx, "GET", key); err == nil {
		t.Fatal("stopped cache still served data")
	}
	// Explicit application fallback, not interpreting a cache error as no memory.
	if rows, err := memory.Recall(ctx, scope); err != nil || len(rows) != 1 || rows[0].Key != "locale" {
		t.Fatal("source fallback failed", rows, err)
	}
	t.Logf("redis=7.2.11 transport=private-unix primary=%s stale-revision=1 current-revision=2 TTL-positive=true delete-not-revived=true scoped=true cache-down-fallback=true", backendName)
}

func TestRedisScopeEncodingHasNoDelimiterCollision(t *testing.T) {
	if redisCacheKey(Scope{"a", "b"}, "locale") == redisCacheKey(Scope{"a", "b"}, "response-style") {
		t.Fatal("memory fields collided")
	}
	if redisCacheKey(Scope{"a:b", "c"}, "response-style") == redisCacheKey(Scope{"a", "b:c"}, "response-style") {
		t.Fatal("scope components collided")
	}
}
