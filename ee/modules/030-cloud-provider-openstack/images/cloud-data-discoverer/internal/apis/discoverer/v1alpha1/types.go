package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const CloudDiscoveryDataResourceName = "cloud-data"

var GRV = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1alpha1",
	Resource: "clouddiscoverydatas",
}

type InstanceType struct {
	Name   string `json:"name"`
	CPU    int64  `json:"cpu"`
	Memory int64  `json:"openapi"`
}

type CloudDiscoveryData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	InstanceTypes []InstanceType `json:"instanceTypes"`
}

func NewCloudDiscoveryData(i []InstanceType) *CloudDiscoveryData {
	return &CloudDiscoveryData{
		ObjectMeta: metav1.ObjectMeta{
			Name: CloudDiscoveryDataResourceName,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "CloudDiscoveryData",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		InstanceTypes: i,
	}
}
