/*
Copyright 2023 Flant JSC

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

package template

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeUser is an system user on nodes.
type NodeUser struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a node user.
	Spec NodeUserSpec `json:"spec"`
}

type NodeUserSpec struct {
	Uid           int      `json:"uid"`
	SshPublicKey  string   `json:"sshPublicKey,omitempty"`
	SshPublicKeys []string `json:"sshPublicKeys,omitempty"`
	PasswordHash  string   `json:"passwordHash"`
	IsSudoer      bool     `json:"isSudoer"`
	NodeGroups    []string `json:"nodeGroups"`
	ExtraGroups   []string `json:"extraGroups,omitempty"`
}

func (nu NodeUserSpec) IsEqual(newSpec NodeUserSpec) bool {
	if nu.Uid != newSpec.Uid {
		return false
	}

	if nu.SshPublicKey != newSpec.SshPublicKey {
		return false
	}

	if nu.PasswordHash != newSpec.PasswordHash {
		return false
	}

	if nu.IsSudoer != newSpec.IsSudoer {
		return false
	}

	if !slices.Equal(nu.NodeGroups, newSpec.NodeGroups) {
		return false
	}

	if !slices.Equal(nu.SshPublicKeys, newSpec.SshPublicKeys) {
		return false
	}

	if !slices.Equal(nu.ExtraGroups, newSpec.ExtraGroups) {
		return false
	}

	return true
}
