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
	file    string
	sysctl  string
	enforce Settings
}

func newHostEnv(t *testing.T, cur string) *hostEnv {
	dir := t.TempDir()
	e := &hostEnv{
		file:    filepath.Join(dir, "90-d8-ipv4-options-guard.conf"),
		sysctl:  filepath.Join(dir, "mptcp_enabled"),
		enforce: Settings{Mode: ModeEnforce, DisableMPTCP: true},
	}
	e.g = &Guard{
		cfg: Config{HostSysctlFile: e.file, HostMPTCPSysctl: e.sysctl},
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

func (e *hostEnv) fileExists() bool {
	_, err := os.Stat(e.file)
	return err == nil
}

func TestHostMPTCPRestoredAfterReboot(t *testing.T) {
	e := newHostEnv(t, "1")
	e.g.syncHostMPTCP(e.enforce, true)
	if e.cur(t) != "0" || !e.fileExists() {
		t.Fatalf("Enforce: cur=%s file=%v", e.cur(t), e.fileExists())
	}

	// Reboot: systemd-sysctl applied the file, the guard state is gone.
	e.setCur(t, "0")
	e.g.syncHostMPTCP(e.enforce, true)
	if orig, _, _ := readHostSysctl(e.file); orig != "1" {
		t.Fatalf("the original value must survive the reboot, got %q", orig)
	}

	e.g.syncHostMPTCP(Settings{Mode: ModeAudit, DisableMPTCP: true}, true)
	if e.cur(t) != "1" || e.fileExists() {
		t.Fatalf("Audit after reboot: cur=%s file=%v, want 1 and no file", e.cur(t), e.fileExists())
	}
}

func TestHostMPTCPKeepsAdminDisabledValue(t *testing.T) {
	e := newHostEnv(t, "0")
	e.g.syncHostMPTCP(e.enforce, true)
	e.g.syncHostMPTCP(Settings{Mode: ModeDisabled}, true)
	if e.cur(t) != "0" || e.fileExists() {
		t.Fatalf("an admin-disabled MPTCP must stay 0: cur=%s file=%v", e.cur(t), e.fileExists())
	}
}

func TestHostMPTCPCleanupWithoutBackend(t *testing.T) {
	e := newHostEnv(t, "1")
	e.g.syncHostMPTCP(e.enforce, true)

	// Restarted sidecar cannot detect the iptables mode, the module is switched to Disabled.
	e.g.syncHostMPTCP(Settings{Mode: ModeDisabled}, false)
	if e.cur(t) != "1" || e.fileExists() {
		t.Fatalf("Disabled without a backend: cur=%s file=%v, want 1 and no file", e.cur(t), e.fileExists())
	}
}

func TestHostMPTCPUntouchedWhenUnprotected(t *testing.T) {
	e := newHostEnv(t, "1")
	e.g.syncHostMPTCP(e.enforce, false)
	if e.cur(t) != "1" || e.fileExists() {
		t.Fatalf("no filter, no MPTCP change: cur=%s file=%v", e.cur(t), e.fileExists())
	}
}

func TestHostMPTCPKeptWhileBackendUnknown(t *testing.T) {
	e := newHostEnv(t, "1")
	e.g.syncHostMPTCP(e.enforce, true)

	// After a reboot the file has already set 0, the backend is not detected yet.
	e.g.syncHostMPTCP(e.enforce, false)
	if e.cur(t) != "0" || !e.fileExists() {
		t.Fatalf("Enforce without a detected backend: cur=%s file=%v, want 0 and the file kept", e.cur(t), e.fileExists())
	}
}

func TestHostSysctlFromOlderGuard(t *testing.T) {
	e := newHostEnv(t, "0")
	if err := os.WriteFile(e.file, []byte("# Managed by the d8 ipv4-options-guard (CNI module settings).\nnet.mptcp.enabled = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.g.syncHostMPTCP(Settings{Mode: ModeAudit}, true)
	if e.cur(t) != "1" || e.fileExists() {
		t.Fatalf("a file without the original value restores the OS default: cur=%s file=%v", e.cur(t), e.fileExists())
	}
}
