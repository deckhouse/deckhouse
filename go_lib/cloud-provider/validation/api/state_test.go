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

package api

import (
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func TestStateListCredentialSecrets(t *testing.T) {
	t.Parallel()

	state := &State{
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "d8-credentials"},
				Type:       cpapi.CredentialsSecretType,
			},
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "not-managed"},
				Type:       "kubernetes.io/tls",
			},
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "extra-managed"},
				Type:       cpapi.CredentialsSecretType,
			},
		},
	}

	managed := state.ListCredentialSecrets()
	if len(managed) != 2 {
		t.Fatalf("ListCredentialSecrets() = %d, want 2", len(managed))
	}
	if managed[0].Name != "d8-credentials" {
		t.Fatalf("ListCredentialSecrets()[0].Name = %q, want d8-credentials", managed[0].Name)
	}
}

func TestStateExistsNodeGroup(t *testing.T) {
	t.Parallel()

	state := &State{
		NodeGroups: []cpapi.NodeGroup{
			{ObjectMeta: cpapi.ObjectMeta{Name: "master"}},
			{ObjectMeta: cpapi.ObjectMeta{Name: "worker"}},
		},
	}

	if !state.ExistsNodeGroup("master") {
		t.Fatal("ExistsNodeGroup(master) = false, want true")
	}
	if state.ExistsNodeGroup("missing") {
		t.Fatal("ExistsNodeGroup(missing) = true, want false")
	}
}

func TestStateExistsInstanceClass(t *testing.T) {
	t.Parallel()

	state := &State{
		InstanceClasses: []cpapi.InstanceClass{
			{ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"}},
			{ObjectMeta: cpapi.ObjectMeta{Name: "worker-dvp"}},
		},
	}

	if !state.ExistsInstanceClass("master-dvp") {
		t.Fatal("ExistsInstanceClass(master-dvp) = false, want true")
	}
	if state.ExistsInstanceClass("missing") {
		t.Fatal("ExistsInstanceClass(missing) = true, want false")
	}
}

func TestStateListInstanceClassConsumers(t *testing.T) {
	t.Parallel()

	state := &State{
		InstanceClassKind: "DVPInstanceClass",
		NodeGroups: []cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "DVPInstanceClass",
							Name: "shared-dvp",
						},
					},
				},
			},
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "worker-a"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "DVPInstanceClass",
							Name: "shared-dvp",
						},
					},
				},
			},
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "worker-b"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "DVPInstanceClass",
							Name: "worker-b-dvp",
						},
					},
				},
			},
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "static"},
				Spec:       cpapi.NodeGroupSpec{}, // not CloudPermanent
			},
		},
	}

	consumers := state.ListInstanceClassConsumers()

	if len(consumers["shared-dvp"]) != 2 {
		t.Fatalf("consumers[shared-dvp] = %d, want 2", len(consumers["shared-dvp"]))
	}
	if len(consumers["worker-b-dvp"]) != 1 {
		t.Fatalf("consumers[worker-b-dvp] = %d, want 1", len(consumers["worker-b-dvp"]))
	}
	if _, ok := consumers["nonexistent"]; ok {
		t.Fatal("consumers should not have entry for nonexistent class")
	}
}

func TestStateExistsCredentialSecretNotFound(t *testing.T) {
	t.Parallel()

	state := &State{
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "other-secret"},
				Type:       cpapi.CredentialsSecretType,
			},
		},
	}

	if state.ExistsCredentialSecret("d8-credentials") {
		t.Fatal("ExistsCredentialSecret(d8-credentials) = true, want false")
	}
}

func TestStateExistsCredentialSecretFindsByNamespaceAndName(t *testing.T) {
	t.Parallel()

	state := &State{
		NamespaceName: "d8-cloud-provider-test",
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "d8-credentials", Namespace: "d8-cloud-provider-test"},
				Type:       cpapi.CredentialsSecretType,
			},
		},
	}

	if !state.ExistsCredentialSecret("d8-credentials") {
		t.Fatal("ExistsCredentialSecret(d8-credentials) = false, want true")
	}
}

func TestStateExistsCredentialSecretSkipsWrongNamespace(t *testing.T) {
	t.Parallel()

	state := &State{
		NamespaceName: "d8-cloud-provider-test",
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "d8-credentials", Namespace: "other-ns"},
				Type:       cpapi.CredentialsSecretType,
			},
		},
	}

	if state.ExistsCredentialSecret("d8-credentials") {
		t.Fatal("ExistsCredentialSecret should skip secrets in wrong namespace")
	}
}

func TestStateExistsCredentialSecretSkipsNonManaged(t *testing.T) {
	t.Parallel()

	state := &State{
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "d8-credentials"},
				Type:       "kubernetes.io/tls",
			},
		},
	}

	if state.ExistsCredentialSecret("d8-credentials") {
		t.Fatal("ExistsCredentialSecret should skip non-managed secrets")
	}
}
