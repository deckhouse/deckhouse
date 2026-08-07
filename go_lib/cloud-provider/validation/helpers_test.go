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

package validation

import (
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	"github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/internal/testprovider"
)

const (
	testNamespace = "d8-cloud-provider-test"
)

type testState = cpvalapi.State[*testprovider.InstanceClass, *testprovider.Settings, *testprovider.ProviderClusterConfig]

func testInstanceClass(kind, name string, withEtcdDisk bool) *testprovider.InstanceClass {
	class := &testprovider.InstanceClass{}
	class.Kind = kind
	class.Name = name
	if withEtcdDisk {
		class.Spec.EtcdDisk = &testprovider.EtcdDisk{Size: "10Gi"}
	}
	return class
}

// hasViolationCode checks that result contains a violation with the given code.
func hasViolationCode(result cpvalapi.Result, code string) bool {
	for _, violation := range result.Errors() {
		if violation.Code == code {
			return true
		}
	}

	for _, violation := range result.Warnings() {
		if violation.Code == code {
			return true
		}
	}

	return false
}

// credentialContentState builds a validation State with managed credential secrets.
// nil secrets produces an empty slice.
func credentialContentState(secrets []cpapi.CredentialSecret) *testState {
	managed := make([]cpapi.CredentialSecret, 0, len(secrets))

	for _, s := range secrets {
		if s.IsManaged() {
			managed = append(managed, s)
		}
	}

	return &testState{
		NamespaceName:     testNamespace,
		CredentialSecrets: managed,
	}
}

// instanceClassState builds a validation State with InstanceClasses and NodeGroups.
func instanceClassState(kind string, nodeGroups []cpapi.NodeGroup, classes []*testprovider.InstanceClass) *testState {
	state := &testState{
		CredentialSecrets: nil,
	}

	if nodeGroups != nil {
		state.NodeGroups = nodeGroups
	}

	if classes != nil {
		state.InstanceClasses = classes
	}

	return state
}

func TestGetNamedResourcePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind string
		obj  string
		want string
	}{
		{"with name", "NgInstanceClass", "worker", "NgInstanceClass/worker"},
		{"empty name", "Secret", "", "Secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getNamedResourcePath(tt.kind, tt.obj); got != tt.want {
				t.Errorf("getNamedResourcePath(%q, %q) = %q, want %q", tt.kind, tt.obj, got, tt.want)
			}
		})
	}
}
