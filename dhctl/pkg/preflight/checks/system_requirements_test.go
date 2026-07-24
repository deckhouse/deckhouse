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
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestSystemRequirementsForConfig(t *testing.T) {
	tests := []struct {
		name          string
		installConfig *config.DeckhouseInstaller
		expected      systemRequirements
	}{
		{
			name: "default bundle",
			installConfig: &config.DeckhouseInstaller{
				Bundle: config.DefaultBundle,
			},
			expected: systemRequirements{
				cpuCores:       4,
				memoryMB:       7680,
				rootDiskSizeGB: 50,
			},
		},
		{
			name: "minimal bundle",
			installConfig: &config.DeckhouseInstaller{
				Bundle: config.MinimalBundle,
			},
			expected: systemRequirements{
				cpuCores:       2,
				memoryMB:       3584,
				rootDiskSizeGB: 50,
			},
		},
		{
			name:          "nil config uses default requirements",
			installConfig: nil,
			expected: systemRequirements{
				cpuCores:       4,
				memoryMB:       7680,
				rootDiskSizeGB: 50,
			},
		},
		{
			name:          "empty bundle uses default requirements",
			installConfig: &config.DeckhouseInstaller{},
			expected: systemRequirements{
				cpuCores:       4,
				memoryMB:       7680,
				rootDiskSizeGB: 50,
			},
		},
		{
			name: "unknown bundle uses default requirements",
			installConfig: &config.DeckhouseInstaller{
				Bundle: "Unknown",
			},
			expected: systemRequirements{
				cpuCores:       4,
				memoryMB:       7680,
				rootDiskSizeGB: 50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := systemRequirementsForConfig(tt.installConfig)

			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestCloudSystemRequirementsMinimalBundlePassesAtMinimumResources(
	t *testing.T,
) {
	installConfig := &config.DeckhouseInstaller{
		Bundle: config.MinimalBundle,
		ProviderClusterConfig: []byte(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 2
    memory: 3584
`),
	}

	check := CloudSystemRequirementsCheck{
		InstallConfig: installConfig,
	}

	require.NoError(t, check.Run(context.Background()))
}

func TestCloudSystemRequirementsDefaultBundlePassesAtMinimumResources(
	t *testing.T,
) {
	installConfig := &config.DeckhouseInstaller{
		Bundle: config.DefaultBundle,
		ProviderClusterConfig: []byte(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 7680
`),
	}

	check := CloudSystemRequirementsCheck{
		InstallConfig: installConfig,
	}

	require.NoError(t, check.Run(context.Background()))
}

func TestCloudSystemRequirementsDefaultBundleRejectsMinimalMemory(
	t *testing.T,
) {
	installConfig := &config.DeckhouseInstaller{
		Bundle: config.DefaultBundle,
		ProviderClusterConfig: []byte(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 3584
`),
	}

	check := CloudSystemRequirementsCheck{
		InstallConfig: installConfig,
	}

	err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "RAM amount")
	require.Contains(
		t,
		err.Error(),
		"expected at least 7680, but 3584 is configured",
	)
}

func TestCloudSystemRequirementsDefaultBundleRejectsInsufficientCPU(
	t *testing.T,
) {
	installConfig := &config.DeckhouseInstaller{
		Bundle: config.DefaultBundle,
		ProviderClusterConfig: []byte(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 3
    memory: 7680
`),
	}

	check := CloudSystemRequirementsCheck{
		InstallConfig: installConfig,
	}

	err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "CPU cores count")
	require.Contains(
		t,
		err.Error(),
		"expected at least 4, but 3 is configured",
	)
}

func TestCloudSystemRequirementsMinimalBundleRejectsInsufficientCPU(
	t *testing.T,
) {
	installConfig := &config.DeckhouseInstaller{
		Bundle: config.MinimalBundle,
		ProviderClusterConfig: []byte(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 1
    memory: 3584
`),
	}

	check := CloudSystemRequirementsCheck{
		InstallConfig: installConfig,
	}

	err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "CPU cores count")
	require.Contains(
		t,
		err.Error(),
		"expected at least 2, but 1 is configured",
	)
}

func TestCloudSystemRequirementsMinimalBundleRejectsInsufficientMemory(
	t *testing.T,
) {
	installConfig := &config.DeckhouseInstaller{
		Bundle: config.MinimalBundle,
		ProviderClusterConfig: []byte(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 2
    memory: 3583
`),
	}

	check := CloudSystemRequirementsCheck{
		InstallConfig: installConfig,
	}

	err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "RAM amount")
	require.Contains(
		t,
		err.Error(),
		"expected at least 3584, but 3583 is configured",
	)
}

func TestCloudSystemRequirementsMinimalBundleRejectsSmallExplicitRootDisk(
	t *testing.T,
) {
	installConfig := &config.DeckhouseInstaller{
		Bundle: config.MinimalBundle,
		ProviderClusterConfig: []byte(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 2
    memory: 3584
    diskSizeGB: 49
`),
	}

	check := CloudSystemRequirementsCheck{
		InstallConfig: installConfig,
	}

	err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "Root disk capacity")
	require.Contains(
		t,
		err.Error(),
		"expected at least 50, but 49 is configured",
	)
}

func TestCloudSystemRequirementsNilConfig(t *testing.T) {
	check := CloudSystemRequirementsCheck{
		InstallConfig: nil,
	}

	require.NoError(t, check.Run(context.Background()))
}

func TestCloudSystemRequirementsWithoutProviderClusterConfig(
	t *testing.T,
) {
	check := CloudSystemRequirementsCheck{
		InstallConfig: &config.DeckhouseInstaller{
			Bundle: config.MinimalBundle,
		},
	}

	require.NoError(t, check.Run(context.Background()))
}
