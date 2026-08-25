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
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

// Validation violation codes for NodeGroup validation.
const (
	CodeMasterNodeGroupRequired             = "master_node_group_required"
	CodeNodeGroupInvalidInstanceClassKind   = "node_group_invalid_instance_class_kind"
	CodeNodeGroupClassReferenceNameRequired = "node_group_class_reference_name_required"
	CodeInstanceClassNotFound               = "instance_class_not_found"
)

// ValidateMasterNodeGroupPresence checks that master NodeGroup exists (before bootstrap or converge).
func ValidateMasterNodeGroupPresence[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
](state *cpvalapi.State[IC, S, PCC]) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	if !state.ExistsCloudPermanentNodeGroup("master") {
		result.AddError("NodeGroup/master", CodeMasterNodeGroupRequired, nil, `NodeGroup "master" is required`)
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
func ValidateNodeGroupsClassReference[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
](state *cpvalapi.State[IC, S, PCC], verifyExistence bool) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	var absentClass IC
	expectedKind := absentClass.GroupVersionKind().Kind

	for _, nodeGroup := range state.NodeGroups {
		if nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
			continue
		}

		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		classRef := nodeGroup.Spec.CloudInstances.ClassReference

		if classRef.Kind != expectedKind {
			result.AddError(
				fmt.Sprintf("NodeGroup/%s.spec.cloudInstances.classReference.kind", nodeGroup.Name),
				CodeNodeGroupInvalidInstanceClassKind,
				classRef.Kind,
				fmt.Sprintf("NodeGroup %q references %q, expected %q", nodeGroup.Name, classRef.Kind, expectedKind),
			)
		}

		if strings.TrimSpace(classRef.Name) == "" {
			result.AddError(
				fmt.Sprintf("NodeGroup/%s.spec.cloudInstances.classReference.name", nodeGroup.Name),
				CodeNodeGroupClassReferenceNameRequired,
				classRef.Name,
				fmt.Sprintf(`NodeGroup "%s" has empty class reference name`, nodeGroup.Name),
			)

			continue
		}

		if verifyExistence && !state.ExistsInstanceClass(classRef.Name) {
			result.AddError(
				fmt.Sprintf("NodeGroup/%s.spec.cloudInstances.classReference.name", nodeGroup.Name),
				CodeInstanceClassNotFound,
				classRef.Name,
				fmt.Sprintf("InstanceClass %q was not found", classRef.Name),
			)
		}
	}

	return result
}
