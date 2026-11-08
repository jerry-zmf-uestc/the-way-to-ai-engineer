package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type cache interface {
	get() (string, error)
	set(string) error
	clear() error
}

type memoryCache struct{ value string }

func (m *memoryCache) get() (string, error) { return m.value, nil }
func (m *memoryCache) set(v string) error   { m.value = v; return nil }
func (m *memoryCache) clear() error         { return m.set("") }

type redisCache struct{ cli, socket, key string }

func (r redisCache) command(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.cli, append([]string{"-e", "--raw", "-s", r.socket}, args...)...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("private Redis command failed: %w", err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

func (r redisCache) get() (string, error) { return r.command("GET", r.key) }
func (r redisCache) set(v string) error {
	_, err := r.command("SET", r.key, v, "EX", "60")
	return err
}
func (r redisCache) clear() error {
	_, err := r.command("DEL", r.key)
	return err
}

type observation struct {
	value                    string
	hits, sourceReads, stale int
}

func exercise(c cache, fixed bool) (observation, error) {
	// Manually advance the same interleaving on every run.
	source := "v1"
	delayedRead := source
	result := observation{sourceReads: 1}
	source = "v2"
	if err := c.clear(); err != nil {
		return result, err
	}
	if err := c.set(delayedRead); err != nil {
		return result, err
	}

	// The immediate read after updating explicitly uses the source.
	// shortcut: ordinary cached readers may still see v1 until expiry.
	var value string
	if !fixed {
		var err error
		value, err = c.get()
		if err != nil {
			return result, err
		}
		if value != "" {
			result.hits++
		}
	}
	if value == "" {
		result.sourceReads++
		value = source
	}
	result.value = value
	if value != source {
		result.stale++
	}
	return result, nil
}

func selectedCache() (cache, string, error) {
	if os.Getenv("REPORT_REDIS_E2E") != "1" {
		return &memoryCache{}, "memory", nil
	}
	cli, socket := os.Getenv("REPORT_REDIS_CLI"), os.Getenv("REPORT_REDIS_SOCKET")
	info, err := os.Lstat(socket)
	if !filepath.IsAbs(cli) || !filepath.IsAbs(socket) || err != nil ||
		info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0600 {
		return nil, "", errors.New("use an absolute redis-cli path and the lab's private Unix socket (0600)")
	}
	dir, err := os.Stat(filepath.Dir(socket))
	if err != nil || dir.Mode().Perm() != 0700 {
		return nil, "", errors.New("the socket must be in a private directory (0700)")
	}
	return redisCache{cli, socket, "report-lab:" + rand.Text()}, "redis", nil
}

func main() {
	for _, fixed := range []bool{false, true} {
		c, backend, err := selectedCache()
		if err != nil {
			log.Fatal(err)
		}
		m, err := exercise(c, fixed)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("backend=%s fixed=%t value=%s hits=%d source_reads=%d stale=%d\n",
			backend, fixed, m.value, m.hits, m.sourceReads, m.stale)
	}
}
