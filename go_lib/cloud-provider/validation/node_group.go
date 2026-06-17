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
)

// ValidateMasterNodeGroupClassReference checks that master NodeGroup references a provider InstanceClass.
func ValidateMasterNodeGroupClassReference(state *State) Result {
	if state == nil {
		return ResultForNilState()
	}

	result := Result{}

	masterNodeGroup, found := findNodeGroup(state, "master")
	if !found {
		result.AddError("NodeGroup/master", "master_node_group_required", nil, `NodeGroup "master" is required`)
		return result
	}

	if masterNodeGroup.Spec.CloudInstances == nil || masterNodeGroup.Spec.CloudInstances.ClassReference == nil {
		result.AddError(
			"NodeGroup/master.spec.cloudInstances.classReference",
			"master_class_reference_required",
			nil,
			fmt.Sprintf(`NodeGroup "master" must reference %s`, state.InstanceClassKind),
		)

		return result
	}

	classRef := masterNodeGroup.Spec.CloudInstances.ClassReference
	if classRef.Kind != state.InstanceClassKind {
		result.AddError(
			"NodeGroup/master.spec.cloudInstances.classReference.kind",
			"master_invalid_instance_class_kind",
			classRef.Kind,
			fmt.Sprintf("must be %q", state.InstanceClassKind),
		)
	}

	if strings.TrimSpace(classRef.Name) == "" {
		result.AddError(
			"NodeGroup/master.spec.cloudInstances.classReference.name",
			"master_instance_class_name_required",
			classRef.Name,
			fmt.Sprintf(`NodeGroup "master" must reference %s by name`, state.InstanceClassKind),
		)
	}

	return result
}
