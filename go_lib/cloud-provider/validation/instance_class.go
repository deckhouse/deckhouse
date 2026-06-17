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

// ValidateMasterInstanceClassReference checks that master NodeGroup references an existing InstanceClass.
func ValidateMasterInstanceClassReference(state *State) Result {
	if state == nil {
		return ResultForNilState()
	}

	result := Result{}

	classRef, ok := masterInstanceClassReference(state)
	if !ok {
		return result
	}

	if !existsInstanceClass(state, classRef.Name) {
		result.AddError(
			"NodeGroup/master.spec.cloudInstances.classReference.name",
			"master_instance_class_not_found",
			classRef.Name,
			fmt.Sprintf("%s %q was not found", state.InstanceClassKind, classRef.Name),
		)
	}

	return result
}

// ValidateInstanceClassesEtcdDisk checks spec.etcdDisk for all InstanceClasses:
// master-attached classes must define etcdDisk; etcdDisk is forbidden on non-master attachments.
func ValidateInstanceClassesEtcdDisk(state *State) Result {
	if state == nil {
		return ResultForNilState()
	}

	result := Result{}
	consumers := collectInstanceClassConsumers(state.InstanceClassKind, state.NodeGroups)

	for _, class := range state.InstanceClasses {
		if class.Kind != "" && class.Kind != state.InstanceClassKind {
			continue
		}

		path := namedResourcePath(state.InstanceClassKind, class.Name)
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
				path+".spec.etcdDisk",
				"master_etcd_disk_required",
				nil,
				fmt.Sprintf("master %s must define spec.etcdDisk", state.InstanceClassKind),
			)
		}

		if hasNonMaster && class.Spec.EtcdDisk != nil {
			result.AddError(
				path+".spec.etcdDisk",
				"etcd_disk_forbidden_for_non_master",
				class.Spec.EtcdDisk,
				"InstanceClass.spec.etcdDisk can be used only when class is attached to NodeGroup master",
			)
		}
	}

	return result
}

// ValidateInstanceClassDelete checks whether an InstanceClass can be safely deleted.
func ValidateInstanceClassDelete(state *State, className string, deletedClass *cpapi.InstanceClass) Result {
	if state == nil {
		return ResultForNilState()
	}

	result := Result{}

	if strings.TrimSpace(className) == "" && deletedClass != nil {
		className = deletedClass.Name
	}

	if strings.TrimSpace(className) == "" {
		return result
	}

	for _, nodeGroup := range state.NodeGroups {
		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		ref := nodeGroup.Spec.CloudInstances.ClassReference
		if ref.Kind == state.InstanceClassKind && ref.Name == className {
			result.AddError(
				state.InstanceClassKind+"/"+className,
				"instance_class_in_use",
				nodeGroup.Name,
				fmt.Sprintf("InstanceClass is used by NodeGroup %q", nodeGroup.Name),
			)
		}
	}

	if deletedClass != nil && len(deletedClass.Status.NodeGroupConsumers) > 0 {
		result.AddError(
			state.InstanceClassKind+"/"+className+".status.nodeGroupConsumers",
			"instance_class_has_consumers",
			len(deletedClass.Status.NodeGroupConsumers),
			fmt.Sprintf("%s is used by %d NodeGroup consumers", state.InstanceClassKind, len(deletedClass.Status.NodeGroupConsumers)),
		)
	}

	return result
}

func masterInstanceClassReference(state *State) (cpapi.ClassReference, bool) {
	masterNodeGroup, found := findNodeGroup(state, "master")
	if !found {
		return cpapi.ClassReference{}, false
	}

	if masterNodeGroup.Spec.CloudInstances == nil || masterNodeGroup.Spec.CloudInstances.ClassReference == nil {
		return cpapi.ClassReference{}, false
	}

	classRef := masterNodeGroup.Spec.CloudInstances.ClassReference
	if strings.TrimSpace(classRef.Name) == "" {
		return cpapi.ClassReference{}, false
	}

	return *classRef, true
}

func collectInstanceClassConsumers(instanceClassKind string, nodeGroups []cpapi.NodeGroup) map[string][]string {
	result := make(map[string][]string, len(nodeGroups))
	for _, nodeGroup := range nodeGroups {
		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		classRef := nodeGroup.Spec.CloudInstances.ClassReference
		if classRef.Kind != instanceClassKind || classRef.Name == "" {
			continue
		}

		result[classRef.Name] = append(result[classRef.Name], nodeGroup.Name)
	}

	return result
}
