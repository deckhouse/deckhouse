// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tofu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// Persistent kubernetes provider daemon.
//
// dhctl spawns the terraform-provider-kubernetes binary in tf5server.WithDebug
// mode once per bootstrap. The provider writes its TF_REATTACH_PROVIDERS JSON
// into a temp file; we read it and inject the same value as env on every
// subsequent `tofu` invocation. Every tofu run (init/plan/show/apply) then
// reconnects to the same gRPC server instead of forking a fresh provider, which
// saves the ~400-500ms cold-start × 5-6 spawns observed per pipeline plan
// (was ~10s of 12s plan wall-clock).
//
// Cache-friendly: globalTypeCache, openapi v3 docs and the RESTMapper survive
// across tofu invocations as long as the daemon is alive.
//
// Health: every tofu invocation goes through EnsureProviderDaemon, which
// checks the daemon process is still alive (kill -0) and respawns it if not.
// If startup fails entirely we fall back silently to the standard
// per-invocation spawn (TF_REATTACH_PROVIDERS just isn't set).
//
// Disable via DHCTL_PROVIDER_DAEMON=off.

var (
	daemonMu         sync.Mutex
	daemonEnv        string
	daemonCmd        *exec.Cmd
	daemonPluginsDir string
	daemonDisabled   bool
)

// EnableProviderDaemon remembers the pluginsDir to use for any (re)spawn of
// the daemon and starts it immediately. Subsequent calls update the pluginsDir
// only if the daemon needs respawning.
func EnableProviderDaemon(pluginsDir string) {
	daemonMu.Lock()
	if pluginsDir != "" {
		daemonPluginsDir = pluginsDir
	}
	daemonMu.Unlock()
	_ = EnsureProviderDaemon()
}

// EnsureProviderDaemon makes sure a daemon is running and returns the
// TF_REATTACH_PROVIDERS env value to use. Returns "" if the daemon is
// unavailable (never enabled, disabled by env, or failed to spawn). Callers
// then skip setting the env var and let tofu spawn its own plugin process.
//
// Safe to call from every tofu invocation — auto-restarts a dead daemon and is
// otherwise an O(1) liveness check.
func EnsureProviderDaemon() string {
	ctx := context.Background()
	daemonMu.Lock()
	defer daemonMu.Unlock()

	if daemonDisabled {
		return ""
	}
	if v := os.Getenv("DHCTL_PROVIDER_DAEMON"); v == "off" || v == "0" || v == "false" {
		daemonDisabled = true
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("provider daemon disabled via DHCTL_PROVIDER_DAEMON=%q", v))
		return ""
	}
	if daemonPluginsDir == "" {
		// Init hasn't fired yet; tofuCmd just returns empty and tofu falls
		// back to its default spawn-per-invocation behaviour.
		return ""
	}

	if isDaemonAliveLocked() {
		return daemonEnv
	}
	if daemonCmd != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("provider daemon pid=%d died, restarting", daemonCmd.Process.Pid))
		go func(c *exec.Cmd) { _, _ = c.Process.Wait() }(daemonCmd)
		daemonCmd = nil
		daemonEnv = ""
	}
	env, cmd, err := startProviderDaemon(daemonPluginsDir)
	if err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("provider daemon disabled: %v", err))
		return ""
	}
	daemonEnv = env
	daemonCmd = cmd
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("provider daemon ready pid=%d", cmd.Process.Pid))
	return daemonEnv
}

// StopProviderDaemon kills the daemon if running. Safe to call multiple times
// and from a defer in main(). Container/process exit will also clean up.
func StopProviderDaemon() {
	ctx := context.Background()
	daemonMu.Lock()
	cmd := daemonCmd
	daemonCmd = nil
	daemonEnv = ""
	daemonDisabled = true // don't auto-restart from a parallel goroutine
	daemonMu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("stopping provider daemon pid=%d", cmd.Process.Pid))
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// isDaemonAliveLocked checks if the recorded daemon process still exists.
// Must be called with daemonMu held.
func isDaemonAliveLocked() bool {
	if daemonCmd == nil || daemonCmd.Process == nil {
		return false
	}
	// Signal 0 reports whether the pid is reachable without delivering one.
	return daemonCmd.Process.Signal(syscall.Signal(0)) == nil
}

// findProviderBinary searches under pluginsDir for the kubernetes provider
// binary. The path varies by registry/version, so we walk the standard layout
// `<pluginsDir>/<registry>/hashicorp/kubernetes/<version>/<os>_<arch>/terraform-provider-kubernetes`.
func findProviderBinary(pluginsDir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(pluginsDir, "*/hashicorp/kubernetes/*/*/terraform-provider-kubernetes*"))
	if err != nil {
		return "", err
	}
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return m, nil
		}
	}
	return "", fmt.Errorf("kubernetes provider binary not found under %s", pluginsDir)
}

func startProviderDaemon(pluginsDir string) (string, *exec.Cmd, error) {
	binary, err := findProviderBinary(pluginsDir)
	if err != nil {
		return "", nil, err
	}

	reattachFile := filepath.Join(os.TempDir(), fmt.Sprintf("dhctl-tpk-reattach-%d.json", os.Getpid()))
	_ = os.Remove(reattachFile)

	// Deliberately exec.Command, not exec.CommandContext: the daemon must
	// outlive the context of whichever tofu invocation happened to spawn it —
	// it is shared across every init/plan/show/apply of the whole bootstrap.
	// Binding it to a per-call ctx would kill the daemon as soon as that call
	// returns. Its lifecycle is managed explicitly via StopProviderDaemon
	// (registered in dhctl's onShutdown handlers).
	cmd := exec.Command(binary, "-reattach-file="+reattachFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Forward stdout/stderr so panics & misconfig are visible in a debug log file. The daemon
	// emits raw subprocess output (not slog records), so we write straight to the file; it
	// lives for the daemon's lifetime and is reclaimed on process exit.
	logFile := filepath.Join(os.TempDir(), fmt.Sprintf("dhctl-tpk-daemon-%d-%s.log", os.Getpid(), time.Now().Format("20060102150405")))
	daemonLog, err := os.Create(logFile)
	if err != nil {
		return "", nil, err
	}
	cmd.Stdout = daemonLog
	cmd.Stderr = daemonLog
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start provider daemon: %w", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var data []byte
	for time.Now().Before(deadline) {
		if cmd.Process.Signal(syscall.Signal(0)) != nil {
			// Daemon died during startup.
			return "", nil, fmt.Errorf("provider daemon exited during startup")
		}
		d, err := os.ReadFile(reattachFile)
		if err == nil && len(d) > 0 {
			data = d
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = os.Remove(reattachFile)
	if len(data) == 0 {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
		return "", nil, fmt.Errorf("provider daemon: timed out waiting for reattach file")
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
		return "", nil, fmt.Errorf("provider daemon: invalid reattach JSON: %w", err)
	}

	return string(data), cmd, nil
}
