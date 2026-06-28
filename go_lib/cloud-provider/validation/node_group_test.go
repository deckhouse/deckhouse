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

const testInstanceClassKind = "DVPInstanceClass"

func masterNodeGroupState(nodeGroups []cpapi.NodeGroup) *State {
	return &State{
		InstanceClassKind: testInstanceClassKind,
		NodeGroups:        nodeGroups,
	}
}

func validMasterNodeGroup() cpapi.NodeGroup {
	return cpapi.NodeGroup{
		ObjectMeta: cpapi.ObjectMeta{Name: "master"},
		Spec: cpapi.NodeGroupSpec{
			NodeType: cpapi.NodeTypeCloudPermanent,
			CloudInstances: &cpapi.CloudInstances{
				ClassReference: &cpapi.ClassReference{
					Kind: testInstanceClassKind,
					Name: "master-dvp",
				},
			},
		},
	}
}

func TestValidateNodeGroupsClassReferenceNilState(t *testing.T) {
	t.Parallel()

	result := ValidateNodeGroupsClassReference(nil, true)
	if !hasViolationCode(result, CodeInternalStateNil) {
		t.Fatalf("ValidateNodeGroupsClassReference(nil) = %q, want %s", result.Error(), CodeInternalStateNil)
	}
}

func TestValidateNodeGroupsClassReferenceRejectsNilCloudInstancesOnMaster(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances = nil

	result := ValidateNodeGroupsClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}), true)
	if !hasViolationCode(result, "node_group_cloud_instances_required") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q, want node_group_cloud_instances_required", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceRejectsInvalidInstanceClassKind(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Kind = "WrongKind"

	result := ValidateNodeGroupsClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}), true)
	if !hasViolationCode(result, "node_group_invalid_instance_class_kind") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceRequiresInstanceClassName(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Name = "  "

	result := ValidateNodeGroupsClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}), true)
	if !hasViolationCode(result, "node_group_class_reference_name_required") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceReportsKindAndNameErrors(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Kind = "WrongKind"
	master.Spec.CloudInstances.ClassReference.Name = ""

	result := ValidateNodeGroupsClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}), true)
	if !hasViolationCode(result, "node_group_invalid_instance_class_kind") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q, want invalid kind", result.Error())
	}
	if !hasViolationCode(result, "node_group_class_reference_name_required") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q, want missing name", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceSuccess(t *testing.T) {
	t.Parallel()

	state := masterNodeGroupState([]cpapi.NodeGroup{validMasterNodeGroup()})
	state.InstanceClasses = []cpapi.InstanceClass{{ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"}}}

	result := ValidateNodeGroupsClassReference(state, true)
	if result.HasErrors() {
		t.Fatalf("ValidateNodeGroupsClassReference() unexpected errors: %s", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceRejectsNilCloudInstancesOnWorker(t *testing.T) {
	t.Parallel()

	state := masterNodeGroupState([]cpapi.NodeGroup{
		validMasterNodeGroup(),
		{
			ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
			Spec:       cpapi.NodeGroupSpec{NodeType: cpapi.NodeTypeCloudPermanent},
		},
	})
	state.InstanceClasses = []cpapi.InstanceClass{{ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"}}}

	result := ValidateNodeGroupsClassReference(state, true)
	if !hasViolationCode(result, "node_group_cloud_instances_required") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q, want node_group_cloud_instances_required for worker", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceSkipsNonCloudPermanentNodeGroups(t *testing.T) {
	t.Parallel()

	state := masterNodeGroupState([]cpapi.NodeGroup{
		validMasterNodeGroup(),
		{
			ObjectMeta: cpapi.ObjectMeta{Name: "static-worker"},
			Spec:       cpapi.NodeGroupSpec{}, // NodeType not set (not CloudPermanent)
		},
	})
	state.InstanceClasses = []cpapi.InstanceClass{{ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"}}}

	result := ValidateNodeGroupsClassReference(state, true)
	if result.HasErrors() {
		t.Fatalf("ValidateNodeGroupsClassReference() unexpected errors for non-CloudPermanent NodeGroup: %s", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceRequiresMasterExistence(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Name = "missing-dvp"

	result := ValidateNodeGroupsClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}), true)
	if !hasViolationCode(result, "instance_class_not_found") {
		t.Fatalf("ValidateNodeGroupsClassReference(verifyExistence=true) = %q, want instance_class_not_found", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceSkipsExistenceInAdmission(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Name = "missing-dvp"

	result := ValidateNodeGroupsClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}), false)
	if result.HasErrors() {
		t.Fatalf("ValidateNodeGroupsClassReference(verifyExistence=false) = %q, want no existence check", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceEmptyNameDoesNotTriggerNotFound(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Name = "  "

	result := ValidateNodeGroupsClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}), true)
	if hasViolationCode(result, "instance_class_not_found") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q, want no instance_class_not_found for empty name", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceIteratesAllNodeGroups(t *testing.T) {
	t.Parallel()

	state := masterNodeGroupState([]cpapi.NodeGroup{
		validMasterNodeGroup(),
		{
			ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
			Spec: cpapi.NodeGroupSpec{
				NodeType: cpapi.NodeTypeCloudPermanent,
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: testInstanceClassKind, Name: "worker-dvp"},
				},
			},
		},
	})
	state.InstanceClasses = []cpapi.InstanceClass{{ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"}}}

	result := ValidateNodeGroupsClassReference(state, true)
	if !hasViolationCode(result, "instance_class_not_found") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q, want instance_class_not_found for worker-dvp", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceRejectsNilClassReferenceOnMaster(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances = &cpapi.CloudInstances{}

	result := ValidateNodeGroupsClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}), true)
	if !hasViolationCode(result, "node_group_cloud_instances_required") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q, want node_group_cloud_instances_required", result.Error())
	}
}

func TestValidateNodeGroupsClassReferenceAllowsNonCloudPermanentWithoutCloudInstances(t *testing.T) {
	t.Parallel()

	state := masterNodeGroupState([]cpapi.NodeGroup{
		validMasterNodeGroup(),
		{
			ObjectMeta: cpapi.ObjectMeta{Name: "static-worker"},
			Spec:       cpapi.NodeGroupSpec{}, // NodeType not set (not CloudPermanent)
		},
	})
	state.InstanceClasses = []cpapi.InstanceClass{{ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"}}}

	result := ValidateNodeGroupsClassReference(state, true)
	if result.HasErrors() {
		t.Fatalf("ValidateNodeGroupsClassReference() unexpected errors for non-CloudPermanent NodeGroup without CloudInstances: %s", result.Error())
	}
}

func TestValidateMasterNodeGroupPresenceNilState(t *testing.T) {
	t.Parallel()

	result := ValidateMasterNodeGroupPresence(nil)
	if !hasViolationCode(result, CodeInternalStateNil) {
		t.Fatalf("ValidateMasterNodeGroupPresence(nil) = %q, want %s", result.Error(), CodeInternalStateNil)
	}
}

func TestValidateMasterNodeGroupPresenceRequiresMaster(t *testing.T) {
	t.Parallel()

	result := ValidateMasterNodeGroupPresence(masterNodeGroupState(nil))
	if !hasViolationCode(result, "master_node_group_required") {
		t.Fatalf("ValidateMasterNodeGroupPresence() = %q, want master_node_group_required", result.Error())
	}
}

func TestValidateMasterNodeGroupPresenceSuccess(t *testing.T) {
	t.Parallel()

	result := ValidateMasterNodeGroupPresence(masterNodeGroupState([]cpapi.NodeGroup{validMasterNodeGroup()}))
	if result.HasErrors() {
		t.Fatalf("ValidateMasterNodeGroupPresence() unexpected errors: %s", result.Error())
	}
}
