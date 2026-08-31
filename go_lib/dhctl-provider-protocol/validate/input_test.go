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

package validate

import (
	"reflect"
	"testing"
)

// The input must survive the wire unchanged: the host builds it from cluster
// objects and the plugin validates those objects field by field.
func TestValidateInputRoundTrip(t *testing.T) {
	input := Input{
		ProviderName:  "dvp",
		ClusterPrefix: "test",
		Layout:        "standard",
		Operation:     OperationConverge,
		ProviderClusterConfig: map[string]any{
			"layout": "Standard",
		},
		CloudProviderVars: &CloudProviderVars{
			Settings: map[string]any{"provider": map[string]any{"namespace": "default"}},
			NodeGroups: map[string]map[string]any{
				"master": {"kind": "NodeGroup", "spec": map[string]any{"nodeType": "CloudPermanent"}},
			},
			Secrets: map[string]map[string]any{
				"d8-credentials": {"type": CredentialsSecretType},
			},
		},
	}

	req, err := ToPBRequest(input)
	if err != nil {
		t.Fatalf("ToPBRequest() = %v", err)
	}

	got, err := FromPBRequest(req)
	if err != nil {
		t.Fatalf("FromPBRequest() = %v", err)
	}
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("round trip changed the input:\ngot  %+v\nwant %+v", got, input)
	}
}

// Absent vars must stay absent rather than become an empty struct: a plugin
// branches on nil to tell "nothing was collected" from "nothing exists".
func TestValidateInputKeepsNilVars(t *testing.T) {
	req, err := ToPBRequest(Input{Operation: OperationDestroy})
	if err != nil {
		t.Fatalf("ToPBRequest() = %v", err)
	}

	got, err := FromPBRequest(req)
	if err != nil {
		t.Fatalf("FromPBRequest() = %v", err)
	}
	if got.CloudProviderVars != nil {
		t.Fatalf("CloudProviderVars = %+v, want nil", got.CloudProviderVars)
	}
}
