package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// LvmVolumeGroup
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LvmVolumeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              struct {
		Type              string `json:"type"`
		ActuaLvgOnTheNode string `json:"actualvgonthenode"`
	} `json:"spec"`
	Status struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"status,omitempty"`
}

// LvmVolumeGroupList
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LvmVolumeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []LvmVolumeGroup `json:"items"`
}
