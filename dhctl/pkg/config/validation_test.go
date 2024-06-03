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
  name: empty kind`,
			errContains: `InvalidYAML: [0]: unmarshal: Object 'Kind' is missing in '{"apiVersion":"vendor.k8s.io/v1","metadata":{"name":"empty kind"}}'`,
		},
		"empty version": {
			config: `
kind: SomeKind
metadata:
  name: empty version`,
			errContains: `ValidationFailed: [0] "empty version": .apiVersion is required`,
		},
		"crd": {
			config: `
apiVersion: vendor.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: crd`,
			errContains: `ValidationFailed: [0] vendor.k8s.io/v1, Kind=CustomResourceDefinition "crd": got unacceptable resource kind: CustomResourceDefinition`,
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
			errContains: `ValidationFailed: exactly one "InitConfiguration" required`,
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
			errContains: `ValidationFailed: exactly one "InitConfiguration" required`,
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
			errContains: `ValidationFailed: [1] deckhouse.io/v1alpha1, Kind=ClusterConfiguration "deckhouse": "ClusterConfiguration, deckhouse.io/v1" document validation failed: 5 errors occurred:
	* .metadata is a forbidden property
	* .clusterType is required
	* .kubernetesVersion is required
	* .podSubnetCIDR is required
	* .serviceSubnetCIDR is required

; unknown kind, expected one of ("InitConfiguration", "ModuleConfig")`,
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
			errContains: `ValidationFailed: [0] deckhouse.io/v1alpha1, Kind=ModuleConfig "global": unknown kind, expected "ClusterConfiguration"
exactly one "ClusterConfiguration" required`,
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
			expected: ClusterConfig{
				ClusterType: "Static",
			},
			errContains: `ValidationFailed: [1] deckhouse.io/v1, Kind=SomeKind: schema not found: no schema for index SomeKind, deckhouse.io/v1; unknown kind, expected "ClusterConfiguration"`,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			clusterConfig, err := ValidateClusterConfiguration(tt.config, newStore, validateOpts...)
			require.Equal(t, tt.expected, clusterConfig)
			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
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
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
metadata:
    name: anotherProvider`,
			clusterConfig: ClusterConfig{
				ClusterType: "Cloud",
				Cloud: struct {
					Provider string `json:"provider"`
				}(struct{ Provider string }{
					Provider: "vSphere",
				}),
			},
			errContains: `exactly one "VsphereClusterConfiguration" required`,
		},
		"bad provider": {
			config: `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
metadata:
    name: badProvider`,
			clusterConfig: ClusterConfig{
				ClusterType: "Cloud",
				Cloud: struct {
					Provider string `json:"provider"`
				}(struct{ Provider string }{
					Provider: "badProvider",
				}),
			},
			errContains: `ValidationFailed: unknown cloud provider 'badProvider', check if 'ClusterConfiguration' is valid
[0] deckhouse.io/v1, Kind=YandexClusterConfiguration "badProvider": "YandexClusterConfiguration, deckhouse.io/v1" document validation failed: 5 errors occurred:
	* .metadata is a forbidden property
	* .masterNodeGroup is required
	* .nodeNetworkCIDR is required
	* .sshPublicKey is required
	* .provider is required`,
		},
		"empty provider": {
			config: `
apiVersion: deckhouse.io/v1
kind: SuperOpenStackClusterConfiguration
metadata:
    name: emptyProvider`,
			clusterConfig: ClusterConfig{
				ClusterType: "Cloud",
				Cloud: struct {
					Provider string `json:"provider"`
				}(struct{ Provider string }{
					Provider: "",
				}),
			},
			errContains: `ValidationFailed: unknown cloud provider '', check if 'ClusterConfiguration' is valid
[0] deckhouse.io/v1, Kind=SuperOpenStackClusterConfiguration "emptyProvider": schema not found: no schema for index SuperOpenStackClusterConfiguration, deckhouse.io/v1`,
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
			errContains: `ValidationFailed: [0] deckhouse.io/v1alpha1, Kind=ModuleConfig "global": unknown kind, expected "VsphereClusterConfiguration"
exactly one "VsphereClusterConfiguration" required`,
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
sshPublicKey: ssh-key`,
			clusterConfig: ClusterConfig{
				ClusterType: "Cloud",
				Cloud: struct {
					Provider string `json:"provider"`
				}(struct{ Provider string }{
					Provider: "Yandex",
				}),
			},
			errContains: `ValidationFailed: exactly one "YandexClusterConfiguration" required`,
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

func TestValidateStaticClusterConfiguration(t *testing.T) {
	t.Parallel()

	const schemasDir = "./../../../candi/openapi"
	newStore := newSchemaStore([]string{schemasDir})

	tests := map[string]struct {
		config      string
		errContains string
	}{
		"ok": {
			config: `
---
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
# address space for the cluster's internal network
internalNetworkCIDRs:
- 192.168.199.0/24
---
`,
		},
		"ok, empty": {
			config: ``,
		},
		"empty StaticClusterConfiguration": {
			config: `
apiVersion: deckhouse.io/v1alpha1
kind: StaticClusterConfiguration`,
		},
		"bad config": {
			config: `
apiVersion: deckhouse.io/v1alpha1
kind: StaticClusterConfiguration
someKey: someValue`,
			errContains: `ValidationFailed: [0] deckhouse.io/v1alpha1, Kind=StaticClusterConfiguration: "StaticClusterConfiguration, deckhouse.io/v1" document validation failed: 1 error occurred:
	* .someKey is a forbidden property

`,
		},
		"bad internalNetworkCIDRs": {
			config: `
apiVersion: deckhouse.io/v1alpha1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.199.0/24test`,
			errContains: `ValidationFailed: [0] deckhouse.io/v1alpha1, Kind=StaticClusterConfiguration: "StaticClusterConfiguration, deckhouse.io/v1" document validation failed: 1 error occurred:
	* internalNetworkCIDRs should match '^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])(\/(3[0-2]|[1-2][0-9]|[0-9]))$'

`,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := ValidateStaticClusterConfiguration(tt.config, newStore, validateOpts...)
			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

var validateOpts = []ValidateOption{ValidateOptionCommanderMode(true)}
