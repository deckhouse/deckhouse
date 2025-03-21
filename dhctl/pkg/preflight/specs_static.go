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
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

func (pc *Checker) CheckStaticNodeSystemRequirements(ctx context.Context) error {
	if app.PreflightSkipSystemRequirementsCheck {
		log.DebugLn("System requirements check is skipped")
		return nil
	}

	ramKb, err := extractRAMCapacityFromNode(ctx, pc.nodeInterface)
	if err != nil {
		return err
	}

	coresCount, err := extractCPULogicalCoresCountFromNode(ctx, pc.nodeInterface)
	if err != nil {
		return err
	}

	failures := make([]string, 0)
	if coresCount < minimumRequiredCPUCores {
		failures = append(failures, fmt.Sprintf(
			" - System requirements mandate at least %d CPU(s) on the node, but it has %d",
			minimumRequiredCPUCores,
			coresCount,
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

func extractRAMCapacityFromNode(ctx context.Context, sshCl node.Interface) (int, error) {
	cmd := sshCl.Command("cat", "/proc/meminfo")
	memInfo, _, err := cmd.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("Failed to read MemTotal from /proc/meminfo: %w", err)
	}

	submatch := regexp.MustCompile(`^MemTotal:\s*(\d+)\s.B`).FindSubmatch(memInfo)
	ramKb, err := strconv.Atoi(string(submatch[1]))
	if err != nil {
		return 0, fmt.Errorf("Failed to parse MemTotal from /proc/meminfo: %w", err)
	}
	return ramKb, nil
}

func extractCPULogicalCoresCountFromNode(ctx context.Context, nodeInterface node.Interface) (int, error) {
	cmd := nodeInterface.Command("cat", "/proc/cpuinfo")
	stdout, _, err := cmd.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("Failed to read CPU info from /proc/cpuinfo: %w", err)
	}

	count, err := logicalCoresCountFromCPUInfo(stdout)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse CPU info from /proc/cpuinfo: %w", err)
	}
	return count, nil
}

func logicalCoresCountFromCPUInfo(cpuinfo []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(cpuinfo))
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
