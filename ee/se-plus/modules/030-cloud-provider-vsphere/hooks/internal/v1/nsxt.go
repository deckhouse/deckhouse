/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

// VsphereNsxt config
type VsphereNsxt struct {
	// Default IP Address pool for LB
	DefaultIPPoolName *string `json:"defaultIpPoolName,omitempty" yaml:"defaultIpPoolName,omitempty"`
	// Default profile name for LB
	DefaultTCPAppProfileName *string `json:"defaultTcpAppProfileName,omitempty" yaml:"defaultTcpAppProfileName,omitempty"`
	DefaultUDPAppProfileName *string `json:"defaultUdpAppProfileName,omitempty" yaml:"defaultUdpAppProfileName,omitempty"`
	// LB size
	Size *string `json:"size,omitempty" yaml:"size,omitempty"`
	// NSX-T path to tier1 gateway
	Tier1GatewayPath *string `json:"tier1GatewayPath,omitempty" yaml:"tier1GatewayPath,omitempty"`
	// NSX-T user
	User *string `json:"user,omitempty" yaml:"user,omitempty"`
	// NSX-T password
	Password *string `json:"password,omitempty" yaml:"password,omitempty"`
	// NSX-T host
	Host         *string `json:"host,omitempty" yaml:"host,omitempty"`
	InsecureFlag *bool   `json:"insecureFlag,omitempty" yaml:"insecureFlag,omitempty"`
	// Additional LB classes
	LoadBalancerClass *[]VsphereNsxtLoadBalancerClass `json:"loadBalancerClass,omitempty" yaml:"loadBalancerClass,omitempty"`
}

// VsphereNsxtLoadBalancerClass
type VsphereNsxtLoadBalancerClass struct {
	// Name of class
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// IP Address pool for LB
	IPPoolName *string `json:"ipPoolName,omitempty" yaml:"ipPoolName,omitempty"`
	// Default profile name for LB
	TCPAppProfileName *string `json:"tcpAppProfileName,omitempty" yaml:"tcpAppProfileName,omitempty"`
	UDPAppProfileName *string `json:"udpAppProfileName,omitempty" yaml:"udpAppProfileName,omitempty"`
}
