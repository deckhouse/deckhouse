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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var NodeUserGVK = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "nodeusers",
}

// NodeUser is an system user on nodes.
type NodeUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NodeUserSpec `json:"spec"`
}

type NodeUserSpec struct {
	UID           int      `json:"uid"`
	SSHPublicKeys []string `json:"sshPublicKeys,omitempty"`
	PasswordHash  string   `json:"passwordHash"`
	IsSudoer      bool     `json:"isSudoer"`
	NodeGroups    []string `json:"nodeGroups"`
}
