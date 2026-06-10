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

import cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"

// ValidateInstanceClassEtcdDiskAttachment checks etcdDisk usage against NodeGroup attachments.
func ValidateInstanceClassEtcdDiskAttachment(
	instanceClassKind string,
	nodeGroups []cpapi.NodeGroup,
	instanceClasses []cpapi.InstanceClass,
) Result {
	result := Result{}
	consumers := collectInstanceClassConsumers(instanceClassKind, nodeGroups)

	for _, class := range instanceClasses {
		if class.Kind != "" && class.Kind != instanceClassKind {
			continue
		}

		if class.Spec.EtcdDisk == nil {
			continue
		}

		path := namedResourcePath(instanceClassKind, class.Name)
		classConsumers := consumers[class.Name]
		hasMasterConsumer := false

		for _, nodeGroupName := range classConsumers {
			if nodeGroupName == "master" {
				hasMasterConsumer = true
				continue
			}

			result.AddError(
				path+".spec.etcdDisk",
				"etcd_disk_forbidden_for_non_master",
				"InstanceClass.spec.etcdDisk can be used only when class is attached to NodeGroup master",
			)
		}

		if !hasMasterConsumer {
			result.AddError(
				path+".spec.etcdDisk",
				"etcd_disk_requires_master_attachment",
				"InstanceClass.spec.etcdDisk can be used only when class is attached to NodeGroup master",
			)
		}
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
