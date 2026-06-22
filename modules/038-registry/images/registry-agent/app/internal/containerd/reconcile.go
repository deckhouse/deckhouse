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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const stateFileName = "deckhouse_hosts_state.json"

// markerFileName is a top-level marker (alongside stateFileName) that signals
// the agent owns registry.d on this node, so bashible step 030 defers to it.
const markerFileName = ".managed-by-agent"

// markerContent is static so an unchanged reconcile rewrites nothing.
var markerContent = []byte("managed-by=registry-agent\n")

// Reconcile makes the registry.d directory under root match desired: it writes
// hosts.toml + CA files for each managed host, records the managed host set in
// deckhouse_hosts_state.json, and removes directories of hosts that were
// managed previously but are no longer desired. Directories the reconciler
// never recorded (unmanaged/user dirs) are left untouched.
func Reconcile(root string, desired DesiredState) error {
	for host := range desired.Hosts {
		if err := validateHost(host); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(root, 0700); err != nil {
		return fmt.Errorf("create root %q: %w", root, err)
	}

	prev, err := readState(root)
	if err != nil {
		return err
	}

	for host, cfg := range desired.Hosts {
		if err := writeHost(root, host, cfg); err != nil {
			return fmt.Errorf("write host %q: %w", host, err)
		}
	}

	for _, host := range prev {
		if _, ok := desired.Hosts[host]; ok {
			continue
		}
		if err := validateHost(host); err != nil {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, host)); err != nil {
			return fmt.Errorf("remove stale host %q: %w", host, err)
		}
	}

	if err := writeState(root, desired); err != nil {
		return err
	}
	return reconcileMarker(root, len(desired.Hosts) > 0)
}

// reconcileMarker writes the ownership marker when the agent manages at least
// one host (so bashible step 030 defers to the agent), and removes it when the
// agent has nothing to manage (so step 030 reclaims registry.d — the
// auto-rollback path). The marker is a top-level file; Reconcile's host-dir
// cleanup never touches top-level files, so it persists across reconciles.
func reconcileMarker(root string, managed bool) error {
	path := filepath.Join(root, markerFileName)
	if !managed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove ownership marker: %w", err)
		}
		return nil
	}
	return saveFileIfChanged(path, markerContent, 0600)
}

func writeHost(root, host string, cfg HostConfig) error {
	dir := filepath.Join(root, host)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	expected := map[string]bool{"hosts.toml": true}
	caPaths := make([]string, len(cfg.Entries))
	for i, e := range cfg.Entries {
		if e.CA == "" {
			continue
		}
		name := fmt.Sprintf("%d.crt", i)
		if err := saveFileIfChanged(filepath.Join(dir, name), []byte(e.CA), 0600); err != nil {
			return err
		}
		caPaths[i] = filepath.Join(dir, name)
		expected[name] = true
	}

	toml, err := renderHostsTOML(cfg, caPaths)
	if err != nil {
		return err
	}
	if err := saveFileIfChanged(filepath.Join(dir, "hosts.toml"), []byte(toml), 0600); err != nil {
		return err
	}

	// Remove files in this host dir that are no longer referenced (e.g. CA
	// files for entries that were dropped). Subdirectories are left alone.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		if ent.IsDir() || expected[ent.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, ent.Name())); err != nil {
			return fmt.Errorf("remove stale file %q in host %q: %w", ent.Name(), host, err)
		}
	}
	return nil
}

func readState(root string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, stateFileName))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	var hosts []string
	if err := json.Unmarshal(data, &hosts); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return hosts, nil
}

func writeState(root string, desired DesiredState) error {
	hosts := make([]string, 0, len(desired.Hosts))
	for h := range desired.Hosts {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	data, err := json.Marshal(hosts)
	if err != nil {
		return err
	}
	return saveFileIfChanged(filepath.Join(root, stateFileName), data, 0600)
}

// validateHost rejects host names that could escape the registry.d root when
// used as a path element (path separators or parent refs). Registry hosts may
// legitimately contain ':' (host:port) and '.', so only path-traversal
// characters are rejected.
func validateHost(host string) error {
	if host == "" || host == "." || host == ".." ||
		strings.Contains(host, "/") || strings.Contains(host, `\`) ||
		strings.Contains(host, "..") {
		return fmt.Errorf("invalid registry host name %q", host)
	}
	return nil
}

// saveFileIfChanged writes content to path only when the file is absent or its
// content differs, so an unchanged reconcile performs no writes.
func saveFileIfChanged(path string, content []byte, perm os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil {
		if sha256.Sum256(existing) == sha256.Sum256(content) {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, content, perm)
}
