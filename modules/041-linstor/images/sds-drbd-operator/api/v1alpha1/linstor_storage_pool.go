package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// LinstorStoragePool
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LinstorStoragePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              struct {
		Type            string `json:"type"`
		LvmVolumeGroups []struct {
			Name         string `json:"name"`
			ThinPoolName string `json:"thinPoolName"`
		} `json:"lvmvolumegroups"`
	} `json:"spec"`
	Status struct {
		Phase  string `json:"phase"`
		Reason string `json:"reason"`
	} `json:"status,omitempty"`
}

// LinstorStoragePoolList contains a list of empty block device
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LinstorStoragePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []LinstorStoragePool `json:"items"`
}
