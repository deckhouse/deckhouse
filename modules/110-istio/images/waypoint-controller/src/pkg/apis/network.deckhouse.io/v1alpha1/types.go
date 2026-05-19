/*
Copyright 2026 Flant JSC

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
)

// +k8s:deepcopy-gen=true
type WaypointInstanceSpec struct {
	WaypointFor         string               `json:"waypointFor,omitempty"`
	NodeSelector        map[string]string    `json:"nodeSelector,omitempty"`
	Tolerations         []corev1.Toleration  `json:"tolerations,omitempty"`
	ReplicasManagement  *ReplicasManagement  `json:"replicasManagement,omitempty"`
	ResourcesManagement *ResourcesManagement `json:"resourcesManagement,omitempty"`
	AllowedRoutes       *AllowedRoutesConfig `json:"allowedRoutes,omitempty"`
}

// +k8s:deepcopy-gen=true
type ReplicasManagement struct {
	Mode   string          `json:"mode,omitempty"`
	Static *ReplicasStatic `json:"static,omitempty"`
	HPA    *ReplicasHPA    `json:"hpa,omitempty"`
}

// +k8s:deepcopy-gen=true
type ReplicasStatic struct {
	Replicas int32 `json:"replicas"`
}

// +k8s:deepcopy-gen=true
type ReplicasHPA struct {
	MinReplicas int32       `json:"minReplicas"`
	MaxReplicas int32       `json:"maxReplicas"`
	Metrics     []HPAMetric `json:"metrics"`
}

// +k8s:deepcopy-gen=true
type HPAMetric struct {
	Type                     string `json:"type"`
	TargetAverageUtilization int32  `json:"targetAverageUtilization"`
}

// +k8s:deepcopy-gen=true
type ResourcesManagement struct {
	Mode   string           `json:"mode,omitempty"`
	VPA    *ResourcesVPA    `json:"vpa,omitempty"`
	Static *ResourcesStatic `json:"static,omitempty"`
}

// +k8s:deepcopy-gen=true
type ResourcesVPA struct {
	Mode   string       `json:"mode,omitempty"`
	CPU    *VPAResource `json:"cpu,omitempty"`
	Memory *VPAResource `json:"memory,omitempty"`
}

// +k8s:deepcopy-gen=true
type VPAResource struct {
	Min        string   `json:"min,omitempty"`
	Max        string   `json:"max,omitempty"`
	LimitRatio *float64 `json:"limitRatio,omitempty"`
}

// +k8s:deepcopy-gen=true
type ResourcesStatic struct {
	Requests *ResourcesRequestsLimits `json:"requests,omitempty"`
	Limits   *ResourcesRequestsLimits `json:"limits,omitempty"`
}

// +k8s:deepcopy-gen=true
type ResourcesRequestsLimits struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// +k8s:deepcopy-gen=true
type AllowedRoutesConfig struct {
	Namespaces *RouteNamespacesConfig `json:"namespaces,omitempty"`
}

// +k8s:deepcopy-gen=true
type RouteNamespacesConfig struct {
	From     *string               `json:"from,omitempty"`
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// +k8s:deepcopy-gen=true
type WaypointInstanceStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	Synced             bool  `json:"synced,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WaypointInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WaypointInstanceSpec   `json:"spec,omitempty"`
	Status            WaypointInstanceStatus `json:"status,omitempty"`
}

func (in *WaypointInstance) Hub() {}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WaypointInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WaypointInstance `json:"items"`
}
