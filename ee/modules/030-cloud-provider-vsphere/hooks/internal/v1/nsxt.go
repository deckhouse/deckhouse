/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

// VsphereNsxt config
type VsphereNsxt struct {
	// Default IP Address pool for LB
	DefaultIPPoolName *string `json:"defaultIpPoolName" yaml:"defaultIpPoolName"`
	// Default profile name for LB
	DefaultTCPAppProfileName *string `json:"defaultTcpAppProfileName,omitempty" yaml:"defaultTcpAppProfileName,omitempty"`
	// LB size
	Size *string `json:"size" yaml:"size"`
	// NSX-T path to tier1 gateway
	Tier1GatewayPath *string `json:"tier1GatewayPath" yaml:"tier1GatewayPath"`
	// NSX-T user
	User *string `json:"user" yaml:"user"`
	// NSX-T password
	Password *string `json:"password" yaml:"password"`
	// NSX-T host
	Host         *string `json:"host" yaml:"host"`
	InsecureFlag *bool   `json:"insecureFlag,omitempty" yaml:"insecureFlag,omitempty"`
	// Additional LB classes
	LoadBalancerClass *[]VsphereNsxtLoadBalancerClass `json:"loadBalancerClass,omitempty" yaml:"loadBalancerClass,omitempty"`
}

// VsphereNsxtLoadBalancerClass
type VsphereNsxtLoadBalancerClass struct {
	// Name of class
	Name *string `json:"name" yaml:"name"`
	// IP Address pool for LB
	IPPoolName *string `json:"ipPoolName" yaml:"ipPoolName"`
	// Default profile name for LB
	TCPAppProfileName *string `json:"tcpAppProfileName,omitempty" yaml:"tcpAppProfileName,omitempty"`
}
