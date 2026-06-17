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
)

func TestFindHelpers(t *testing.T) {
	t.Parallel()

	const namespace = "d8-cloud-provider-test"

	state := &State{
		NamespaceName: namespace,
		CredentialSecrets: []cpapi.CredentialSecret{{
			ObjectMeta: cpapi.ObjectMeta{
				Name:      cpapi.CredentialSecretName,
				Namespace: namespace,
			},
		}},
		NodeGroups: []cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
		}},
		InstanceClasses: []cpapi.InstanceClass{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"},
		}},
	}

	if _, ok := findCredentialSecret(state, cpapi.CredentialSecretName); !ok {
		t.Fatal("findCredentialSecret() = false, want true")
	}

	emptyState := &State{NamespaceName: namespace}
	if _, ok := findCredentialSecret(emptyState, "missing"); ok {
		t.Fatal("findCredentialSecret() = true, want false")
	}

	if _, ok := findNodeGroup(state, "master"); !ok {
		t.Fatal("findNodeGroup() = false, want true")
	}

	if _, ok := findInstanceClass(state, "master-dvp"); !ok {
		t.Fatal("findInstanceClass() = false, want true")
	}
}

func TestFindCredentialSecretMatchesEmptyNamespace(t *testing.T) {
	t.Parallel()

	state := &State{
		NamespaceName: "d8-cloud-provider-test",
		CredentialSecrets: []cpapi.CredentialSecret{{
			ObjectMeta: cpapi.ObjectMeta{Name: cpapi.CredentialSecretName},
		}},
	}
	if _, ok := findCredentialSecret(state, cpapi.CredentialSecretName); !ok {
		t.Fatal("findCredentialSecret() = false for empty secret namespace")
	}
}
