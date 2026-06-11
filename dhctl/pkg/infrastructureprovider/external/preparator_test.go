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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
)

func TestToWireInput_VarsTravelStructurally(t *testing.T) {
	cv := &providerdata.CloudProviderVars{
		Settings: map[string]interface{}{"zone": "a"},
		NodeGroups: map[string]map[string]interface{}{
			"worker": {"apiVersion": "deckhouse.io/v1", "kind": "NodeGroup", "metadata": map[string]interface{}{"name": "worker"}},
		},
		InstanceClasses: map[string]map[string]interface{}{
			"m": {"apiVersion": "deckhouse.io/v1", "kind": "DVPInstanceClass", "metadata": map[string]interface{}{"name": "m"}},
		},
		Secrets: map[string]map[string]interface{}{
			"d8-x/cloud-credentials": {"apiVersion": "v1", "kind": "Secret", "type": "cloud-provider.deckhouse.io/credentials"},
		},
	}
	input := config.ProviderInput{
		ProviderName:      "dvp",
		Operation:         "converge",
		CloudProviderVars: cv,
	}

	wire, err := toWireInput(input)
	require.NoError(t, err)
	require.Same(t, cv, wire.Vars, "vars must be passed through, not re-encoded")
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
