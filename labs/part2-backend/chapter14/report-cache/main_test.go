package main

import (
	"os"
	"testing"
)

func checkCache(t *testing.T, newCache func() cache) {
	t.Helper()
	for _, tc := range []struct {
		fixed bool
		want  observation
	}{
		{false, observation{"v1", 1, 1, 1}},
		{true, observation{"v2", 0, 2, 0}},
	} {
		got, err := exercise(newCache(), tc.fixed)
		if err != nil || got != tc.want {
			t.Fatalf("fixed=%t: got %+v, %v; want %+v", tc.fixed, got, err, tc.want)
		}
	}
}

func TestStaleCache(t *testing.T) {
	checkCache(t, func() cache { return &memoryCache{} })
}

func TestRealRedis(t *testing.T) {
	if os.Getenv("REPORT_REDIS_E2E") != "1" {
		t.Skip("requires the private Redis instance in the README")
	}
	checkCache(t, func() cache {
		c, backend, err := selectedCache()
		if err != nil || backend != "redis" {
			t.Fatalf("Redis setup: %v", err)
		}
		return c
	})
}
