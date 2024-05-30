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

type SourceIPMode string

const (
	VirtualIPAddress                    SourceIPMode = "VirtualIPAddress"
	PrimaryIPFromEgressGatewayInterface SourceIPMode = "PrimaryIPFromEgressGatewayNodeInterface"
)

type EgressGatewaySpec struct {
	NodeSelector map[string]string     `json:"nodeSelector,omitempty"`
	SourceIP     EgressGatewaySourceIP `json:"sourceIP"`
}

type EgressGatewaySourceIP struct {
	Mode                                    SourceIPMode                                `json:"mode"`
	VirtualIPAddress                        VirtualIPAddressSpec                        `json:"virtualIPAddress,omitempty"`
	PrimaryIPFromEgressGatewayNodeInterface PrimaryIPFromEgressGatewayNodeInterfaceSpec `json:"primaryIPFromEgressGatewayNodeInterface,omitempty"`
}

type VirtualIPAddressSpec struct {
	IP               string `json:"ip,omitempty"`
	RoutingTableName string `json:"routingTableName,omitempty"`
}

type PrimaryIPFromEgressGatewayNodeInterfaceSpec struct {
	InterfaceName string `json:"interfaceName,omitempty"`
}

type EgressGatewayStatus struct {
	ReadyNodes         int64               `json:"readyNodes,omitempty"`
	ObservedGeneration int64               `json:"observedGeneration,omitempty"`
	ActiveNodeName     string              `json:"activeNodeName,omitempty"`
	Conditions         []ExtendedCondition `json:"conditions,omitempty"`
}

type EgressGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EgressGatewaySpec   `json:"spec,omitempty"`
	Status EgressGatewayStatus `json:"status,omitempty"`
}

// EgressGatewayList contains a list of EgressGateway
type EgressGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EgressGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EgressGateway{}, &EgressGatewayList{})
}
