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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
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

func TestValidateMasterNodeGroupNilState(t *testing.T) {
	t.Parallel()

	result := ValidateMasterNodeGroup(nil)
	if !hasViolationCode(result, CodeInternalStateNil) {
		t.Fatalf("ValidateMasterNodeGroup(nil) = %q, want %s", result.Error(), CodeInternalStateNil)
	}
}

func TestValidateMasterNodeGroupRequiresMaster(t *testing.T) {
	t.Parallel()

	result := ValidateMasterNodeGroup(masterNodeGroupState(nil))
	if !hasViolationCode(result, "master_node_group_required") {
		t.Fatalf("ValidateMasterNodeGroup() = %q", result.Error())
	}
}

func TestValidateMasterNodeGroupRequiresClassReference(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances = nil

	result := ValidateMasterNodeGroup(masterNodeGroupState([]cpapi.NodeGroup{master}))
	if !hasViolationCode(result, "master_class_reference_required") {
		t.Fatalf("ValidateMasterNodeGroup() = %q", result.Error())
	}
}

func TestValidateMasterNodeGroupRejectsInvalidInstanceClassKind(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Kind = "WrongKind"

	result := ValidateMasterNodeGroup(masterNodeGroupState([]cpapi.NodeGroup{master}))
	if !hasViolationCode(result, "master_invalid_instance_class_kind") {
		t.Fatalf("ValidateMasterNodeGroup() = %q", result.Error())
	}
}

func TestValidateMasterNodeGroupRequiresInstanceClassName(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Name = "  "

	result := ValidateMasterNodeGroup(masterNodeGroupState([]cpapi.NodeGroup{master}))
	if !hasViolationCode(result, "master_instance_class_name_required") {
		t.Fatalf("ValidateMasterNodeGroup() = %q", result.Error())
	}
}

func TestValidateMasterNodeGroupReportsKindAndNameErrors(t *testing.T) {
	t.Parallel()

	master := validMasterNodeGroup()
	master.Spec.CloudInstances.ClassReference.Kind = "WrongKind"
	master.Spec.CloudInstances.ClassReference.Name = ""

	result := ValidateMasterNodeGroup(masterNodeGroupState([]cpapi.NodeGroup{master}))
	if !hasViolationCode(result, "master_invalid_instance_class_kind") {
		t.Fatalf("ValidateMasterNodeGroup() = %q, want invalid kind", result.Error())
	}
	if !hasViolationCode(result, "master_instance_class_name_required") {
		t.Fatalf("ValidateMasterNodeGroup() = %q, want missing name", result.Error())
	}
}

func TestValidateMasterNodeGroupSuccess(t *testing.T) {
	t.Parallel()

	result := ValidateMasterNodeGroup(masterNodeGroupState([]cpapi.NodeGroup{validMasterNodeGroup()}))
	if result.HasErrors() {
		t.Fatalf("ValidateMasterNodeGroup() unexpected errors: %s", result.Error())
	}
}
