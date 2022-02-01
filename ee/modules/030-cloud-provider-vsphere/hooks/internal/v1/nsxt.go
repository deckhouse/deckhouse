/*
Copyright 2021 Flant JSC

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

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VsphereNsxt config
type VsphereNsxt struct {
	// Default IP Address pool for LB
	DefaultIpPoolName string `json:"defaultIpPoolName" yaml:"defaultIpPoolName"`
	// Default profile name for LB
	DefaulTcpAppProfileName string `json:"defaulTcpAppProfileName,omitempty" yaml:"defaulTcpAppProfileName,omitempty"`
	// LB size
	Size string `json:"size" yaml:"size"`
	// NSX-T path to tier1 gateway
	Tier1GatewayPath string `json:"tier1GatewayPath" yaml:"tier1GatewayPath"`
	// NSX-T user
	User string `json:"user" yaml:"user"`
	// NSX-T password
	Password string `json:"password" yaml:"password"`
	// NSX-T host
	Host         string `json:"host" yaml:"host"`
	InsecureFlag bool   `json:"insecureFlag,omitempty" yaml:"insecureFlag,omitempty"`
	// Additional LB classes
	LoadBalancerClass []VsphereNsxtLoadBalancerClass `json:"loadBalancerClass,omitempty" yaml:"loadBalancerClass,omitempty"`
}

// VsphereNsxtLoadBalancerClass
type VsphereNsxtLoadBalancerClass struct {
	// Name of class
	Name string `json:"name" yaml:"name"`
	// IP Address pool for LB
	IpPoolName string `json:"ipPoolName" yaml:"ipPoolName"`
	// Default profile name for LB
	TcpAppProfileName string `json:"tcpAppProfileName,omitempty" yaml:"tcpAppProfileName,omitempty"`
}
