// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

func (pc *Checker) CheckStaticNodeSystemRequirements() error {
	if app.PreflightSkipSystemRequirementsCheck {
		log.DebugLn("System requirements check is skipped")
		return nil
	}

	buf := &bytes.Buffer{}
	ramKb, err := extractRAMCapacityFromNode(pc.sshClient, buf)
	if err != nil {
		return err
	}

	buf.Reset()
	physicalCoresCount, err := extractCPULogicalCoresCountFromNode(pc.sshClient, buf)
	if err != nil {
		return err
	}

	failures := make([]string, 0)
	if physicalCoresCount < minimumRequiredCPUCores {
		failures = append(failures, fmt.Sprintf(
			" - System requirements mandate at least %d CPU(s) on the node, but it has %d",
			minimumRequiredCPUCores,
			physicalCoresCount,
		))
	}

	if ramKb < minimumRequiredMemoryMB*1024 {
		failures = append(failures, fmt.Sprintf(
			" - System requirements mandate at least %d MiB of RAM on the node, but it has %d MiB",
			minimumRequiredMemoryMB,
			ramKb/1024,
		))
	}

	if len(failures) > 0 {
		return fmt.Errorf("Deckhouse system requirements are not met by your current configuration:\n%s", strings.Join(failures, ";\n"))
	}

	return nil
}

func extractRAMCapacityFromNode(sshCl *ssh.Client, buf *bytes.Buffer) (int, error) {
	err := sshCl.Command(`cat /proc/meminfo | grep MemTotal | awk '{print $2}' | tr -d "\n"`).
		CaptureStdout(buf).
		Run()
	if err != nil {
		return 0, fmt.Errorf("Failed to read MemTotal from /proc/meminfo: %w", err)
	}

	ramKb, err := strconv.Atoi(buf.String())
	if err != nil {
		return 0, fmt.Errorf("Failed to parse MemTotal from /proc/meminfo: %w", err)
	}
	return ramKb, nil
}

func extractCPULogicalCoresCountFromNode(sshCl *ssh.Client, buf *bytes.Buffer) (int, error) {
	err := sshCl.Command("cat", "/proc/cpuinfo").CaptureStdout(buf).Run()
	if err != nil {
		return 0, fmt.Errorf("Failed to read CPU info from /proc/cpuinfo: %w", err)
	}

	count, err := logicalCoresCountFromCPUInfo(buf)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse CPU info from /proc/cpuinfo: %w", err)
	}
	return count, nil
}

func logicalCoresCountFromCPUInfo(info *bytes.Buffer) (int, error) {
	scanner := bufio.NewScanner(info)
	processors := make(map[string]struct{})
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}

		field := strings.SplitN(line, ": ", 2)
		if strings.TrimSpace(field[0]) == "processor" {
			processors[strings.TrimSpace(field[1])] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("Failed to parse cpu info from /proc/cpuinfo: %w", err)
	}

	return len(processors), nil
}
