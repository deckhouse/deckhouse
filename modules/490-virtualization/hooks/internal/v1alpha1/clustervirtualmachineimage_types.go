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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// The desired state of `ClusterVirtualMachineImage`.
type ClusterVirtualMachineImageSpec struct {
	Remote ReducedDataVolumeSource `json:"remote,omitempty"`
	Source *TypedObjectReference   `json:"source,omitempty"`
}

// The source for `VirtualMachineImage`, this can be HTTP, S3, Registry or an existing PVC.
type ReducedDataVolumeSource struct {
	HTTP     *cdiv1.DataVolumeSourceHTTP      `json:"http,omitempty"`
	S3       *cdiv1.DataVolumeSourceS3        `json:"s3,omitempty"`
	Registry *ReducedDataVolumeSourceRegistry `json:"registry,omitempty"`
	PVC      *cdiv1.DataVolumeSourcePVC       `json:"pvc,omitempty"`
	Blank    *cdiv1.DataVolumeBlankImage      `json:"blank,omitempty"`
}

// Parameters to create a Data Volume from an OCI registry.
type ReducedDataVolumeSourceRegistry struct {
	// The url of the registry source (starting with the scheme: `docker`, `oci-archive`).
	// +optional
	URL *string `json:"url,omitempty"`
	// A reference to the Secret needed to access the Registry source.
	// +optional
	SecretRef *string `json:"secretRef,omitempty"`
	// A reference to the Registry certs.
	// +optional
	CertConfigMap *string `json:"certConfigMap,omitempty"`
}

// Contains enough information to let locate the typed referenced object in the cluster.
type TypedObjectReference struct {
	corev1.TypedLocalObjectReference `json:",inline"`
	// The Namespace of resource being referenced.
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
}

// The observed state of `ClusterVirtualMachineImage`.
type ClusterVirtualMachineImageStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName={"cvmi","cvmimage","cvmimages"}

// Defines remotely available images on cluster level.
type ClusterVirtualMachineImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterVirtualMachineImageSpec   `json:"spec,omitempty"`
	Status ClusterVirtualMachineImageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// Contains a list of `ClusterVirtualMachineImages`.
type ClusterVirtualMachineImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterVirtualMachineImage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterVirtualMachineImage{}, &ClusterVirtualMachineImageList{})
}
