/*
Copyright 2024 Flant JSC

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

package v1

import (
	"sort"

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
	Spec   NodeUserSpec   `json:"spec"`
	Status NodeUserStatus `json:"status"`
}

type NodeUserSpec struct {
	UID           int      `json:"uid"`
	SSHPublicKey  string   `json:"sshPublicKey,omitempty"`
	SSHPublicKeys []string `json:"sshPublicKeys,omitempty"`
	PasswordHash  string   `json:"passwordHash"`
	IsSudoer      bool     `json:"isSudoer"`
	NodeGroups    []string `json:"nodeGroups"`
	ExtraGroups   []string `json:"extraGroups,omitempty"`
}

type NodeUserStatus struct {
	Errors map[string]string `json:"errors"`
}

func (nu NodeUserSpec) IsEqual(newSpec NodeUserSpec) bool {
	if nu.UID != newSpec.UID {
		return false
	}

	if nu.SSHPublicKey != newSpec.SSHPublicKey {
		return false
	}

	if nu.PasswordHash != newSpec.PasswordHash {
		return false
	}

	if nu.IsSudoer != newSpec.IsSudoer {
		return false
	}

	if !slicesIsEqual(nu.NodeGroups, newSpec.NodeGroups) {
		return false
	}

	if !slicesIsEqual(nu.SSHPublicKeys, newSpec.SSHPublicKeys) {
		return false
	}

	if !slicesIsEqual(nu.ExtraGroups, newSpec.ExtraGroups) {
		return false
	}

	return true
}

func slicesIsEqual(s1orig, s2orig []string) bool {
	s1 := make([]string, len(s1orig))
	s2 := make([]string, len(s2orig))
	copy(s1, s1orig)
	copy(s2, s2orig)

	if len(s1) != len(s2) {
		return false
	}

	sort.Strings(s1)
	sort.Strings(s2)

	for i := 0; i < len(s2); i++ {
		if s1[i] != s2[i] {
			return false
		}
	}

	return true
}
