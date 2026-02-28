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

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestChecker_getSystemRequirements(t *testing.T) {
	tests := []struct {
		name     string
		checker  *Checker
		expected systemRequirements
	}{
		{
			name:    "default bundle requirements",
			checker: &Checker{installConfig: &config.DeckhouseInstaller{Bundle: config.DefaultBundle}},
			expected: systemRequirements{
				cpuCores:       defaultRequiredCPUCores,
				memoryMB:       defaultRequiredMemoryMB,
				rootDiskSizeGB: defaultRequiredRootDiskSizeGB,
			},
		},
		{
			name:    "minimal bundle requirements",
			checker: &Checker{installConfig: &config.DeckhouseInstaller{Bundle: config.MinimalBundle}},
			expected: systemRequirements{
				cpuCores:       minimalRequiredCPUCores,
				memoryMB:       minimalRequiredMemoryMB,
				rootDiskSizeGB: minimalRequiredRootDiskSizeGB,
			},
		},
		{
			name:    "empty bundle falls back to default requirements",
			checker: &Checker{installConfig: &config.DeckhouseInstaller{}},
			expected: systemRequirements{
				cpuCores:       defaultRequiredCPUCores,
				memoryMB:       defaultRequiredMemoryMB,
				rootDiskSizeGB: defaultRequiredRootDiskSizeGB,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.checker.getSystemRequirements())
		})
	}
}
