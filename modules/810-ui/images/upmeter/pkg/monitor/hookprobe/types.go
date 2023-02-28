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

package hookprobe

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Spec is the spec in the HookProbe CRD
type Spec struct {
	Inited string `json:"inited"`
	Mirror string `json:"mirror"`
}

// HookProbe is the Schema for the remote_write options
type HookProbe struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec Spec `json:"spec,omitempty"`
}

// HookProbeList contains a list of HookProbe objects
type HookProbeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []HookProbe `json:"items"`
}
