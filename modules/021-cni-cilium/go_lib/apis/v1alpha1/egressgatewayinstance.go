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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EgressGatewayInstanceSpec defines the desired state of EgressGatewayInstance
type EgressGatewayInstanceSpec struct {
	NodeName string                `json:"nodeName,omitempty"`
	SourceIP EgressGatewaySourceIP `json:"sourceIP,omitempty"`
}

// EgressGatewayInstanceStatus defines the observed state of EgressGatewayInstance
type EgressGatewayInstanceStatus struct {
	ObservedGeneration int64               `json:"observedGeneration,omitempty"`
	Conditions         []ExtendedCondition `json:"conditions,omitempty"`
}

// EgressGatewayInstance is the Schema for the egressgatewayinstances API
type EgressGatewayInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EgressGatewayInstanceSpec   `json:"spec,omitempty"`
	Status EgressGatewayInstanceStatus `json:"status,omitempty"`
}

// EgressGatewayInstanceList contains a list of EgressGatewayInstance
type EgressGatewayInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EgressGatewayInstance `json:"items"`
}

type ExtendedCondition struct {
	metav1.Condition  `json:",inline"`
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime,omitempty"`
}

func init() {
	SchemeBuilder.Register(&EgressGatewayInstance{}, &EgressGatewayInstanceList{})
}
