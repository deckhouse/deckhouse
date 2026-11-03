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
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// The host net.mptcp.enabled is managed through the sysctl.d file only, not through the
// filter markers: the file survives reboots, so it also keeps the value from before the guard
// (after a reboot the current value is already the one the file applied).
const hostOrigPrefix = "# original net.mptcp.enabled = "

func hostSysctlContent(orig string) string {
	return "# Managed by the d8 ipv4-options-guard (CNI module settings).\n" +
		hostOrigPrefix + orig + "\n" +
		"net.mptcp.enabled = 0\n"
}

// readHostSysctl returns the original value recorded in the file and whether the file exists.
// A file written by an older guard has no record; the OS default (1) is assumed then.
func readHostSysctl(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), hostOrigPrefix); ok && (v == "0" || v == "1") {
			return v, true, nil
		}
	}
	return "1", true, nil
}

// syncHostMPTCP runs on every resync, also when no filter backend is available, so that
// switching off disableMPTCP or Enforce always removes the file and restores the value.
func (g *Guard) syncHostMPTCP(st Settings, filterInstalled bool) {
	path := g.cfg.HostSysctlFile
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
	orig, exists, err := readHostSysctl(path)
	if err != nil {
		g.log.Error("read host sysctl file", "error", err)
		return
	}

	if manageMPTCP(st) {
		// MPTCP is only disabled together with the filter. Without one the node is left as is,
		// including a file from an earlier run: the backend may just not be detected yet after a reboot.
		if !filterInstalled || cur == mptcpAbsent {
			return
		}
		if !exists {
			orig = cur
		}
		if err := writeIfChanged(path, hostSysctlContent(orig)); err != nil {
			g.log.Error("write host sysctl file", "error", err)
			return
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
		g.log.Error("remove host sysctl file", "error", err)
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

func writeIfChanged(path, content string) error {
	if b, err := os.ReadFile(path); err == nil && string(b) == content {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
