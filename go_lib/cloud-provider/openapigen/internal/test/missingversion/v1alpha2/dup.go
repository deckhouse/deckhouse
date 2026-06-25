// +groupName=test.openapigen.io
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
type MultiVersionResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MultiVersionResourceSpec `json:"spec"`
}
type MultiVersionResourceSpec struct {
	Name string `json:"name"`
}
