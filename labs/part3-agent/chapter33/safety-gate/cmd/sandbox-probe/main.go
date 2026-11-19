// Command sandbox-probe is a fixed container-only probe, never an Agent tool.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type report struct {
	UID                                                                                           int
	ApprovedRead, MountReadOnly, RootReadOnly, TemporaryWrite, UnmountedAbsent, NoRoute, Loopback bool
	NoNewPrivileges, NoCapabilities                                                               bool
}

func pidEvents() (uint64, error) {
	data, err := os.ReadFile("/sys/fs/cgroup/pids.events")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "max" {
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return 0, errors.New("missing pids max counter")
}

type CPUStats struct{ Periods, Throttled, Microseconds uint64 }

func cpuStats() (CPUStats, error) {
	data, err := os.ReadFile("/sys/fs/cgroup/cpu.stat")
	if err != nil {
		return CPUStats{}, err
	}
	out := CPUStats{}
	seen := 0
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		var target *uint64
		switch fields[0] {
		case "nr_periods":
			target = &out.Periods
		case "nr_throttled":
			target = &out.Throttled
		case "throttled_usec":
			target = &out.Microseconds
		default:
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return CPUStats{}, err
		}
		*target = value
		seen++
	}
	if seen != 3 {
		return CPUStats{}, errors.New("missing CPU throttle counters")
	}
	return out, nil
}

func main() {
	if os.Getenv("SAFETY_SANDBOX_PROBE") != "1" || runtime.GOOS != "linux" || os.Getuid() != 65532 || len(os.Args) > 2 {
		os.Exit(2)
	}
	mode := "probe"
	if len(os.Args) == 2 {
		mode = os.Args[1]
	}
	switch mode {
	case "cpu":
		before, err := cpuStats()
		if err != nil {
			os.Exit(3)
		}
		started := time.Now()
		var work uint64
		for time.Since(started) < 2*time.Second {
			for range 4096 {
				work = work*1664525 + 1013904223
			}
		}
		runtime.KeepAlive(work)
		after, err := cpuStats()
		if err != nil {
			os.Exit(3)
		}
		if err := json.NewEncoder(os.Stdout).Encode(struct {
			Before, After CPUStats
		}{before, after}); err != nil {
			os.Exit(1)
		}
	case "pids":
		before, err := pidEvents()
		if err != nil {
			os.Exit(3)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		children := []*exec.Cmd{}
		for range 16 {
			child := exec.CommandContext(ctx, "/probe", "wait")
			if child.Start() == nil {
				children = append(children, child)
			}
		}
		for _, child := range children {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
		cancel()
		after, err := pidEvents()
		if err != nil {
			os.Exit(3)
		}
		if err := json.NewEncoder(os.Stdout).Encode(struct {
			Started       int
			Before, After uint64
		}{len(children), before, after}); err != nil {
			os.Exit(1)
		}
	case "memory":
		blocks := make([][]byte, 0, 256)
		for range 256 {
			block := make([]byte, 1<<20)
			for n := 0; n < len(block); n += 4096 {
				block[n] = 1
			}
			blocks = append(blocks, block)
		}
		runtime.KeepAlive(blocks)
	case "wait":
		time.Sleep(15 * time.Second)
	case "probe":
		r := report{UID: os.Getuid()}
		status, err := os.ReadFile("/proc/self/status")
		if err == nil {
			for _, line := range strings.Split(string(status), "\n") {
				fields := strings.Fields(line)
				if len(fields) != 2 {
					continue
				}
				if fields[0] == "NoNewPrivs:" {
					r.NoNewPrivileges = fields[1] == "1"
				}
				if fields[0] == "CapEff:" {
					r.NoCapabilities = fields[1] == "0000000000000000"
				}
			}
		}
		data, err := os.ReadFile("/input/approved.txt")
		r.ApprovedRead = err == nil && string(data) == "course-public-fixture"
		r.MountReadOnly = errors.Is(os.WriteFile("/input/attempt.txt", []byte("attempt"), 0600), syscall.EROFS)
		r.RootReadOnly = errors.Is(os.WriteFile("/root-writable.txt", []byte("attempt"), 0600), syscall.EROFS)
		r.TemporaryWrite = os.WriteFile("/tmp/probe.txt", []byte("temporary"), 0600) == nil
		_, err = os.ReadFile("/host-canary.txt")
		r.UnmountedAbsent = errors.Is(err, os.ErrNotExist)
		connection, err := net.DialTimeout("tcp", "198.51.100.1:443", time.Second)
		r.NoRoute = errors.Is(err, syscall.ENETUNREACH) || errors.Is(err, syscall.EHOSTUNREACH)
		if connection != nil {
			connection.Close()
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err == nil {
			connection, err = net.DialTimeout("tcp", listener.Addr().String(), time.Second)
			r.Loopback = err == nil
			if connection != nil {
				connection.Close()
			}
			listener.Close()
		}
		if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
			os.Exit(1)
		}
	default:
		os.Exit(2)
	}
}
