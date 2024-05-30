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

type EgressGatewayPolicySpec struct {
	EgressGatewayName string     `json:"egressGatewayName"`
	Selectors         []Selector `json:"selectors,omitempty"`
	DestinationCIDRs  []string   `json:"destinationCIDRs,omitempty"`
	ExcludedCIDRs     []string   `json:"excludedCIDRs,omitempty"`
}

type Selector struct {
	PodSelector metav1.LabelSelector `json:"podSelector"`
}

type EgressGatewayPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

type EgressGatewayPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EgressGatewayPolicySpec   `json:"spec,omitempty"`
	Status EgressGatewayPolicyStatus `json:"status,omitempty"`
}
