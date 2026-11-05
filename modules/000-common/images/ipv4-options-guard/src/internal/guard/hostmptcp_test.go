/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package guard

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type hostEnv struct {
	g       *Guard
	dir     string
	state   string
	sysctl  string
	enforce Settings
}

func newHostEnv(t *testing.T, cur string) *hostEnv {
	dir := t.TempDir()
	run := filepath.Join(dir, "run")
	if err := os.Mkdir(run, 0o755); err != nil {
		t.Fatal(err)
	}
	e := &hostEnv{
		dir:     run,
		state:   filepath.Join(run, "host-mptcp-original-state"),
		sysctl:  filepath.Join(dir, "mptcp_enabled"),
		enforce: Settings{Mode: ModeEnforce, DisableMPTCP: true},
	}
	e.g = &Guard{
		cfg: Config{HostMPTCPStateFile: e.state, HostMPTCPSysctl: e.sysctl},
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	e.setCur(t, cur)
	return e
}

func (e *hostEnv) setCur(t *testing.T, v string) {
	if err := os.WriteFile(e.sysctl, []byte(v+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e *hostEnv) cur(t *testing.T) string {
	b, err := os.ReadFile(e.sysctl)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// recorded returns the original value in the state file, or "" when there is none.
func (e *hostEnv) recorded(t *testing.T) string {
	b, err := os.ReadFile(e.state)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// reboot clears the host /run and brings the OS value back.
func (e *hostEnv) reboot(t *testing.T, osValue string) {
	if err := os.RemoveAll(e.dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(e.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	e.setCur(t, osValue)
}

func TestHostMPTCPEnforceAndRestore(t *testing.T) {
	e := newHostEnv(t, "1")
	e.g.syncHostMPTCP(e.enforce, true)
	if e.cur(t) != "0" || e.recorded(t) != "1" {
		t.Fatalf("Enforce: cur=%s recorded=%q, want 0 and 1", e.cur(t), e.recorded(t))
	}

	// A resync in the same mode changes nothing.
	e.g.syncHostMPTCP(e.enforce, true)
	if e.cur(t) != "0" || e.recorded(t) != "1" {
		t.Fatalf("Enforce resync: cur=%s recorded=%q", e.cur(t), e.recorded(t))
	}

	e.g.syncHostMPTCP(Settings{Mode: ModeAudit, DisableMPTCP: true}, true)
	if e.cur(t) != "1" || e.recorded(t) != "" {
		t.Fatalf("Audit: cur=%s recorded=%q, want 1 and no record", e.cur(t), e.recorded(t))
	}
	entries, err := os.ReadDir(e.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the state directory must be left empty, found %v", entries)
	}
}

func TestHostMPTCPAfterReboot(t *testing.T) {
	e := newHostEnv(t, "1")
	e.g.syncHostMPTCP(e.enforce, true)

	// Reboot: /run is cleared and the kernel starts with the OS value.
	e.reboot(t, "1")
	e.g.syncHostMPTCP(e.enforce, true)
	if e.cur(t) != "0" || e.recorded(t) != "1" {
		t.Fatalf("Enforce after reboot: cur=%s recorded=%q, want 0 and 1", e.cur(t), e.recorded(t))
	}

	e.g.syncHostMPTCP(Settings{Mode: ModeDisabled}, true)
	if e.cur(t) != "1" || e.recorded(t) != "" {
		t.Fatalf("Disabled after reboot: cur=%s recorded=%q, want 1 and no record", e.cur(t), e.recorded(t))
	}
}

func TestHostMPTCPKeepsAdminDisabledValue(t *testing.T) {
	e := newHostEnv(t, "0")
	e.g.syncHostMPTCP(e.enforce, true)
	if e.recorded(t) != "0" {
		t.Fatalf("recorded=%q, want 0", e.recorded(t))
	}
	e.g.syncHostMPTCP(Settings{Mode: ModeDisabled}, true)
	if e.cur(t) != "0" || e.recorded(t) != "" {
		t.Fatalf("an admin-disabled MPTCP must stay 0: cur=%s recorded=%q", e.cur(t), e.recorded(t))
	}
}

func TestHostMPTCPCleanupWithoutBackend(t *testing.T) {
	e := newHostEnv(t, "1")
	e.g.syncHostMPTCP(e.enforce, true)

	// Restarted sidecar cannot detect the iptables mode, the module is switched to Disabled.
	e.g.syncHostMPTCP(Settings{Mode: ModeDisabled}, false)
	if e.cur(t) != "1" || e.recorded(t) != "" {
		t.Fatalf("Disabled without a backend: cur=%s recorded=%q, want 1 and no record", e.cur(t), e.recorded(t))
	}
}

func TestHostMPTCPUntouchedWhenUnprotected(t *testing.T) {
	e := newHostEnv(t, "1")
	e.g.syncHostMPTCP(e.enforce, false)
	if e.cur(t) != "1" || e.recorded(t) != "" {
		t.Fatalf("no filter, no MPTCP change: cur=%s recorded=%q", e.cur(t), e.recorded(t))
	}
}

func TestHostMPTCPKeptWhileBackendUnknown(t *testing.T) {
	e := newHostEnv(t, "1")
	e.g.syncHostMPTCP(e.enforce, true)

	// The sidecar restarted on the same boot and has not detected the backend yet.
	e.g.syncHostMPTCP(e.enforce, false)
	if e.cur(t) != "0" || e.recorded(t) != "1" {
		t.Fatalf("Enforce without a detected backend: cur=%s recorded=%q, want 0 and 1 kept", e.cur(t), e.recorded(t))
	}
}

func TestHostMPTCPNotChangedWithoutRecord(t *testing.T) {
	e := newHostEnv(t, "1")
	// The state directory is not mounted: the value is not changed when it cannot be recorded.
	e.g.cfg.HostMPTCPStateFile = filepath.Join(e.dir, "missing", "host-mptcp-original-state")
	e.g.syncHostMPTCP(e.enforce, true)
	if e.cur(t) != "1" {
		t.Fatalf("cur=%s, want 1 when the original value cannot be recorded", e.cur(t))
	}
}

func TestHostMPTCPInvalidRecord(t *testing.T) {
	e := newHostEnv(t, "0")
	if err := os.WriteFile(e.state, []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An unreadable record is not guessed at: neither restored nor overwritten.
	e.g.syncHostMPTCP(Settings{Mode: ModeDisabled}, true)
	if e.cur(t) != "0" || e.recorded(t) != "garbage" {
		t.Fatalf("Disabled with an invalid record: cur=%s recorded=%q", e.cur(t), e.recorded(t))
	}
	e.g.syncHostMPTCP(e.enforce, true)
	if e.cur(t) != "0" || e.recorded(t) != "garbage" {
		t.Fatalf("Enforce with an invalid record: cur=%s recorded=%q", e.cur(t), e.recorded(t))
	}
}
