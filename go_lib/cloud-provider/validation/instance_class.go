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
func ValidateInstanceClassesEtcdDisk[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
](state *cpvalapi.State[IC, S, PCC]) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}
	consumers := state.ListInstanceClassConsumers()

	for _, class := range state.InstanceClasses {
		if cpvalapi.IsResourceAbsent(class) {
			continue
		}

		kind := class.GroupVersionKind().Kind
		path := getNamedResourcePath(kind, class.GetName())
		nodeGroups := consumers[class.GetName()]

		hasMaster := false
		hasNonMaster := false
		for _, nodeGroupName := range nodeGroups {
			if nodeGroupName == "master" {
				hasMaster = true
				continue
			}

			hasNonMaster = true
		}

		if hasMaster && class.GetEtcdDisk() == nil {
			result.AddError(
				fmt.Sprintf("%s.spec.etcdDisk", path),
				CodeMasterEtcdDiskRequired,
				nil,
				fmt.Sprintf("%s for NodeGroup master must define spec.etcdDisk", kind),
			)
		}

		if hasNonMaster && class.GetEtcdDisk() != nil {
			result.AddError(
				fmt.Sprintf("%s.spec.etcdDisk", path),
				CodeEtcdDiskForbiddenForNonMaster,
				class.GetEtcdDisk(),
				"InstanceClass.spec.etcdDisk can be used only when class is attached to NodeGroup master",
			)
		}
	}

	return result
}

// ValidateInstanceClassDeletion checks whether an InstanceClass can be safely deleted
// (whether an InstanceClass has NodeGroup consumers).
func ValidateInstanceClassDeletion[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
](state *cpvalapi.State[IC, S, PCC], deletedClass IC) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	if cpvalapi.IsResourceAbsent(deletedClass) {
		return result
	}

	deletedKind := deletedClass.GroupVersionKind().Kind

	for _, nodeGroup := range state.NodeGroups {
		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		ref := nodeGroup.Spec.CloudInstances.ClassReference
		if ref.Kind == deletedKind && ref.Name == deletedClass.GetName() {
			result.AddError(
				fmt.Sprintf("%s/%s", deletedKind, deletedClass.GetName()),
				CodeInstanceClassInUse,
				nodeGroup.Name,
				fmt.Sprintf("InstanceClass is used by NodeGroup %q", nodeGroup.Name),
			)
		}
	}

	consumers := deletedClass.GetNodeGroupConsumers()
	if len(consumers) > 0 {
		result.AddError(
			fmt.Sprintf("%s/%s.status.nodeGroupConsumers", deletedKind, deletedClass.GetName()),
			CodeInstanceClassHasConsumers,
			len(consumers),
			fmt.Sprintf("%s is used by %d NodeGroup consumers", deletedKind, len(consumers)),
		)
	}

	return result
}
