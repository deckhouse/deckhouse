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

package testprovider

import "testing"

func TestInstanceClassContract(t *testing.T) {
	t.Parallel()

	class := &InstanceClass{}
	class.Kind = "SomethingElse"
	class.Name = "master-test"
	class.Status.NodeGroupConsumers = []string{"master"}

	if class.GetName() != "master-test" {
		t.Fatalf("GetName() = %q, want master-test", class.GetName())
	}
	// The kind is static, not read from TypeMeta: a class deserialized without
	// apiVersion/kind must still report the provider kind.
	if class.GroupVersionKind().Kind != "TestInstanceClass" {
		t.Fatalf("GroupVersionKind().Kind = %q, want TestInstanceClass", class.GroupVersionKind().Kind)
	}
	if class.GetEtcdDisk() != nil {
		t.Fatal("GetEtcdDisk() != nil, want nil")
	}
	if len(class.GetNodeGroupConsumers()) != 1 {
		t.Fatalf("GetNodeGroupConsumers() = %v, want one consumer", class.GetNodeGroupConsumers())
	}

	class.Spec.EtcdDisk = &EtcdDisk{Size: "10Gi"}
	if class.GetEtcdDisk() == nil {
		t.Fatal("GetEtcdDisk() = nil, want the etcd disk")
	}
}

func TestInstanceClassContractNilSafe(t *testing.T) {
	t.Parallel()

	var class *InstanceClass

	// GroupVersionKind() must report the static kind even on a nil receiver:
	// ValidateNodeGroupsClassReference derives the expected classReference kind
	// from the zero value of the InstanceClass type parameter.
	if class.GroupVersionKind().Kind != "TestInstanceClass" {
		t.Fatalf("GroupVersionKind().Kind on nil = %q, want TestInstanceClass", class.GroupVersionKind().Kind)
	}
	if class.GetEtcdDisk() != nil {
		t.Fatal("GetEtcdDisk() on nil must be nil")
	}
	if class.GetNodeGroupConsumers() != nil {
		t.Fatal("GetNodeGroupConsumers() on nil must be nil")
	}
	if class.GetName() != "" {
		t.Fatal("GetName() on nil must be empty")
	}
}

func TestSettingsSectionPresence(t *testing.T) {
	t.Parallel()

	settings := &Settings{Provider: Section{Parameters: map[string]string{"namespace": "ns"}}}

	if !settings.HasProviderSection() {
		t.Fatal("HasProviderSection() = false, want true")
	}
	if settings.HasNodesSection() || settings.HasStorageSection() || settings.HasCCMSection() {
		t.Fatal("empty sections must be reported as absent")
	}

	var nilSettings *Settings
	if nilSettings.HasProviderSection() {
		t.Fatal("HasProviderSection() on nil must be false")
	}
}

func TestProviderClusterConfigContract(t *testing.T) {
	t.Parallel()

	pcc := &ProviderClusterConfig{
		MasterNodeGroup: &MasterNodeGroup{Replicas: 3},
		NodeGroups:      []NodeGroup{{Name: "worker", Replicas: 1}},
	}

	if !pcc.HasMasterNodeGroup() {
		t.Fatal("HasMasterNodeGroup() = false, want true")
	}
	names := pcc.NodeGroupNames()
	if len(names) != 1 || names[0] != "worker" {
		t.Fatalf("NodeGroupNames() = %v, want [worker]", names)
	}

	var nilPCC *ProviderClusterConfig
	if nilPCC.HasMasterNodeGroup() {
		t.Fatal("HasMasterNodeGroup() on nil must be false")
	}
	if nilPCC.NodeGroupNames() != nil {
		t.Fatal("NodeGroupNames() on nil must be nil")
	}
}
