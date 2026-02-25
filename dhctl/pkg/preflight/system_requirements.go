// Copyright 2026 Flant JSC
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

import "github.com/deckhouse/deckhouse/dhctl/pkg/config"

const (
	// System requirements for Deckhouse Default bundle
	defaultRequiredCPUCores       = 4
	defaultRequiredMemoryMB       = 8192 - reservedMemoryThresholdMB
	defaultRequiredRootDiskSizeGB = 50

	// System requirements for Deckhouse Minimal bundle
	minimalRequiredCPUCores       = 2
	minimalRequiredMemoryMB       = 4096 - reservedMemoryThresholdMB
	minimalRequiredRootDiskSizeGB = 30

	reservedMemoryThresholdMB = 512
)

type systemRequirements struct {
	cpuCores       int
	memoryMB       int
	rootDiskSizeGB int
}

func (pc *Checker) getSystemRequirements() systemRequirements {
	switch pc.installConfig.Bundle {
	case config.MinimalBundle:
		return systemRequirements{
			cpuCores:       minimalRequiredCPUCores,
			memoryMB:       minimalRequiredMemoryMB,
			rootDiskSizeGB: minimalRequiredRootDiskSizeGB,
		}
	default:
		return systemRequirements{
			cpuCores:       defaultRequiredCPUCores,
			memoryMB:       defaultRequiredMemoryMB,
			rootDiskSizeGB: defaultRequiredRootDiskSizeGB,
		}
	}
}
