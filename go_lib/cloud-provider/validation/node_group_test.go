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

func TestValidateMasterNodeGroupClassReferenceNilState(t *testing.T) {
	t.Parallel()

	result := ValidateMasterNodeGroupClassReference(nil)
	if !hasViolationCode(result, CodeInternalStateNil) {
		t.Fatalf("ValidateMasterNodeGroupClassReference(nil) = %q, want %s", result.Error(), CodeInternalStateNil)
	}
}

func TestValidateMasterNodeGroupClassReferenceRequiresMaster(t *testing.T) {
	t.Parallel()

	result := ValidateMasterNodeGroupClassReference(masterNodeGroupState(nil))
	if !hasViolationCode(result, "master_node_group_required") {
		t.Fatalf("ValidateMasterNodeGroupClassReference() = %q", result.Error())
	}
}

func TestValidateMasterNodeGroupClassReferenceRequiresClassReference(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances = nil

	result := ValidateMasterNodeGroupClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}))
	if !hasViolationCode(result, "master_class_reference_required") {
		t.Fatalf("ValidateMasterNodeGroupClassReference() = %q", result.Error())
	}
}

func TestValidateMasterNodeGroupClassReferenceRejectsInvalidInstanceClassKind(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Kind = "WrongKind"

	result := ValidateMasterNodeGroupClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}))
	if !hasViolationCode(result, "master_invalid_instance_class_kind") {
		t.Fatalf("ValidateMasterNodeGroupClassReference() = %q", result.Error())
	}
}

func TestValidateMasterNodeGroupClassReferenceRequiresInstanceClassName(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Name = "  "

	result := ValidateMasterNodeGroupClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}))
	if !hasViolationCode(result, "master_instance_class_name_required") {
		t.Fatalf("ValidateMasterNodeGroupClassReference() = %q", result.Error())
	}
}

func TestValidateMasterNodeGroupClassReferenceReportsKindAndNameErrors(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Kind = "WrongKind"
	master.Spec.CloudInstances.ClassReference.Name = ""

	result := ValidateMasterNodeGroupClassReference(masterNodeGroupState([]cpapi.NodeGroup{master}))
	if !hasViolationCode(result, "master_invalid_instance_class_kind") {
		t.Fatalf("ValidateMasterNodeGroupClassReference() = %q, want invalid kind", result.Error())
	}
	if !hasViolationCode(result, "master_instance_class_name_required") {
		t.Fatalf("ValidateMasterNodeGroupClassReference() = %q, want missing name", result.Error())
	}
}

func TestValidateMasterNodeGroupClassReferenceSuccess(t *testing.T) {
	t.Parallel()

	result := ValidateMasterNodeGroupClassReference(masterNodeGroupState([]cpapi.NodeGroup{validMasterNodeGroup()}))
	if result.HasErrors() {
		t.Fatalf("ValidateMasterNodeGroupClassReference() unexpected errors: %s", result.Error())
	}
}
