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
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

func TestValidateResources(t *testing.T) {
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
		t.Run(name, func(t *testing.T) {
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
	const schemasDir = "./../../../candi/openapi"
	const deckhouseSchemasDir = "./../../../modules/002-deckhouse/openapi"
	newStore := newSchemaStore(nil, []string{schemasDir, deckhouseSchemasDir})

	tests := map[string]struct {
		config      string
		errContains string
	}{
		"ok": {
			config: `
---
---
# https://deckhouse.ru/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ce
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
  imagesRepo: registry.deckhouse.io/deckhouse/ce
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ce`,
			errContains: `ValidationFailed: exactly one "InitConfiguration" required`,
		},
		"extra kinds": {
			config: `
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ce
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
		t.Run(name, func(t *testing.T) {
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
	const schemasDir = "./../../../candi/openapi"
	newStore := newSchemaStore(&options.New().Global, []string{schemasDir})

	tests := map[string]struct {
		config      string
		expected    ClusterConfig
		errContains string
	}{
		"ok, Static": {
			config: `
# https://deckhouse.ru/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration
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
# https://deckhouse.ru/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration
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
			errContains: `ValidationFailed: [1] deckhouse.io/v1, Kind=SomeKind: schema not found; unknown kind, expected "ClusterConfiguration"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
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
	// CI builds candi/cloud-providers from modules/030-cloud-provider-*
	// (see tools/build_includes/candi-cloud-providers-CE.yaml); skip locally
	// when the prepared tree is not materialised.
	const schemasDir = "./../../../candi/cloud-providers"
	if info, err := os.Stat(schemasDir); err != nil || !info.IsDir() {
		t.Skipf("%s not present; run `make test` after werf bundles cloud-providers, or skip", schemasDir)
	}
	newStore := newSchemaStore(&options.New().Global, []string{schemasDir})

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
[0] deckhouse.io/v1, Kind=SuperOpenStackClusterConfiguration "emptyProvider": schema not found`,
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
		t.Run(name, func(t *testing.T) {
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
	const schemasDir = "./../../../candi/openapi"
	newStore := newSchemaStore(&options.New().Global, []string{schemasDir})

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
		"ok, IPv6": {
			config: `
---
apiVersion: deckhouse.io/v1alpha1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- fd00:10:42::/64
---
`,
		},
		"ok, dual-stack": {
			config: `
---
apiVersion: deckhouse.io/v1alpha1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.199.0/24
- fd00:10:42::/64
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
	* internalNetworkCIDRs must be of type cidr: "192.168.199.0/24test"

`,
		},
		"bad internalNetworkCIDRs, IPv4 missing mask": {
			config: `
apiVersion: deckhouse.io/v1alpha1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.199.0`,
			errContains: `internalNetworkCIDRs must be of type cidr: "192.168.199.0"`,
		},
		"bad internalNetworkCIDRs, malformed IPv6": {
			config: `
apiVersion: deckhouse.io/v1alpha1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- fd00::1::/64`,
			errContains: `internalNetworkCIDRs must be of type cidr: "fd00::1::/64"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
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

func TestValidationError_Append_SetsReasonAndTopLevelKind(t *testing.T) {
	ve := &ValidationError{}
	ve.Append(ErrKindValidationFailed, Error{Messages: []string{"bad apiVersion"}})

	require.Equal(t, ErrKindValidationFailed, ve.Kind)
	require.Len(t, ve.Errors, 1)
	require.Equal(t, ErrKindValidationFailed, ve.Errors[0].Reason)
}

// Domain-specific reasons (CNI*) bucket to ValidationFailed at the top level.
// Per-Error Reason retains the precise kind for fine-grained rendering.
func TestValidationError_Append_DomainReasonBucketsToValidationFailed(t *testing.T) {
	ve := &ValidationError{}
	ve.Append(ErrKindCNIMismatch, Error{Messages: []string{"cni mismatch"}})

	require.Equal(t, ErrKindValidationFailed, ve.Kind, "top-level must bucket CNI* into ValidationFailed")
	require.Equal(t, ErrKindCNIMismatch, ve.Errors[0].Reason, "per-Error Reason must keep the precise kind")
}

func TestValidationError_Append_CNISettingsBucketsToValidationFailed(t *testing.T) {
	ve := &ValidationError{}
	ve.Append(ErrKindCNISettingsMismatch, Error{Messages: []string{"settings mismatch"}})

	require.Equal(t, ErrKindValidationFailed, ve.Kind)
	require.Equal(t, ErrKindCNISettingsMismatch, ve.Errors[0].Reason)
}

func TestValidationError_Append_MixedReasons(t *testing.T) {
	ve := &ValidationError{}
	ve.Append(ErrKindCNIMismatch, Error{Messages: []string{"cni"}})
	ve.Append(ErrKindValidationFailed, Error{Messages: []string{"bad"}})
	ve.Append(ErrKindCNISettingsMismatch, Error{Messages: []string{"cni s"}})

	require.Equal(t, ErrKindValidationFailed, ve.Kind, "all reasons bucket to ValidationFailed → top-level is ValidationFailed")
	require.Equal(t, ErrKindCNIMismatch, ve.Errors[0].Reason)
	require.Equal(t, ErrKindValidationFailed, ve.Errors[1].Reason)
	require.Equal(t, ErrKindCNISettingsMismatch, ve.Errors[2].Reason)
}

// InvalidYAML (legacy, value 3) wins over CNI (bucket to ValidationFailed,
// value 2) on top-level via plain max. Preserves stop-semantics for clients
// that key on InvalidYAML.
func TestValidationError_Append_InvalidYAMLWinsOverCNI(t *testing.T) {
	ve := &ValidationError{}
	ve.Append(ErrKindCNIMismatch, Error{Messages: []string{"cni"}})
	ve.Append(ErrKindInvalidYAML, Error{Messages: []string{"bad yaml"}})

	require.Equal(t, ErrKindInvalidYAML, ve.Kind)

	// Order-invariance.
	ve2 := &ValidationError{}
	ve2.Append(ErrKindInvalidYAML, Error{Messages: []string{"bad yaml"}})
	ve2.Append(ErrKindCNIMismatch, Error{Messages: []string{"cni"}})
	require.Equal(t, ErrKindInvalidYAML, ve2.Kind)
}

func TestValidationError_Append_KindMonotonicity(t *testing.T) {
	ve := &ValidationError{}
	ve.Append(ErrKindValidationFailed, Error{Messages: []string{"a"}})
	ve.Append(ErrKindChangesValidationFailed, Error{Messages: []string{"b"}})

	require.Equal(t, ErrKindValidationFailed, ve.Kind, "lower reason must not lower top-level Kind")
}

func TestValidationError_Merge(t *testing.T) {
	a := &ValidationError{}
	a.Append(ErrKindValidationFailed, Error{Messages: []string{"a1"}})

	b := &ValidationError{}
	b.Append(ErrKindCNIMismatch, Error{Messages: []string{"b1"}})
	b.Append(ErrKindInvalidYAML, Error{Messages: []string{"b2"}})

	a.Merge(b)

	require.Equal(t, ErrKindInvalidYAML, a.Kind)
	require.Len(t, a.Errors, 3)
	require.Equal(t, ErrKindValidationFailed, a.Errors[0].Reason)
	require.Equal(t, ErrKindCNIMismatch, a.Errors[1].Reason)
	require.Equal(t, ErrKindInvalidYAML, a.Errors[2].Reason)
}

func TestValidationError_Merge_NilNoop(t *testing.T) {
	a := &ValidationError{}
	a.Append(ErrKindValidationFailed, Error{Messages: []string{"a"}})
	a.Merge(nil)
	require.Len(t, a.Errors, 1)
	require.Equal(t, ErrKindValidationFailed, a.Kind)
}

func TestError_JSONOmitemptyAllFields(t *testing.T) {
	// All fields are omitempty: zero values disappear from the wire.
	// Keeps CNI/domain errors compact (no Index/Group/Version/Kind/Name noise).
	e := Error{Index: nil, Group: "", Version: "", Kind: "", Name: "", Messages: nil}
	b, err := json.Marshal(e)
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(b))
}

func TestError_JSONOmitemptyKeepsNonZeroFields(t *testing.T) {
	e := Error{
		Reason:   ErrKindValidationFailed,
		Index:    new(2),
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Kind:     "ModuleConfig",
		Name:     "cni-cilium",
		Messages: []string{"bad"},
	}
	b, err := json.Marshal(e)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"Reason": 2,
		"Index": 2,
		"Group": "deckhouse.io",
		"Version": "v1alpha1",
		"Kind": "ModuleConfig",
		"Name": "cni-cilium",
		"Messages": ["bad"]
	}`, string(b))
}

func TestError_JSON_CNIErrorCarriesResourceIdentity(t *testing.T) {
	// CNI errors point at the offending user MC: GVK + Name are populated
	// so commander UI can navigate to the resource. Index is unset (we don't
	// re-derive doc position in the filtered payload).
	ve := &ValidationError{}
	ve.Append(ErrKindCNIMismatch, Error{
		Group:    ModuleConfigGroup,
		Version:  ModuleConfigVersion,
		Kind:     ModuleConfigKind,
		Name:     "cni-simple-bridge",
		Messages: []string{`user configured "cni-simple-bridge", provider recommends "cni-cilium"`},
	})
	b, err := json.Marshal(ve)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"Kind": 2,
		"Errors": [
			{
				"Reason": 4,
				"Group": "deckhouse.io",
				"Version": "v1alpha1",
				"Kind": "ModuleConfig",
				"Name": "cni-simple-bridge",
				"Messages": ["user configured \"cni-simple-bridge\", provider recommends \"cni-cilium\""]
			}
		]
	}`, string(b))
}
