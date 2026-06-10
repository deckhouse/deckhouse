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
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
)

// ValidatePreflight checks resources required before cluster bootstrap or converge.
func ValidatePreflight(state *cpval.State) cpval.Result {
	result := cpval.Result{}
	if state == nil || cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return result
	}

	secret, secretFound := findCredentialSecret(state.CredentialSecrets, cpapi.CredentialSecretName)
	if !secretFound {
		result.AddError("Secret/"+cpapi.CredentialSecretName, "preflight_credential_secret_required", `credential Secret "d8-credentials" is required`)
	} else if secret.Type != cpapi.CredentialsSecretType {
		result.AddError("Secret/"+cpapi.CredentialSecretName+".type", "preflight_invalid_credential_secret_type", fmt.Sprintf("credential Secret type must be %q", cpapi.CredentialsSecretType))
	}

	masterNodeGroup, found := findNodeGroup(state.NodeGroups, "master")
	if !found {
		result.AddError("NodeGroup/master", "preflight_master_node_group_required", `NodeGroup "master" is required`)
		return result
	}

	if masterNodeGroup.Spec.CloudInstances == nil || masterNodeGroup.Spec.CloudInstances.ClassReference == nil {
		result.AddError("NodeGroup/master.spec.cloudInstances.classReference", "preflight_master_class_reference_required", `NodeGroup "master" must reference DVPInstanceClass`)
		return result
	}

	classRef := masterNodeGroup.Spec.CloudInstances.ClassReference
	if classRef.Kind != InstanceClassKind {
		result.AddError("NodeGroup/master.spec.cloudInstances.classReference.kind", "preflight_master_invalid_instance_class_kind", fmt.Sprintf("must be %q", InstanceClassKind))
	}

	if strings.TrimSpace(classRef.Name) == "" {
		result.AddError("NodeGroup/master.spec.cloudInstances.classReference.name", "preflight_master_instance_class_name_required", `NodeGroup "master" must reference DVPInstanceClass by name`)
		return result
	}

	class, found := findInstanceClass(state.InstanceClasses, classRef.Name)
	if !found {
		result.AddError("NodeGroup/master.spec.cloudInstances.classReference.name", "preflight_master_instance_class_not_found", fmt.Sprintf("DVPInstanceClass %q was not found", classRef.Name))
		return result
	}

	if class.Spec.EtcdDisk == nil {
		result.AddError(InstanceClassKind+"/"+classRef.Name+".spec.etcdDisk", "preflight_master_etcd_disk_required", "master DVPInstanceClass must define spec.etcdDisk")
	}

	return result
}

func findCredentialSecret(secrets []cpapi.CredentialSecret, name string) (cpapi.CredentialSecret, bool) {
	for _, secret := range secrets {
		if secret.Name == name && (secret.Namespace == "" || secret.Namespace == Namespace) {
			return secret, true
		}
	}

	return cpapi.CredentialSecret{}, false
}

func findNodeGroup(nodeGroups []cpapi.NodeGroup, name string) (cpapi.NodeGroup, bool) {
	for _, nodeGroup := range nodeGroups {
		if nodeGroup.Name == name {
			return nodeGroup, true
		}
	}

	return cpapi.NodeGroup{}, false
}

func findInstanceClass(classes []cpapi.InstanceClass, name string) (cpapi.InstanceClass, bool) {
	for _, class := range classes {
		if class.Name == name {
			return class, true
		}
	}

	return cpapi.InstanceClass{}, false
}
