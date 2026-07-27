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

import "github.com/deckhouse/deckhouse/dhctl/pkg/config"

const (
	reservedMemoryThresholdMB = 512

	minimumRequiredCPUCores       = 4
	minimumRequiredMemoryMB       = 8192 - reservedMemoryThresholdMB
	minimumRequiredRootDiskSizeGB = 50

	minimalBundleRequiredCPUCores = 2
	minimalBundleRequiredMemoryMB = 4096 - reservedMemoryThresholdMB
)

type systemRequirements struct {
	cpuCores       int
	memoryMB       int
	rootDiskSizeGB int
}

func systemRequirementsForConfig(installConfig *config.DeckhouseInstaller) systemRequirements {
	requirements := systemRequirements{
		cpuCores:       minimumRequiredCPUCores,
		memoryMB:       minimumRequiredMemoryMB,
		rootDiskSizeGB: minimumRequiredRootDiskSizeGB,
	}

	if installConfig != nil && installConfig.Bundle == config.MinimalBundle {
		requirements.cpuCores = minimalBundleRequiredCPUCores
		requirements.memoryMB = minimalBundleRequiredMemoryMB
	}

	return requirements
}
