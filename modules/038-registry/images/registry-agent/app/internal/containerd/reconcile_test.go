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

package containerd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func primaryState() DesiredState {
	return DesiredState{Hosts: map[string]HostConfig{
		"registry.d8-system.svc:5001": {
			Server: "registry.d8-system.svc:5001",
			Entries: []HostEntry{
				{URL: "https://127.0.0.1:5001", Capabilities: []string{"pull", "resolve"}, CA: "AGENTCA"},
			},
		},
	}}
}

func TestReconcile_WritesHostAndState(t *testing.T) {
	root := t.TempDir()
	if err := Reconcile(root, primaryState()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	hostDir := filepath.Join(root, "registry.d8-system.svc:5001")
	if _, err := os.Stat(filepath.Join(hostDir, "hosts.toml")); err != nil {
		t.Fatalf("hosts.toml missing: %v", err)
	}
	ca, err := os.ReadFile(filepath.Join(hostDir, "0.crt"))
	if err != nil {
		t.Fatalf("ca file missing: %v", err)
	}
	if string(ca) != "AGENTCA" {
		t.Fatalf("ca content = %q, want AGENTCA", ca)
	}
	state, err := os.ReadFile(filepath.Join(root, "deckhouse_hosts_state.json"))
	if err != nil {
		t.Fatalf("state missing: %v", err)
	}
	if string(state) != `["registry.d8-system.svc:5001"]` {
		t.Fatalf("state = %s", state)
	}
}

func TestReconcile_Idempotent(t *testing.T) {
	root := t.TempDir()
	if err := Reconcile(root, primaryState()); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	tomlPath := filepath.Join(root, "registry.d8-system.svc:5001", "hosts.toml")
	info1, err := os.Stat(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(root, primaryState()); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	info2, err := os.Stat(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatalf("hosts.toml rewritten on identical reconcile (mtime changed)")
	}
}

func TestReconcile_RemovesStaleManagedHostButKeepsUnmanaged(t *testing.T) {
	root := t.TempDir()

	// An unmanaged dir the reconciler never recorded — must survive.
	unmanaged := filepath.Join(root, "user-registry.example.com")
	if err := os.MkdirAll(unmanaged, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unmanaged, "hosts.toml"), []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}

	// First reconcile manages host A.
	withA := DesiredState{Hosts: map[string]HostConfig{
		"a.example.com": {Server: "a.example.com", Entries: []HostEntry{{URL: "https://127.0.0.1:5001", Capabilities: []string{"pull"}}}},
	}}
	if err := Reconcile(root, withA); err != nil {
		t.Fatalf("reconcile A: %v", err)
	}

	// Second reconcile drops A, manages B.
	withB := DesiredState{Hosts: map[string]HostConfig{
		"b.example.com": {Server: "b.example.com", Entries: []HostEntry{{URL: "https://127.0.0.1:5001", Capabilities: []string{"pull"}}}},
	}}
	if err := Reconcile(root, withB); err != nil {
		t.Fatalf("reconcile B: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "a.example.com")); !os.IsNotExist(err) {
		t.Fatalf("stale managed host a.example.com not removed (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(root, "b.example.com")); err != nil {
		t.Fatalf("host b.example.com missing: %v", err)
	}
	if _, err := os.Stat(unmanaged); err != nil {
		t.Fatalf("unmanaged dir wrongly removed: %v", err)
	}
}

func TestReconcile_RejectsUnsafeHostName(t *testing.T) {
	unsafeHosts := []string{"../evil", "a/b"}
	for _, unsafe := range unsafeHosts {
		t.Run(unsafe, func(t *testing.T) {
			root := t.TempDir()
			err := Reconcile(root, DesiredState{Hosts: map[string]HostConfig{
				unsafe: {Server: "x", Entries: []HostEntry{{URL: "https://127.0.0.1:5001", Capabilities: []string{"pull"}}}},
			}})
			if err == nil {
				t.Fatalf("Reconcile with host %q: expected error, got nil", unsafe)
			}
			// Nothing must have been written: no state file, no escaped path.
			if _, statErr := os.Stat(filepath.Join(root, stateFileName)); !os.IsNotExist(statErr) {
				t.Fatalf("state file written despite invalid host %q (stat err=%v)", unsafe, statErr)
			}
			// The escaped path (relative to root) must not exist.
			escaped := filepath.Join(root, unsafe)
			if _, statErr := os.Stat(escaped); !os.IsNotExist(statErr) {
				t.Fatalf("escaped path %q exists despite invalid host (stat err=%v)", escaped, statErr)
			}
		})
	}
}

func TestReconcile_EmptyDesiredRemovesAllManaged(t *testing.T) {
	root := t.TempDir()

	// First reconcile manages two hosts.
	withTwo := DesiredState{Hosts: map[string]HostConfig{
		"a.example.com": {Server: "a.example.com", Entries: []HostEntry{{URL: "https://127.0.0.1:5001", Capabilities: []string{"pull"}}}},
		"b.example.com": {Server: "b.example.com", Entries: []HostEntry{{URL: "https://127.0.0.1:5002", Capabilities: []string{"pull"}}}},
	}}
	if err := Reconcile(root, withTwo); err != nil {
		t.Fatalf("reconcile two: %v", err)
	}

	// Second reconcile with empty desired — both host dirs must be removed.
	if err := Reconcile(root, DesiredState{Hosts: map[string]HostConfig{}}); err != nil {
		t.Fatalf("reconcile empty: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "a.example.com")); !os.IsNotExist(err) {
		t.Fatalf("a.example.com not removed after empty reconcile (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(root, "b.example.com")); !os.IsNotExist(err) {
		t.Fatalf("b.example.com not removed after empty reconcile (err=%v)", err)
	}

	// State file must contain [] not null.
	data, err := os.ReadFile(filepath.Join(root, stateFileName))
	if err != nil {
		t.Fatalf("state file missing: %v", err)
	}
	var hosts []string
	if err := json.Unmarshal(data, &hosts); err != nil {
		t.Fatalf("state file unparseable: %v", err)
	}
	if len(hosts) != 0 {
		t.Fatalf("state file hosts = %v, want []", hosts)
	}
	if string(data) != "[]" {
		t.Fatalf("state file = %q, want []", data)
	}
}

func TestReconcile_RemovesStaleCAFileWithinHost(t *testing.T) {
	root := t.TempDir()

	twoEntries := DesiredState{Hosts: map[string]HostConfig{
		"a.example.com": {Server: "a.example.com", Entries: []HostEntry{
			{URL: "https://127.0.0.1:5001", Capabilities: []string{"pull"}, CA: "CA0"},
			{URL: "https://10.0.0.1:5001", Capabilities: []string{"pull"}, CA: "CA1"},
		}},
	}}
	if err := Reconcile(root, twoEntries); err != nil {
		t.Fatalf("reconcile two: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a.example.com", "1.crt")); err != nil {
		t.Fatalf("1.crt missing after first reconcile: %v", err)
	}

	oneEntry := DesiredState{Hosts: map[string]HostConfig{
		"a.example.com": {Server: "a.example.com", Entries: []HostEntry{
			{URL: "https://127.0.0.1:5001", Capabilities: []string{"pull"}, CA: "CA0"},
		}},
	}}
	if err := Reconcile(root, oneEntry); err != nil {
		t.Fatalf("reconcile one: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a.example.com", "1.crt")); !os.IsNotExist(err) {
		t.Fatalf("stale 1.crt not removed within host dir (err=%v)", err)
	}
}

func TestReconcile_WritesMarkerWhenManaging(t *testing.T) {
	root := t.TempDir()
	if err := Reconcile(root, primaryState()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".managed-by-agent"))
	if err != nil {
		t.Fatalf("marker missing: %v", err)
	}
	if string(data) != "managed-by=registry-agent\n" {
		t.Fatalf("marker content = %q", data)
	}
}

func TestReconcile_EmptyDesiredRemovesMarker(t *testing.T) {
	root := t.TempDir()
	if err := Reconcile(root, primaryState()); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	markerPath := filepath.Join(root, ".managed-by-agent")
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("marker should exist after first reconcile: %v", err)
	}
	if err := Reconcile(root, DesiredState{Hosts: map[string]HostConfig{}}); err != nil {
		t.Fatalf("reconcile 2 (empty): %v", err)
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("marker should be removed on empty desired, stat err = %v", err)
	}
	// The managed host dir is also gone (existing behavior).
	if _, err := os.Stat(filepath.Join(root, "registry.d8-system.svc:5001")); !os.IsNotExist(err) {
		t.Fatalf("host dir should be removed on empty desired, stat err = %v", err)
	}
}

func TestReconcile_MarkerSurvivesHostChange(t *testing.T) {
	root := t.TempDir()
	if err := Reconcile(root, primaryState()); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	other := DesiredState{Hosts: map[string]HostConfig{
		"docker.io": {
			Server: "docker.io",
			Entries: []HostEntry{
				{URL: "https://127.0.0.1:5001", Capabilities: []string{"pull", "resolve"}},
			},
		},
	}}
	if err := Reconcile(root, other); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	// Old host dir removed, new present, marker still present (top-level, untouched by host cleanup).
	if _, err := os.Stat(filepath.Join(root, "registry.d8-system.svc:5001")); !os.IsNotExist(err) {
		t.Fatalf("old host dir should be gone, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docker.io")); err != nil {
		t.Fatalf("new host dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".managed-by-agent")); err != nil {
		t.Fatalf("marker should survive host change: %v", err)
	}
}

func TestReconcile_MarkerIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := Reconcile(root, primaryState()); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	markerPath := filepath.Join(root, ".managed-by-agent")
	info1, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("marker missing: %v", err)
	}
	if err := Reconcile(root, primaryState()); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	info2, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("marker missing after reconcile 2: %v", err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatalf("marker rewritten on unchanged reconcile (mtime changed): %v -> %v", info1.ModTime(), info2.ModTime())
	}
}
