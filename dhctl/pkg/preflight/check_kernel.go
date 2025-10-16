// Copyright 2025 Flant JSC
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
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// CVE-2025-37999 impacts Linux kernels 6.12.0–6.12.28 and 6.14.0–6.14.6
func (pc *Checker) CheckKernelEROFSCVE(ctx context.Context) error {
	if app.PreflightSkipKernelEROFSCVE {
		log.InfoLn("Kernel EROFS CVE preflight check was skipped (via skip flag)")
		return nil
	}

	cmd := pc.nodeInterface.Command("uname", "-r")
	stdout, _, err := cmd.Output(ctx)
	if err != nil {
		log.InfoF("Kernel EROFS CVE preflight check was skipped, cannot read kernel version: %v\n", err)
		return nil
	}

	fullVersion := strings.TrimSpace(string(stdout))
	baseVersion := strings.SplitN(fullVersion, "-", 2)[0]

	if isKernelEROFSCVEVulnerable(baseVersion) {
		return fmt.Errorf("linux kernel %s is affected by CVE-2025-37999 (erofs); please upgrade to a fixed version 6.12.29 or newer, 6.14.7 or newer, or 6.15 or newer", fullVersion)
	}

	return nil
}

func isKernelEROFSCVEVulnerable(version string) bool {
	major, minor, patch := parseKernelVersion(version)
	if major == 6 {
		if minor == 12 && patch >= 0 && patch < 29 {
			return true
		}
		if minor == 14 && patch >= 0 && patch < 7 {
			return true
		}
	}
	return false
}

func parseKernelVersion(v string) (int, int, int) {
	parts := strings.Split(v, ".")
	var (
		major, minor, patch int
	)
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		patch, _ = strconv.Atoi(parts[2])
	}
	return major, minor, patch
}
