// Copyright 2024 Flant JSC
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

package config

import (
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/require"
)

func TestValidateResources(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config      string
		errContains string
	}{
		"ok": {
			config: `
---
apiVersion: vendor.k8s.io/v1
kind: SomeKind
metadata:
  name: ok
---
apiVersion: vendor.k8s.io/v2
kind: AnotherKind
metadata:
  name: ok
---`,
		},
		"empty kind": {
			config: `
apiVersion: vendor.k8s.io/v1
metadata:
  name: ok`,
			errContains: "'Kind' is missing",
		},
		"empty version": {
			config: `
kind: SomeKind
metadata:
  name: ok`,
			errContains: "no version information",
		},
		"crd": {
			config: `
apiVersion: vendor.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ok`,
			errContains: "got unacceptable resource kind",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := ValidateResources(tt.config, validateOpts...)
			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestValidateInitConfiguration(t *testing.T) {
	t.Parallel()

	const schemasDir = "./../../../candi/openapi"
	newStore := newSchemaStore([]string{schemasDir})
	newStore.moduleConfigsCache["deckhouse"] = &spec.Schema{}

	tests := map[string]struct {
		config      string
		errContains string
	}{
		"ok": {
			config: `
---
---
# https://deckhouse.ru/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  registryDockerCfg: eyJhdXRocyI6eyJyZWdpc3RyeS5kZWNraG91c2UuaW8iOnsiYXV0aCI6ImJHbGpaVzV6WlMxMGIydGxianBtZWxkeFMzZGxOR2s0VEU0ME5tUmtNbGxWTWxKWmJYTkNXVmcyV25sTVJ3PT0ifX19
  releaseChannel: Alpha
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    bundle: Default
    logLevel: Info
    releaseChannel: Alpha
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true
---
`,
		},
		"no init config": {
			config: `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true`,
			errContains: `"InitConfiguration" required`,
		},
		"multiple init configs": {
			config: `
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  registryDockerCfg: 1eyJhdXRocyI6eyJyZWdpc3RyeS5kZWNraG91c2UuaW8iOnsiYXV0aCI6ImJHbGpaVzV6WlMxMGIydGxianBtZWxkeFMzZGxOR2s0VEU0ME5tUmtNbGxWTWxKWmJYTkNXVmcyV25sTVJ3PT0ifX19
  releaseChannel: Alpha
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  registryDockerCfg: 2eyJhdXRocyI6eyJyZWdpc3RyeS5kZWNraG91c2UuaW8iOnsiYXV0aCI6ImJHbGpaVzV6WlMxMGIydGxianBtZWxkeFMzZGxOR2s0VEU0ME5tUmtNbGxWTWxKWmJYTkNXVmcyV25sTVJ3PT0ifX19
  releaseChannel: Stable`,
			errContains: `only one "InitConfiguration" expected`,
		},
		"extra kinds": {
			config: `
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  registryDockerCfg: eyJhdXRocyI6eyJyZWdpc3RyeS5kZWNraG91c2UuaW8iOnsiYXV0aCI6ImJHbGpaVzV6WlMxMGIydGxianBtZWxkeFMzZGxOR2s0VEU0ME5tUmtNbGxWTWxKWmJYTkNXVmcyV25sTVJ3PT0ifX19
  releaseChannel: Alpha
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
metadata:
  name: deckhouse
`,
			errContains: `unknown kind "ClusterConfiguration", expected one of ("InitConfiguration", "ModuleConfig")`,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := ValidateInitConfiguration(tt.config, newStore, validateOpts...)
			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestValidateClusterConfiguration(t *testing.T) {
	t.Parallel()

	const schemasDir = "./../../../candi/openapi"
	newStore := newSchemaStore([]string{schemasDir})

	tests := map[string]struct {
		config      string
		expected    ClusterConfig
		errContains string
	}{
		"ok, Static": {
			config: `
# https://deckhouse.ru/documentation/v1/installing/configuration.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
`,
			expected: ClusterConfig{
				ClusterType: "Static",
			},
		},
		"ok, Cloud": {
			config: `
# https://deckhouse.ru/documentation/v1/installing/configuration.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  # PARAMETER
  prefix: cmdr-test
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
`,
			expected: ClusterConfig{
				ClusterType: "Cloud",
				Cloud: struct {
					Provider string `json:"provider"`
				}(struct{ Provider string }{
					Provider: "Yandex",
				}),
			},
		},
		"no cluster config": {
			config: `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true`,
			errContains: `unknown kind "ModuleConfig", expected "InitConfiguration"`,
		},
		"extra kinds": {
			config: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1
kind: SomeKind
clusterType: Static
`,
			expected:    ClusterConfig{},
			errContains: `unknown kind "SomeKind", expected "InitConfiguration"`,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			clusterConfig, err := ValidateClusterConfiguration(tt.config, newStore, validateOpts...)
			if tt.errContains == "" {
				require.NoError(t, err)
				require.Equal(t, tt.expected, clusterConfig)
			} else {
				require.ErrorContains(t, err, tt.errContains)
				require.Empty(t, clusterConfig)
			}
		})
	}
}

func TestValidateProviderSpecificClusterConfiguration(t *testing.T) {
	t.Parallel()

	const schemasDir = "./../../../candi/cloud-providers"
	newStore := newSchemaStore([]string{schemasDir})

	tests := map[string]struct {
		config        string
		clusterConfig ClusterConfig
		errContains   string
	}{
		"ok": {
			config: `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
provider:
  cloudID: 'YjFnYnA2bHVybDBzbXA2Y2kzanMK'
  folderID: 'b1gsqe7ct9jtss0mlmid'
  serviceAccountJSON: |
    {"id": "ajeqlssun75pno7f46t7"}
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 8
    memory: 8192
    # https://cloud.yandex.ru/marketplace/products/yc/ubuntu-22-04-lts
    imageID: fd8li2lvvfc6bdj4c787
    externalIPAddresses:
    - "Auto"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: ssh-key
`,
			clusterConfig: ClusterConfig{
				ClusterType: "Cloud",
				Cloud: struct {
					Provider string `json:"provider"`
				}(struct{ Provider string }{
					Provider: "Yandex",
				}),
			},
		},
		"ok, empty for static": {
			config: ``,
			clusterConfig: ClusterConfig{
				ClusterType: "Static",
			},
		},
		"another provider": {
			config: `
version: deckhouse.io/v1
kind: OpenStackClusterConfiguration`,
			clusterConfig: ClusterConfig{
				ClusterType: "Cloud",
				Cloud: struct {
					Provider string `json:"provider"`
				}(struct{ Provider string }{
					Provider: "vSphere",
				}),
			},
			errContains: `unknown kind "OpenStackClusterConfiguration", expected "VsphereClusterConfiguration"`,
		},
		"no config": {
			config: `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true`,
			clusterConfig: ClusterConfig{
				ClusterType: "Cloud",
				Cloud: struct {
					Provider string `json:"provider"`
				}(struct{ Provider string }{
					Provider: "vSphere",
				}),
			},
			errContains: `unknown kind "ModuleConfig", expected "VsphereClusterConfiguration"`,
		},
		"extra provider": {
			config: `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
provider:
  cloudID: 'YjFnYnA2bHVybDBzbXA2Y2kzanMK'
  folderID: 'b1gsqe7ct9jtss0mlmid'
  serviceAccountJSON: |
    {"id": "ajeqlssun75pno7f46t7"}
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 8
    memory: 8192
    # https://cloud.yandex.ru/marketplace/products/yc/ubuntu-22-04-lts
    imageID: fd8li2lvvfc6bdj4c787
    externalIPAddresses:
    - "Auto"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: ssh-key
---
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration`,
			clusterConfig: ClusterConfig{
				ClusterType: "Cloud",
				Cloud: struct {
					Provider string `json:"provider"`
				}(struct{ Provider string }{
					Provider: "Yandex",
				}),
			},
			errContains: `only one "YandexClusterConfiguration" expected`,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := ValidateProviderSpecificClusterConfiguration(tt.config, tt.clusterConfig, newStore, validateOpts...)
			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

// deprecated
func TestValidateClusterSettingsFormat(t *testing.T) {
	once.Do(func() {
		store = newSchemaStore([]string{"./../../../candi/openapi"})
	})

	t.Run("ok", func(t *testing.T) {
		t.Run("cluster configuration", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(clusterConfigFormat, validateOpts...)
			require.NoError(t, err)
		})
		t.Run("resource-1", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(resourceFormat1, validateOpts...)
			require.NoError(t, err)
		})
		t.Run("resource-2", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(resourceFormat2, validateOpts...)
			require.NoError(t, err)
		})
		t.Run("cluster configuration with resource", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(clusterConfigWithResourcesFormat, validateOpts...)
			require.NoError(t, err)
		})
	})

	t.Run("not ok", func(t *testing.T) {
		t.Run("unexpected field", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(unknownFieldFormat, validateOpts...)
			require.Error(t, err)
		})
	})
}

var validateOpts = []ValidateOption{ValidateOptionCommanderMode(true)}

var (
	clusterConfigFormat = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: "cmdr-test-03051973"
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"`
	resourceFormat1 = `---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse-admin
spec:
  enabled: true`
	resourceFormat2 = `---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: some-name`
	clusterConfigWithResourcesFormat = clusterConfigFormat + "\n" + resourceFormat1
	unknownFieldFormat               = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: "cmdr-test-03051973"
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
unexpected: "fail"`
)
