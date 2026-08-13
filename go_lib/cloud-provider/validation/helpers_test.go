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

func testInstanceClass(name string, withEtcdDisk bool) *testprovider.InstanceClass {
	class := &testprovider.InstanceClass{}
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

// credentialContentState builds a validation State with the given credential secrets.
// Secrets are passed through as-is: filtering by type and namespace is the State's own
// job, so tests exercising that filter must be able to feed it non-managed secrets.
func credentialContentState(secrets []cpapi.CredentialSecret) *testState {
	return &testState{
		NamespaceName:     testNamespace,
		CredentialSecrets: secrets,
	}
}

// instanceClassState builds a validation State with InstanceClasses and NodeGroups.
func instanceClassState(nodeGroups []cpapi.NodeGroup, classes []*testprovider.InstanceClass) *testState {
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
