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

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

// Validation violation codes for InstanceClass validation.
const (
	CodeMasterEtcdDiskRequired        = "master_etcd_disk_required"
	CodeEtcdDiskForbiddenForNonMaster = "etcd_disk_forbidden_for_non_master"
	CodeInstanceClassInUse            = "instance_class_in_use"
	CodeInstanceClassHasConsumers     = "instance_class_has_consumers"
)

// ValidateInstanceClassesEtcdDisk checks spec.etcdDisk for all InstanceClasses:
// master-attached classes must define etcdDisk; etcdDisk is forbidden on non-master attachments.
func ValidateInstanceClassesEtcdDisk(state *cpvalapi.State) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}
	consumers := state.ListInstanceClassConsumers()

	for _, class := range state.InstanceClasses {
		if class.Kind != "" && class.Kind != state.InstanceClassKind {
			continue
		}

		path := getNamedResourcePath(state.InstanceClassKind, class.Name)
		nodeGroups := consumers[class.Name]

		hasMaster := false
		hasNonMaster := false
		for _, nodeGroupName := range nodeGroups {
			if nodeGroupName == "master" {
				hasMaster = true
				continue
			}

			hasNonMaster = true
		}

		if hasMaster && class.Spec.EtcdDisk == nil {
			result.AddError(
				fmt.Sprintf("%s.spec.etcdDisk", path),
				CodeMasterEtcdDiskRequired,
				nil,
				fmt.Sprintf("%s for NodeGroup master must define spec.etcdDisk", state.InstanceClassKind),
			)
		}

		if hasNonMaster && class.Spec.EtcdDisk != nil {
			result.AddError(
				fmt.Sprintf("%s.spec.etcdDisk", path),
				CodeEtcdDiskForbiddenForNonMaster,
				class.Spec.EtcdDisk,
				"InstanceClass.spec.etcdDisk can be used only when class is attached to NodeGroup master",
			)
		}
	}

	return result
}

// ValidateInstanceClassDeletion checks whether an InstanceClass can be safely deleted
// (whether an InstanceClass has NodeGroup consumers).
func ValidateInstanceClassDeletion(state *cpvalapi.State, deletedClass *cpapi.InstanceClass) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	if deletedClass == nil {
		return result
	}

	for _, nodeGroup := range state.NodeGroups {
		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		ref := nodeGroup.Spec.CloudInstances.ClassReference
		if ref.Kind == state.InstanceClassKind && ref.Name == deletedClass.Name {
			result.AddError(
				fmt.Sprintf("%s/%s", state.InstanceClassKind, deletedClass.Name),
				CodeInstanceClassInUse,
				nodeGroup.Name,
				fmt.Sprintf("InstanceClass is used by NodeGroup %q", nodeGroup.Name),
			)
		}
	}

	if len(deletedClass.Status.NodeGroupConsumers) > 0 {
		result.AddError(
			fmt.Sprintf("%s/%s.status.nodeGroupConsumers", state.InstanceClassKind, deletedClass.Name),
			CodeInstanceClassHasConsumers,
			len(deletedClass.Status.NodeGroupConsumers),
			fmt.Sprintf("%s is used by %d NodeGroup consumers", state.InstanceClassKind, len(deletedClass.Status.NodeGroupConsumers)),
		)
	}

	return result
}
