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
	. "github.com/deckhouse/deckhouse/egress-gateway-agent/pkg/apis/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SDNInternalEgressGatewaySourceIP struct {
	Mode                                    SourceIPMode                                `json:"mode"`
	VirtualIPAddress                        VirtualIPAddressSpec                        `json:"virtualIPAddress,omitempty"`
	PrimaryIPFromEgressGatewayNodeInterface PrimaryIPFromEgressGatewayNodeInterfaceSpec `json:"primaryIPFromEgressGatewayNodeInterface,omitempty"`
}

// SDNInternalEgressGatewayInstanceSpec defines the desired state of SDNInternalEgressGatewayInstance
type SDNInternalEgressGatewayInstanceSpec struct {
	NodeName string                           `json:"nodeName,omitempty"`
	SourceIP SDNInternalEgressGatewaySourceIP `json:"sourceIP,omitempty"`
}

// SDNInternalEgressGatewayInstanceStatus defines the observed state of SDNInternalEgressGatewayInstance
type SDNInternalEgressGatewayInstanceStatus struct {
	ObservedGeneration int64               `json:"observedGeneration,omitempty"`
	Conditions         []ExtendedCondition `json:"conditions,omitempty"`
}

// SDNInternalEgressGatewayInstance is the Schema for the egressgatewayinstances API
type SDNInternalEgressGatewayInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SDNInternalEgressGatewayInstanceSpec   `json:"spec,omitempty"`
	Status SDNInternalEgressGatewayInstanceStatus `json:"status,omitempty"`
}

// SDNInternalEgressGatewayInstanceList contains a list of SDNInternalEgressGatewayInstance
type SDNInternalEgressGatewayInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SDNInternalEgressGatewayInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SDNInternalEgressGatewayInstance{}, &SDNInternalEgressGatewayInstanceList{})
}
