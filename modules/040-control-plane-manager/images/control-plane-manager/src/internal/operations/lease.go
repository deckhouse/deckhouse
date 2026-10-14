/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operations

import (
	"strings"

	"k8s.io/apimachinery/pkg/types"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// LeaderLeaseKey is the leader election Lease of a control plane component.
func LeaderLeaseKey(component controlplanev1alpha1.OperationComponent) types.NamespacedName {
	return types.NamespacedName{Namespace: constants.KubeSystemNamespace, Name: component.PodComponentName()}
}

// LeaseHolderNode returns the node part of a leader election holderIdentity ("<hostname>_<uuid>").
// Control plane static pods run in the host network namespace, so the hostname is the node's.
func LeaseHolderNode(holderIdentity string) string {
	node, _, _ := strings.Cut(holderIdentity, "_")
	return node
}
