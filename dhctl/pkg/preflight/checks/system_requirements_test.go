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
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
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

	_, err := check.Run(context.Background())
	require.NoError(t, err)
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

	_, err := check.Run(context.Background())
	require.NoError(t, err)
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

	_, err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "memory: 3584 MB configured")
	require.Contains(t, err.Error(), "at least 7680 MB required")
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

	_, err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "masterNodeGroup.instanceClass.cores: 3 configured, at least 4 required")
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

	_, err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "masterNodeGroup.instanceClass.cores: 1 configured, at least 2 required")
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

	_, err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "masterNodeGroup.instanceClass.memory: 3583 MB configured, at least 3584 MB required")
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

	_, err := check.Run(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "masterNodeGroup.instanceClass.diskSizeGB: 49 GB configured, at least 50 GB required")
}

func TestCloudSystemRequirementsNilConfig(t *testing.T) {
	check := CloudSystemRequirementsCheck{
		InstallConfig: nil,
	}

	_, err := check.Run(context.Background())
	require.ErrorIs(t, err, preflight.ErrNotApplicable)
}

func TestCloudSystemRequirementsWithoutProviderClusterConfig(
	t *testing.T,
) {
	check := CloudSystemRequirementsCheck{
		InstallConfig: &config.DeckhouseInstaller{
			Bundle: config.MinimalBundle,
		},
	}

	_, err := check.Run(context.Background())
	require.ErrorIs(t, err, preflight.ErrNotApplicable)
}
