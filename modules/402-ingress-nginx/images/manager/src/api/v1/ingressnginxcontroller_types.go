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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Inlet string

const (
	InletLoadBalancer                   Inlet = "LoadBalancer"
	InletLoadBalancerWithProxyProtocol  Inlet = "LoadBalancerWithProxyProtocol"
	InletLoadBalancerWithSSLPassthrough Inlet = "LoadBalancerWithSSLPassthrough"
	InletHostPort                       Inlet = "HostPort"
	InletHostPortWithProxyProtocol      Inlet = "HostPortWithProxyProtocol"
	InletHostPortWithSSLPassthrough     Inlet = "HostPortWithSSLPassthrough"
	InletHostWithFailover               Inlet = "HostWithFailover"
)

// IngressNginxControllerSpec defines the desired state of IngressNginxController.
// This is a minimal starting point for deckhouse.io/v1 and can be expanded
// as controller logic starts using more fields from the CRD.
type IngressNginxControllerSpec struct {
	IngressClass                   string                                  `json:"ingressClass,omitempty"`
	Inlet                          Inlet                                   `json:"inlet,omitempty"`
	ControllerVersion              string                                  `json:"controllerVersion,omitempty"`
	EnableIstioSidecar             bool                                    `json:"enableIstioSidecar,omitempty"`
	WaitLoadBalancerOnTerminating  *intstr.IntOrString                     `json:"waitLoadBalancerOnTerminating,omitempty"`
	ChaosMonkey                    bool                                    `json:"chaosMonkey,omitempty"`
	ValidationEnabled              bool                                    `json:"validationEnabled,omitempty"`
	AnnotationValidationEnabled    bool                                    `json:"annotationValidationEnabled,omitempty"`
	NodeSelector                   map[string]string                       `json:"nodeSelector,omitempty"`
	Tolerations                    []corev1.Toleration                     `json:"tolerations,omitempty"`
	LoadBalancer                   *IngressNginxControllerLoadBalancerSpec `json:"loadBalancer,omitempty"`
	LoadBalancerWithProxyProtocol  *IngressNginxControllerLoadBalancerSpec `json:"loadBalancerWithProxyProtocol,omitempty"`
	LoadBalancerWithSSLPassthrough *IngressNginxControllerLoadBalancerSpec `json:"loadBalancerWithSSLPassthrough,omitempty"`
	HostPort                       *IngressNginxControllerHostPortSpec     `json:"hostPort,omitempty"`
	HostPortWithProxyProtocol      *IngressNginxControllerHostPortSpec     `json:"hostPortWithProxyProtocol,omitempty"`
	HostPortWithSSLPassthrough     *IngressNginxControllerHostPortSpec     `json:"hostPortWithSSLPassthrough,omitempty"`
}

type IngressNginxControllerLoadBalancerSpec struct {
	SourceRanges              []string          `json:"sourceRanges,omitempty"`
	Annotations               map[string]string `json:"annotations,omitempty"`
	LoadBalancerClass         string            `json:"loadBalancerClass,omitempty"`
	HTTPPort                  *int32            `json:"httpPort,omitempty"`
	HTTPSPort                 *int32            `json:"httpsPort,omitempty"`
	BehindL7Proxy             bool              `json:"behindL7Proxy,omitempty"`
	RealIPHeader              string            `json:"realIPHeader,omitempty"`
	AcceptClientIPHeadersFrom []string          `json:"acceptClientIPHeadersFrom,omitempty"`
}

type IngressNginxControllerHostPortSpec struct {
	HTTPPort                  *int32   `json:"httpPort,omitempty"`
	HTTPSPort                 *int32   `json:"httpsPort,omitempty"`
	BehindL7Proxy             bool     `json:"behindL7Proxy,omitempty"`
	RealIPHeader              string   `json:"realIPHeader,omitempty"`
	AcceptClientIPHeadersFrom []string `json:"acceptClientIPHeadersFrom,omitempty"`
}

type IngressNginxControllerStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ingressnginxcontrollers,singular=ingressnginxcontroller,scope=Cluster

type IngressNginxController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IngressNginxControllerSpec   `json:"spec,omitempty"`
	Status IngressNginxControllerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type IngressNginxControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IngressNginxController `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IngressNginxController{}, &IngressNginxControllerList{})
}
