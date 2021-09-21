/*
Copyright 2021 Flant JSC

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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeUser is a linux user for all nodes.
type NodeUser struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines parameters for user.
	Spec NodeUserSpec `json:"spec"`
}

type NodeUserSpec struct {
	// Unique user ID.
	UID int32 `json:"uid"`

	// Ssh public key.
	SSHPublicKey string `json:"sshPublicKey"`

	// Hashed user password for /etc/shadow.
	PasswordHash string `json:"passwordHash"`

	// Is node user belongs to the sudo group.
	IsSudoer bool `json:"isSudoer"`

	// Additional system groups.
	ExtraGroups []string `json:"extraGroups,omitempty"`
}
