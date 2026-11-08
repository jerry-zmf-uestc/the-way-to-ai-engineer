package main

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestActualContainerBoundaries(t *testing.T) {
	if os.Getenv("SAFETY_SANDBOX_E2E") != "1" {
		t.Skip("explicit SAFETY_SANDBOX_E2E=1 required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	endpoint := ""
	docker := func(callCtx context.Context, args ...string) *exec.Cmd {
		if endpoint != "" {
			args = append([]string{"--host", endpoint}, args...)
		}
		cmd := exec.CommandContext(callCtx, "docker", args...)
		if endpoint != "" {
			for _, value := range os.Environ() {
				if !strings.HasPrefix(value, "DOCKER_HOST=") && !strings.HasPrefix(value, "DOCKER_CONTEXT=") {
					cmd.Env = append(cmd.Env, value)
				}
			}
		}
		return cmd
	}
	run := func(args ...string) []byte {
		t.Helper()
		out, err := docker(ctx, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %s: %v: %s", args[0], err, out)
		}
		return out
	}
	endpoint = strings.TrimSpace(string(run("context", "inspect", "--format", "{{.Endpoints.docker.Host}}")))
	if override := os.Getenv("DOCKER_HOST"); override != "" && os.Getenv("DOCKER_CONTEXT") == "" {
		endpoint = override
	}
	if !strings.HasPrefix(endpoint, "unix://") {
		t.Fatal("only a local Unix Docker endpoint is allowed")
	}
	architecture := strings.TrimSpace(string(run("info", "--format", "{{.OSType}}/{{.Architecture}}")))
	arch := map[string]string{"linux/aarch64": "arm64", "linux/arm64": "arm64", "linux/x86_64": "amd64", "linux/amd64": "amd64"}[architecture]
	if arch == "" {
		t.Fatal("unsupported Linux engine architecture", architecture)
	}
	root := t.TempDir()
	buildDir, approved := filepath.Join(root, "build"), filepath.Join(root, "approved")
	for _, dir := range []string{buildDir, approved} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Writable source permissions isolate read-only mount enforcement from UID denial.
	if err := os.Chmod(approved, 0777); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(approved, "approved.txt"):      "course-public-fixture",
		filepath.Join(root, "host-canary.txt"):       "synthetic-unmounted-fixture",
		filepath.Join(buildDir, "root-writable.txt"): "public-root-fixture",
	} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(buildDir, "root-writable.txt"), 0666); err != nil {
		t.Fatal(err)
	}
	compiler := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(buildDir, "probe"), "./cmd/sandbox-probe")
	compiler.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
	if out, err := compiler.CombinedOutput(); err != nil {
		t.Fatalf("compile fixed probe: %v: %s", err, out)
	}
	marker := strings.ToLower(rand.Text())
	image := "course-safety-probe:" + marker
	label := "course.safety.probe=" + marker
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		for _, kind := range []string{"container", "image"} {
			out, err := docker(cleanup, kind, "ls", "--all", "--quiet", "--filter", "label="+label).CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != "" {
				t.Errorf("owned %s cleanup not confirmed: %v: %s", kind, err, out)
			}
		}
	})
	// Register by unique label before mutations, including an unknown CLI outcome.
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		for _, kind := range []string{"container", "image"} {
			out, err := docker(cleanup, kind, "ls", "--all", "--quiet", "--filter", "label="+label).CombinedOutput()
			if err != nil {
				t.Errorf("list owned %s: %v", kind, err)
				continue
			}
			for _, id := range strings.Fields(string(out)) {
				if (len(id) != 12 && len(id) != 64) || strings.Trim(id, "0123456789abcdef") != "" {
					t.Errorf("invalid owned %s identity", kind)
					continue
				}
				args := []string{"rm", "-f", id}
				if kind == "image" {
					args = []string{"image", "rm", id}
				}
				if out, err := docker(cleanup, args...).CombinedOutput(); err != nil {
					t.Errorf("remove owned %s: %v: %s", kind, err, out)
				}
			}
		}
	})
	archive := filepath.Join(root, "rootfs.tar")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := tar.NewWriter(file)
	for _, name := range []string{"probe", "root-writable.txt"} {
		data, err := os.ReadFile(filepath.Join(buildDir, name))
		if err != nil {
			t.Fatal(err)
		}
		mode := int64(0755)
		if name == "root-writable.txt" {
			mode = 0666
		}
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	run("image", "import", "--platform", "linux/"+arch, "--change", "USER 65532:65532", "--change", `ENTRYPOINT ["/probe"]`, "--change", "LABEL "+label, archive, image)
	create := func(mode string) string {
		t.Helper()
		args := []string{
			"create", "--label", label,
			"--read-only", "--network=none", "--user=65532:65532",
			"--cap-drop=ALL", "--security-opt=no-new-privileges=true",
			"--memory=64m", "--memory-swap=64m", "--cpus=0.5", "--pids-limit=16",
			"--tmpfs", "/tmp:rw,noexec,nosuid,size=8m,mode=1777",
			"--mount", "type=bind,src=" + approved + ",dst=/input,readonly",
			"--env", "SAFETY_SANDBOX_PROBE=1", "--env", "GOMAXPROCS=1",
			image, mode,
		}
		id := strings.TrimSpace(string(run(args...)))
		if len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
			t.Fatal("invalid created container identity")
		}
		return id
	}
	id := create("probe")
	var config struct {
		ReadonlyRootfs       bool
		NetworkMode          string
		Memory, MemorySwap   int64
		NanoCpus, PidsLimit  int64
		CapDrop, SecurityOpt []string
	}
	if err := json.Unmarshal(run("inspect", "--format", "{{json .HostConfig}}", id), &config); err != nil {
		t.Fatal(err)
	}
	if !config.ReadonlyRootfs || config.NetworkMode != "none" || config.Memory != 64<<20 || config.MemorySwap != 64<<20 || config.NanoCpus != 500000000 || config.PidsLimit != 16 || !slices.Contains(config.CapDrop, "ALL") || !slices.Contains(config.SecurityOpt, "no-new-privileges=true") {
		t.Fatalf("unexpected configured limits: %+v", config)
	}
	var actual struct {
		UID                                                                                           int
		ApprovedRead, MountReadOnly, RootReadOnly, TemporaryWrite, UnmountedAbsent, NoRoute, Loopback bool
		NoNewPrivileges, NoCapabilities                                                               bool
	}
	if err := json.Unmarshal(run("start", "-a", id), &actual); err != nil {
		t.Fatal(err)
	}
	if actual.UID != 65532 || !actual.ApprovedRead || !actual.MountReadOnly || !actual.RootReadOnly || !actual.TemporaryWrite || !actual.UnmountedAbsent || !actual.NoRoute || !actual.Loopback || !actual.NoNewPrivileges || !actual.NoCapabilities {
		t.Fatalf("boundary probe: %+v", actual)
	}
	memory := create("memory")
	_, _ = docker(ctx, "start", "-a", memory).CombinedOutput()
	state := strings.TrimSpace(string(run("inspect", "--format", "{{.State.OOMKilled}}/{{.State.ExitCode}}/{{.State.Running}}", memory)))
	if state != "true/137/false" {
		t.Fatal("memory limit not evidenced", state)
	}
	pid := create("pids")
	var pidResult struct {
		Started       int
		Before, After uint64
	}
	if err := json.Unmarshal(run("start", "-a", pid), &pidResult); err != nil {
		t.Fatal(err)
	}
	if pidResult.Started < 1 || pidResult.Started > 16 || pidResult.After <= pidResult.Before {
		t.Fatalf("PID limit not enforced: %+v", pidResult)
	}
	cpu := create("cpu")
	var cpuResult struct {
		Before, After struct{ Periods, Throttled, Microseconds uint64 }
	}
	if err := json.Unmarshal(run("start", "-a", cpu), &cpuResult); err != nil {
		t.Fatal(err)
	}
	if cpuResult.After.Periods <= cpuResult.Before.Periods || cpuResult.After.Throttled <= cpuResult.Before.Throttled || cpuResult.After.Microseconds <= cpuResult.Before.Microseconds {
		t.Fatalf("CPU throttle not evidenced: %+v", cpuResult)
	}
	waiter := create("wait")
	run("start", waiter)
	waitCtx, stop := context.WithTimeout(ctx, 300*time.Millisecond)
	_, _ = docker(waitCtx, "wait", waiter).CombinedOutput()
	stopped := waitCtx.Err()
	stop()
	if stopped != context.DeadlineExceeded || strings.TrimSpace(string(run("inspect", "--format", "{{.State.Running}}", waiter))) != "true" {
		t.Fatal("client wait is not container cancellation", stopped)
	}
	run("kill", waiter)
	if strings.TrimSpace(string(run("inspect", "--format", "{{.State.Running}}", waiter))) != "false" {
		t.Fatal("owned process still running")
	}
	// Drop this created ID: cleanup must discover ownership from the pre-registered label.
	_ = create("wait")
	t.Log(fmt.Sprintf("engine=%s boundaries=%+v memory=OOMKilled pid_events=%d->%d started=%d wait_timeout=container_still_running explicit_kill=stopped", architecture, actual, pidResult.Before, pidResult.After, pidResult.Started))
	t.Logf("CPU throttle: before=%+v after=%+v", cpuResult.Before, cpuResult.After)
}
