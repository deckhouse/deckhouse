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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The host net.mptcp.enabled is not kept in the filter markers: they live in the netns rules,
// and the host value has to survive their removal. The value from before the guard is kept in
// a state file on the host /run instead. The guard writes nothing to the host root filesystem:
// /etc is read-only on Deckhouse Engine.
//
// /run is a tmpfs, so a reboot clears the record together with the change it describes: the
// kernel starts with the OS value again, and the guard records it anew and sets 0 once the
// filter is installed.

// readHostMPTCPState returns the original value recorded in the state file and whether the
// file exists.
func readHostMPTCPState(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	v := strings.TrimSpace(string(b))
	if v != "0" && v != "1" {
		return "", true, fmt.Errorf("%s: unexpected original value %q", path, v)
	}
	return v, true, nil
}

// syncHostMPTCP runs on every resync, also when no filter backend is available, so that
// switching off disableMPTCP or Enforce always removes the record and restores the value.
func (g *Guard) syncHostMPTCP(st Settings, filterInstalled bool) {
	path := g.cfg.HostMPTCPStateFile
	if path == "" {
		return
	}
	sysctl := g.cfg.HostMPTCPSysctl
	if sysctl == "" {
		sysctl = mptcpSysctl
	}
	cur, err := readMPTCP(sysctl)
	if err != nil {
		g.log.Error("read host net.mptcp.enabled", "error", err)
		return
	}
	orig, exists, err := readHostMPTCPState(path)
	if err != nil {
		g.log.Error("read host MPTCP state", "error", err)
		return
	}

	if manageMPTCP(st) {
		// MPTCP is only disabled together with the filter. Without one the node is left as is,
		// including a record from an earlier run: a restarted sidecar may not have detected the
		// backend yet.
		if !filterInstalled || cur == mptcpAbsent {
			return
		}
		if !exists {
			// Recorded before the value is changed: a crash in between leaves a record of the
			// untouched value, not a 0 nobody remembers.
			if err := writeAtomic(path, cur+"\n"); err != nil {
				g.log.Error("write host MPTCP state", "error", err)
				return
			}
		}
		if cur == "1" {
			if err := os.WriteFile(sysctl, []byte("0"), 0o644); err != nil {
				g.log.Error("set host net.mptcp.enabled", "error", err)
			}
		}
		return
	}

	if !exists {
		return
	}
	if orig == "1" && cur == "0" {
		if err := os.WriteFile(sysctl, []byte("1"), 0o644); err != nil {
			g.log.Error("restore host net.mptcp.enabled", "error", err)
			return
		}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		g.log.Error("remove host MPTCP state", "error", err)
	}
}

func readMPTCP(path string) (string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return mptcpAbsent, nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// writeAtomic replaces path through a rename, so a reader never sees a partial record.
func writeAtomic(path, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
