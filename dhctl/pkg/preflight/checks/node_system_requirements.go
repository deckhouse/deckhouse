// Copyright 2026 Flant JSC
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

package checks

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type NodeSystemRequirementsCheck struct {
	// NodeInterface resolves the connection at the moment the check runs, the way every other
	// node check does — see NodeInterfaceFunc. This one held the initializer instead, which
	// made it the one node check that could not be run against a fake.
	NodeInterface NodeInterfaceFunc
	InstallConfig *config.DeckhouseInstaller
}

const NodeSystemRequirementsCheckName preflight.CheckName = "node-system-requirements"

func (NodeSystemRequirementsCheck) Description() string {
	return "the node has the CPU and RAM Deckhouse needs"
}

func (NodeSystemRequirementsCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeSystemRequirementsCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeSystemRequirementsCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	ramKb, err := extractRAMCapacityFromNode(ctx, nodeInterface)
	if err != nil {
		return "", scriptFailure("read /proc/meminfo", nodeInterface, nil, err)
	}

	coresCount, err := extractCPULogicalCoresCountFromNode(ctx, nodeInterface)
	if err != nil {
		return "", scriptFailure("read /proc/cpuinfo", nodeInterface, nil, err)
	}

	requirements := systemRequirementsForConfig(c.InstallConfig)

	var violations []string
	if coresCount < requirements.cpuCores {
		violations = append(violations, fmt.Sprintf("%d CPU, at least %d required", coresCount, requirements.cpuCores))
	}
	if ramKb < requirements.memoryMB*1024 {
		violations = append(violations, fmt.Sprintf("%d MiB of RAM, at least %d MiB required (%d GB minus %d MiB tolerance)",
			ramKb/1024, requirements.memoryMB, (requirements.memoryMB+reservedMemoryThresholdMB)/1024, reservedMemoryThresholdMB))
	}

	if len(violations) > 0 {
		// Hardware does not grow between two attempts of the same check.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("/proc/cpuinfo and /proc/meminfo on %s", host),
			Observed: "- " + strings.Join(violations, "\n- "),
			Expected: fmt.Sprintf("at least %d CPU and %d MiB of RAM", requirements.cpuCores, requirements.memoryMB),
			Fix:      "give the machine more CPU and RAM, or set bundle: Minimal in the \"deckhouse\" ModuleConfig",
		})
	}

	return fmt.Sprintf("%s has %d CPU and %d MiB of RAM", host, coresCount, ramKb/1024), nil
}

func extractRAMCapacityFromNode(ctx context.Context, nodeInterface libcon.Interface) (int, error) {
	cmd := nodeInterface.Command("cat", "/proc/meminfo")
	memInfo, _, err := cmd.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("cat /proc/meminfo: %w", err)
	}

	submatch := regexp.MustCompile(`^MemTotal:\s*(\d+)\s.B`).FindSubmatch(memInfo)
	if len(submatch) < 2 {
		return 0, fmt.Errorf("/proc/meminfo has no MemTotal line")
	}
	ramKb, err := strconv.Atoi(string(submatch[1]))
	if err != nil {
		return 0, fmt.Errorf("MemTotal in /proc/meminfo is not a number: %w", err)
	}
	return ramKb, nil
}

func extractCPULogicalCoresCountFromNode(ctx context.Context, nodeInterface libcon.Interface) (int, error) {
	cmd := nodeInterface.Command("cat", "/proc/cpuinfo")
	stdout, _, err := cmd.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("cat /proc/cpuinfo: %w", err)
	}

	count, err := logicalCoresCountFromCPUInfo(stdout)
	if err != nil {
		return 0, fmt.Errorf("count the processors in /proc/cpuinfo: %w", err)
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
		return 0, fmt.Errorf("scan /proc/cpuinfo: %w", err)
	}

	return len(processors), nil
}

func NodeSystemRequirements(
	nodeInterface NodeInterfaceFunc,
	installConfig *config.DeckhouseInstaller,
) preflight.Check {
	check := NodeSystemRequirementsCheck{
		NodeInterface: nodeInterface,
		InstallConfig: installConfig,
	}

	return preflight.Check{
		Name:        NodeSystemRequirementsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
