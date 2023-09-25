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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// LinstorStorageClass
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LinstorStorageClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              struct {
		LinstorStoragePool string `json:"linstorstoragepool,omitempty"`
		PlacementCount     int    `json:"placementcount"`
		ReclaimPolicy      string `json:"reclaimpolicy"`
		VolumeBindingMode  string `json:"volumebindingmode"`
		AllowVolumeExpand  bool   `json:"allowvolumeexpand"`
		DrbdOptions        struct {
			AutoQuorum string `json:"autoquorum"`
		} `json:"drbdoptions"`
	} `json:"spec"`
	Status struct {
		Phase  string `json:"phase"`
		Reason string `json:"reason"`
	} `json:"status,omitempty"`
}

// LinstorStorageClassList contains a list of empty block device
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LinstorStorageClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []LinstorStorageClass `json:"items"`
}
