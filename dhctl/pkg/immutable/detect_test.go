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

package immutable

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestIsImmutableMaster(t *testing.T) {
	nodeGroup := func(systemType string) map[string]any {
		spec := map[string]any{"nodeType": "CloudPermanent"}
		if systemType != "" {
			spec["systemType"] = systemType
		}
		return map[string]any{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"spec":       spec,
		}
	}

	tests := []struct {
		name       string
		nodeGroups map[string]map[string]any
		want       bool
	}{
		{name: "no resources at all"},
		{
			name:       "immutable master among other groups",
			nodeGroups: map[string]map[string]any{"master": nodeGroup("Immutable"), "worker": nodeGroup("")},
			want:       true,
		},
		{
			name:       "master without systemType",
			nodeGroups: map[string]map[string]any{"master": nodeGroup("")},
		},
		{
			name:       "master asking for a mutable system",
			nodeGroups: map[string]map[string]any{"master": nodeGroup("Mutable")},
		},
		{
			// Only the master decides how the first node is created.
			name:       "only an immutable worker",
			nodeGroups: map[string]map[string]any{"worker": nodeGroup("Immutable")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaConfig := &config.MetaConfig{}
			if tt.nodeGroups != nil {
				metaConfig.CloudProviderVars = &config.CloudProviderVars{NodeGroups: tt.nodeGroups}
			}

			isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)
			require.NoError(t, err)
			require.Equal(t, tt.want, isImmutable)
		})
	}

	t.Run("a config nothing has parsed yet", func(t *testing.T) {
		isImmutable, err := IsImmutableMaster(t.Context(), &config.MetaConfig{})
		require.NoError(t, err)
		require.False(t, isImmutable)
	})
}

func TestIsImmutableMasterReadsResourcesOfAStaticCluster(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.StaticClusterType,
		ResourcesYAML: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
`,
	}

	isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)
	require.NoError(t, err)
	require.True(t, isImmutable)
}

func TestIsImmutableMasterIgnoresOtherGroups(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.StaticClusterType,
		ResourcesYAML: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  systemType: Immutable
`,
	}

	isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)
	require.NoError(t, err)
	require.False(t, isImmutable)
}

// The master comes last, behind a foreign document of the same name: a walk that
// stops at the first NodeGroup, or that reads any kind, misses the real group.
func TestIsImmutableMasterReadsAMasterDeclaredLast(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.StaticClusterType,
		ResourcesYAML: `
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master
spec:
  systemType: Mutable
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
`,
	}

	isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)
	require.NoError(t, err)
	require.True(t, isImmutable)
}

// NodeGroupConfiguration is a real deckhouse.io kind, so a document of the right
// group and the right name can still be the wrong object. Declared first, it
// masks the master unless the kind is matched too.
func TestIsImmutableMasterIgnoresAnotherKindOfTheSameGroup(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.StaticClusterType,
		ResourcesYAML: `
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: master
spec:
  weight: 100
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
`,
	}

	isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)
	require.NoError(t, err)
	require.True(t, isImmutable)
}

// A NodeGroup of a foreign API group, named master and declared first, must not
// mask the real one: the walk returns on its first match, so matching on kind
// and name alone reads the wrong document and reports no immutable master.
func TestIsImmutableMasterIgnoresAForeignGroupMaskingTheMaster(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.StaticClusterType,
		ResourcesYAML: `
apiVersion: example.com/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
`,
	}

	isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)
	require.NoError(t, err)
	require.True(t, isImmutable)
}

