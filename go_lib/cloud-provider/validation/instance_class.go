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

// ValidateMasterInstanceClass checks master InstanceClass existence and etcdDisk requirements.
func ValidateMasterInstanceClass(state *State) Result {
	if state == nil {
		return ResultForNilState()
	}

	result := Result{}

	masterNodeGroup, found := findNodeGroup(state, "master")
	if !found {
		return result
	}

	if masterNodeGroup.Spec.CloudInstances == nil || masterNodeGroup.Spec.CloudInstances.ClassReference == nil {
		return result
	}

	classRef := masterNodeGroup.Spec.CloudInstances.ClassReference
	if strings.TrimSpace(classRef.Name) == "" {
		return result
	}

	class, found := findInstanceClass(state, classRef.Name)
	if !found {
		result.AddError(
			"NodeGroup/master.spec.cloudInstances.classReference.name",
			"master_instance_class_not_found",
			classRef.Name,
			fmt.Sprintf("%s %q was not found", state.InstanceClassKind, classRef.Name),
		)

		return result
	}

	if class.Spec.EtcdDisk == nil {
		result.AddError(
			state.InstanceClassKind+"/"+classRef.Name+".spec.etcdDisk",
			"master_etcd_disk_required",
			nil,
			fmt.Sprintf("master %s must define spec.etcdDisk", state.InstanceClassKind),
		)
	}

	return result
}

// ValidateInstanceClassEtcdDiskAttachment checks etcdDisk usage against NodeGroup attachments.
func ValidateInstanceClassEtcdDiskAttachment(state *State) Result {
	if state == nil {
		return ResultForNilState()
	}

	result := Result{}

	consumers := collectInstanceClassConsumers(state.InstanceClassKind, state.NodeGroups)

	for _, class := range state.InstanceClasses {
		if class.Kind != "" && class.Kind != state.InstanceClassKind {
			continue
		}

		if class.Spec.EtcdDisk == nil {
			continue
		}

		path := namedResourcePath(state.InstanceClassKind, class.Name)

		for _, nodeGroupName := range consumers[class.Name] {
			if nodeGroupName == "master" {
				continue
			}

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
