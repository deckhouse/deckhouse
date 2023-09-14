package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BlockDevice empty block device
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ConsumableBlockDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status BlockDeviceStatus `json:"status,omitempty"`
}

// BlockDeviceList contains a list of empty block device
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ConsumableBlockDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ConsumableBlockDevice `json:"items"`
}

type BlockDeviceStatus struct {
	NodeName  string `json:"nodename"`
	ID        string `json:"id"`
	Path      string `json:"path"`
	Size      string `json:"size"`
	Model     string `json:"model"`
	MachineID string `json:"machine-id"`
}