// The immutable worker comes first on purpose: a walk that returns the first
// NodeGroup it meets instead of the master one passes every other case here.
func TestIsImmutableMasterIgnoresAnImmutableWorkerBeforeTheMaster(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.StaticClusterType,
		ResourcesYAML: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  systemType: Immutable
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
`,
	}

	isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)
	require.NoError(t, err)
	require.False(t, isImmutable)
}

// A static cluster runs no BaseInfra phase, so nothing reports the master's
// address and the handoff has nothing to dial unless the operator names it.
// Refused before anything is created, so every rerun dies the same way.
func TestValidateInputsRefusesAStaticClusterWithoutHosts(t *testing.T) {
	metaConfig := &config.MetaConfig{ClusterType: config.StaticClusterType}

	err := ValidateInputs(t.Context(), metaConfig, nil)

	require.ErrorContains(t, err, "--master-host")
}

func TestValidateInputsAcceptsAStaticClusterWithHosts(t *testing.T) {
	metaConfig := &config.MetaConfig{ClusterType: config.StaticClusterType}

	err := ValidateInputs(t.Context(), metaConfig, map[string]string{"master-0": "10.0.0.11"})

	require.NoError(t, err)
}

func TestValidateInputsRefusesHostsInACloudCluster(t *testing.T) {
	metaConfig := &config.MetaConfig{ClusterType: config.CloudClusterType}

	err := ValidateInputs(t.Context(), metaConfig, map[string]string{"master-0": "10.0.0.11"})

	require.ErrorContains(t, err, "cloud infrastructure reports")
}

func TestValidateInputsAcceptsACloudClusterWithoutHosts(t *testing.T) {
	metaConfig := &config.MetaConfig{ClusterType: config.CloudClusterType}

	require.NoError(t, ValidateInputs(t.Context(), metaConfig, nil))
}

func TestParseHostsSplitsNameAndAddress(t *testing.T) {
	hosts, err := ParseHosts([]string{"master-0=10.0.0.11", "master-1=10.0.0.12"})

	require.NoError(t, err)
	require.Equal(t, map[string]string{"master-0": "10.0.0.11", "master-1": "10.0.0.12"}, hosts)
}

// kingpin splits DHCTL_CLI_MASTER_HOSTS on newlines and trims only the trailing
// one, so an indented multi-line envar reaches ParseHosts with the indentation
// still on the node name, where it fails much later and much less clearly.
func TestParseHostsTrimsEnvarIndentation(t *testing.T) {
	hosts, err := ParseHosts([]string{"  master-0=10.0.0.11", "\tmaster-1=10.0.0.12  "})

	require.NoError(t, err)
	require.Equal(t, map[string]string{"master-0": "10.0.0.11", "master-1": "10.0.0.12"}, hosts)
}

// The spaces an operator puts around the "=" are not part of either half, and
// the whole path keys off the node name: a name with a space on it is a node
// kubelet never registers under and nothing ever finds.
func TestParseHostsTrimsAroundTheSeparator(t *testing.T) {
	hosts, err := ParseHosts([]string{"master-0 = 10.0.0.11"})

	require.NoError(t, err)
	require.Equal(t, map[string]string{"master-0": "10.0.0.11"}, hosts)
}

func TestParseHostsRefusesAMissingName(t *testing.T) {
	_, err := ParseHosts([]string{"10.0.0.11"})

	require.ErrorContains(t, err, "<node-name>=<address>")
}

func TestParseHostsRefusesADuplicate(t *testing.T) {
	_, err := ParseHosts([]string{"master-0=10.0.0.11", "master-0=10.0.0.12"})

	require.ErrorContains(t, err, "master-0")
}

// config/base.go admits a document with a duplicated root-level kind into the
// resources, and a walk that skips what it cannot index reads no systemType
// there: the bootstrap then takes the SSH path against a machine with no sshd.
func TestIsImmutableMasterRefusesAnAmbiguousDocument(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.StaticClusterType,
		ResourcesYAML: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
`,
	}

	_, err := IsImmutableMaster(t.Context(), metaConfig)

	require.ErrorContains(t, err, "resource document 1",
		"the refusal has to name the document the operator has to fix")
	require.ErrorContains(t, err, `key "kind" already set in map`,
		"and say what is wrong with it")
}

// The other half of that refusal: SplitYAML hands over every chunk of the
// resources, and one that is no resource at all — a comment, a stray fragment —
// says nothing about the master and must not end a bootstrap.
func TestIsImmutableMasterReadsPastWhatIsNotAResource(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.StaticClusterType,
		ResourcesYAML: `
# a comment nobody parses
---
notAResource: true
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
`,
	}

	isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)

	require.NoError(t, err)
	require.True(t, isImmutable)
}

// IsImmutableMaster runs on every bootstrap before any gate, so a duplicated
// mapping key in a document that is not the master NodeGroup — the kind a
// hand-written resources.yml has carried for years — must not end one.
func TestIsImmutableMasterReadsPastADuplicatedKeyInAnotherDocument(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.StaticClusterType,
		ResourcesYAML: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: something
metadata:
  name: something
data:
  key: value
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
`,
	}

	isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)

	require.NoError(t, err, "a document that says nothing about the master must not fail an unrelated bootstrap")
	require.False(t, isImmutable)
}

// A cloud bootstrap has the master group parsed into CloudProviderVars, which is
// the answer: the raw resources are the static cluster's fallback, and reading
// them anyway makes any malformed document there end a cloud bootstrap.
func TestIsImmutableMasterAnswersACloudBootstrapFromCloudProviderVars(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType: config.CloudClusterType,
		CloudProviderVars: &config.CloudProviderVars{
			NodeGroups: map[string]map[string]any{
				"master": {
					"apiVersion": "deckhouse.io/v1",
					"kind":       "NodeGroup",
					"spec":       map[string]any{"nodeType": "CloudPermanent"},
				},
			},
		},
		ResourcesYAML: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: something
metadata:
  name: something
`,
	}

	isImmutable, err := IsImmutableMaster(t.Context(), metaConfig)

	require.NoError(t, err)
	require.False(t, isImmutable)
}
