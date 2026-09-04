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
	"reflect"
	"testing"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// The vars dhctl collected must reach the validator as they are. Re-encoding them on
// the way would flatten the typed maps the validator reads field by field, so the
// pointer itself has to survive toWireInput.
func TestToWireInputVarsTravelStructurally(t *testing.T) {
	vars := &config.CloudProviderVars{
		Settings: map[string]any{"zone": "a"},
		NodeGroups: map[string]map[string]any{
			"worker": {"apiVersion": "deckhouse.io/v1", "kind": "NodeGroup", "metadata": map[string]any{"name": "worker"}},
		},
		InstanceClasses: map[string]map[string]any{
			"m": {"apiVersion": "deckhouse.io/v1", "kind": "DVPInstanceClass", "metadata": map[string]any{"name": "m"}},
		},
		Secrets: map[string]map[string]any{
			"d8-x/cloud-credentials": {"apiVersion": "v1", "kind": "Secret", "type": validatev1.CredentialsSecretType},
		},
	}

	wire, err := toWireInput(config.ProviderInput{
		ProviderName:      "dvp",
		Operation:         string(validatev1.OperationConverge),
		CloudProviderVars: vars,
	})
	if err != nil {
		t.Fatalf("toWireInput() = %v", err)
	}

	if wire.CloudProviderVars != vars {
		t.Error("vars must be passed through, not re-encoded")
	}
}

// The provider cluster configuration arrives as raw JSON and has to be decoded, not
// forwarded as a string: the validator addresses it as an object.
func TestToWireInputProviderClusterConfigJSONConverted(t *testing.T) {
	wire, err := toWireInput(config.ProviderInput{
		ProviderName: "dvp",
		ProviderClusterConfig: map[string]json.RawMessage{
			"layout": json.RawMessage(`{"foo":"bar"}`),
		},
	})
	if err != nil {
		t.Fatalf("toWireInput() = %v", err)
	}

	want := map[string]any{"foo": "bar"}
	if got := wire.ProviderClusterConfig["layout"]; !reflect.DeepEqual(got, want) {
		t.Errorf("ProviderClusterConfig[\"layout\"] = %#v, want %#v", got, want)
	}
}

// Malformed JSON must stop the call rather than reach the validator as a hole in the
// configuration it is supposed to check.
func TestToWireInputRejectsMalformedJSON(t *testing.T) {
	_, err := toWireInput(config.ProviderInput{
		ProviderName: "dvp",
		ProviderClusterConfig: map[string]json.RawMessage{
			"layout": json.RawMessage(`{`),
		},
	})
	if err == nil {
		t.Fatal("toWireInput() = nil, want an error")
	}
}

func TestViolationsToStrings(t *testing.T) {
	tests := []struct {
		name       string
		violations []*validatev1.ViolationResponse
		want       []string
	}{
		{
			name: "no violations render as no lines",
			want: []string{},
		},
		{
			name:       "a violation with a path is prefixed with it",
			violations: []*validatev1.ViolationResponse{{Path: "NodeGroup/worker", Message: "replicas is 0"}},
			want:       []string{"NodeGroup/worker: replicas is 0"},
		},
		{
			name:       "a violation without a path is the message alone",
			violations: []*validatev1.ViolationResponse{{Message: "credential Secret is required"}},
			want:       []string{"credential Secret is required"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := violationsToStrings(test.violations); !reflect.DeepEqual(got, test.want) {
				t.Errorf("violationsToStrings() = %q, want %q", got, test.want)
			}
		})
	}
}
