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
	"fmt"
	"strings"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// ValidateMasterNodeGroupPresence checks that master NodeGroup exists (before bootstrap or converge).
func ValidateMasterNodeGroupPresence(state *State) Result {
	if state == nil {
		return ResultForNilState()
	}

	result := Result{}

	if !existsNodeGroup(state, "master") {
		result.AddError("NodeGroup/master", "master_node_group_required", nil, `NodeGroup "master" is required`)
	}

	return result
}

// ValidateNodeGroupsClassReference checks NodeGroup class references for CloudPermanent nodes:
//   - .spec.cloudInstances.classReference field presence
//   - valid kind classReference
//   - classReference name presence
//   - existent InstanceClass classReference (when verifyExistence is true)
//
// Non-CloudPermanent NodeGroups are skipped.
// Set verifyExistence=false during admission (InstanceClass may not exist yet).
func ValidateNodeGroupsClassReference(state *State, verifyExistence bool) Result {
	if state == nil {
		return ResultForNilState()
	}

	result := Result{}

	for _, nodeGroup := range state.NodeGroups {
		if nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
			continue
		}

		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		classRef := nodeGroup.Spec.CloudInstances.ClassReference
		if classRef.Kind != state.InstanceClassKind {
			result.AddError(
				"NodeGroup/"+nodeGroup.Name+".spec.cloudInstances.classReference.kind",
				"node_group_invalid_instance_class_kind",
				classRef.Kind,
				fmt.Sprintf(`NodeGroup "%s" must have reference with kind %s`, nodeGroup.Name, state.InstanceClassKind),
			)
		}

		if strings.TrimSpace(classRef.Name) == "" {
			result.AddError(
				"NodeGroup/"+nodeGroup.Name+".spec.cloudInstances.classReference.name",
				"node_group_class_reference_name_required",
				classRef.Name,
				fmt.Sprintf(`NodeGroup "%s" has empty class reference name`, nodeGroup.Name),
			)

			continue
		}

		if verifyExistence && !existsInstanceClass(state, classRef.Name) {
			result.AddError(
				"NodeGroup/"+nodeGroup.Name+".spec.cloudInstances.classReference.name",
				"instance_class_not_found",
				classRef.Name,
				fmt.Sprintf("%s %q was not found", state.InstanceClassKind, classRef.Name),
			)
		}
	}

	return result
}

func existsNodeGroup(state *State, name string) bool {
	for _, nodeGroup := range state.NodeGroups {
		if nodeGroup.Name == name {
			return true
		}
	}

	return false
}

func existsInstanceClass(state *State, name string) bool {
	for _, class := range state.InstanceClasses {
		if class.Name == name {
			return true
		}
	}

	return false
}
