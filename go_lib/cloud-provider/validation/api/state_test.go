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
	"github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/internal/testprovider"
)

// testState instantiates the generic State with provider stubs.
type testState = State[*testprovider.InstanceClass, *testprovider.Settings, *testprovider.ProviderClusterConfig]

// testInstanceClass builds a stub InstanceClass with the given name.
func testInstanceClass(name string) *testprovider.InstanceClass {
	class := &testprovider.InstanceClass{}
	class.Name = name

	return class
}

func TestStateListCredentialSecrets(t *testing.T) {
	t.Parallel()

	state := &testState{
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

	state := &testState{
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

	state := &testState{
		InstanceClasses: []*testprovider.InstanceClass{testInstanceClass("master-dvp"), testInstanceClass("worker-dvp")},
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

	state := &testState{
		NodeGroups: []cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "TestInstanceClass",
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
							Kind: "TestInstanceClass",
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
							Kind: "TestInstanceClass",
							Name: "worker-b-dvp",
						},
					},
				},
			},
			{
				// No cloudInstances: a NodeGroup without a class reference has no consumers.
				ObjectMeta: cpapi.ObjectMeta{Name: "no-class-reference"},
				Spec:       cpapi.NodeGroupSpec{},
			},
			{
				// Static NodeGroups are not consumers even with a class reference:
				// ListInstanceClassConsumers keeps only CloudPermanent NodeGroups.
				ObjectMeta: cpapi.ObjectMeta{Name: "static"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeStatic,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "TestInstanceClass",
							Name: "static-dvp",
						},
					},
				},
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
	if _, ok := consumers["static-dvp"]; ok {
		t.Fatal("consumers should not have entry for a class referenced only by a Static NodeGroup")
	}
}

func TestStateExistsCredentialSecretNotFound(t *testing.T) {
	t.Parallel()

	state := &testState{
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

	state := &testState{
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

	state := &testState{
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

	state := &testState{
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

func TestStateFindInstanceClass(t *testing.T) {
	t.Parallel()

	state := &testState{
		InstanceClasses: []*testprovider.InstanceClass{testInstanceClass("master-dvp")},
	}

	class, ok := state.FindInstanceClass("master-dvp")
	if !ok {
		t.Fatal("FindInstanceClass(master-dvp) = false, want true")
	}
	if class.GetName() != "master-dvp" {
		t.Fatalf("FindInstanceClass().GetName() = %q, want master-dvp", class.GetName())
	}

	missing, ok := state.FindInstanceClass("missing")
	if ok {
		t.Fatal("FindInstanceClass(missing) = true, want false")
	}
	if missing != nil {
		t.Fatalf("FindInstanceClass(missing) = %#v, want nil", missing)
	}
}

func TestStateHasProviderClusterConfig(t *testing.T) {
	t.Parallel()

	empty := &testState{}
	if empty.HasProviderClusterConfig() {
		t.Fatal("HasProviderClusterConfig() on nil PCC = true, want false")
	}

	withPCC := &testState{
		ProviderClusterConfig: &testprovider.ProviderClusterConfig{
			MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
		},
	}
	if !withPCC.HasProviderClusterConfig() {
		t.Fatal("HasProviderClusterConfig() with PCC = false, want true")
	}
}

func TestIsResourceAbsent(t *testing.T) {
	t.Parallel()

	var nilClass *testprovider.InstanceClass

	if !IsResourceAbsent(nilClass) {
		t.Fatal("IsResourceAbsent(nil pointer) = false, want true")
	}
	if !IsResourceAbsent(nil) {
		t.Fatal("IsResourceAbsent(nil) = false, want true")
	}
	if IsResourceAbsent(testInstanceClass("master-dvp")) {
		t.Fatal("IsResourceAbsent(non-nil pointer) = true, want false")
	}
	if IsResourceAbsent(cpapi.NodeGroup{ObjectMeta: cpapi.ObjectMeta{Name: "master"}}) {
		t.Fatal("IsResourceAbsent(non-zero struct) = true, want false")
	}
}
