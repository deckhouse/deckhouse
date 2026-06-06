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

package external

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
)

func TestEncodeResourcesYAML_Deterministic(t *testing.T) {
	cv := &providerdata.CloudProviderVars{
		NodeGroups: map[string]map[string]interface{}{
			"b": {"apiVersion": "deckhouse.io/v1", "kind": "NodeGroup", "metadata": map[string]interface{}{"name": "b"}, "spec": map[string]interface{}{"replicas": 1}},
			"a": {"apiVersion": "deckhouse.io/v1", "kind": "NodeGroup", "metadata": map[string]interface{}{"name": "a"}, "spec": map[string]interface{}{"replicas": 2}},
		},
		InstanceClasses: map[string]map[string]interface{}{
			"z": {"apiVersion": "deckhouse.io/v1", "kind": "DVPInstanceClass", "metadata": map[string]interface{}{"name": "z"}},
			"m": {"apiVersion": "deckhouse.io/v1", "kind": "DVPInstanceClass", "metadata": map[string]interface{}{"name": "m"}},
		},
		Secrets: map[string]map[string]interface{}{
			"d8-x/cloud-credentials": {"apiVersion": "v1", "kind": "Secret", "metadata": map[string]interface{}{"name": "cloud-credentials", "namespace": "d8-x"}, "type": "cloud-provider.deckhouse.io/credentials"},
			"d8-y/cloud-credentials": {"apiVersion": "v1", "kind": "Secret", "metadata": map[string]interface{}{"name": "cloud-credentials", "namespace": "d8-y"}, "type": "cloud-provider.deckhouse.io/credentials"},
		},
	}

	first, err := encodeResourcesYAML(cv)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		next, err := encodeResourcesYAML(cv)
		require.NoError(t, err)
		require.Equal(t, first, next, "encodeResourcesYAML must be deterministic")
	}
}

func TestToWireInput_PreservesUserResourcesYAML(t *testing.T) {
	user := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: extra\n"
	input := config.ProviderInput{
		ProviderName:      "dvp",
		Operation:         "bootstrap",
		ResourcesYAML:     user,
		CloudProviderVars: &providerdata.CloudProviderVars{},
	}

	wire, err := toWireInput(input)
	require.NoError(t, err)
	require.Contains(t, wire.ResourcesYAML, "kind: ConfigMap")
	require.Contains(t, wire.ResourcesYAML, "name: extra")
}

func TestToWireInput_PrefersUserResourcesYAMLOverEncoded(t *testing.T) {
	user := "apiVersion: deckhouse.io/v1\nkind: NodeGroup\nmetadata:\n  name: worker\n"
	input := config.ProviderInput{
		ProviderName:  "dvp",
		ResourcesYAML: user,
		CloudProviderVars: &providerdata.CloudProviderVars{
			NodeGroups: map[string]map[string]interface{}{
				"worker": {"apiVersion": "deckhouse.io/v1", "kind": "NodeGroup", "metadata": map[string]interface{}{"name": "worker"}},
			},
		},
	}

	wire, err := toWireInput(input)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(wire.ResourcesYAML, "kind: NodeGroup"), "user YAML must not be duplicated by encoded CloudProviderVars")
}

func TestToWireInput_EncodesCloudProviderVarsWhenNoUserYAML(t *testing.T) {
	input := config.ProviderInput{
		ProviderName: "dvp",
		CloudProviderVars: &providerdata.CloudProviderVars{
			NodeGroups: map[string]map[string]interface{}{
				"worker": {"apiVersion": "deckhouse.io/v1", "kind": "NodeGroup", "metadata": map[string]interface{}{"name": "worker"}},
			},
		},
	}

	wire, err := toWireInput(input)
	require.NoError(t, err)
	require.Contains(t, wire.ResourcesYAML, "kind: NodeGroup")
	require.Contains(t, wire.ResourcesYAML, "name: worker")
}

func TestToWireInput_ProviderClusterConfigJSONConverted(t *testing.T) {
	input := config.ProviderInput{
		ProviderName: "dvp",
		ProviderClusterConfig: map[string]json.RawMessage{
			"layout": json.RawMessage(`{"foo":"bar"}`),
		},
	}

	wire, err := toWireInput(input)
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{"foo": "bar"}, wire.ProviderClusterConfig["layout"])
}
