/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package v1 mirrors the ZvirtInstanceClass spec for the module hooks.
//
// The canonical, marker-annotated definition lives in pkg/api/instanceclass/v1 and drives
// crds/instance_class.yaml. Hooks compile inside the repository-root go.mod, which does not
// require the module's own go.mod, so the shape is repeated here without the generator markers.
//
// Only v1 is mirrored: v1alpha1 is frozen and the migration creates v1 objects exclusively.
package v1

const (
	ZvirtInstanceClassAPIVersion = "deckhouse.io/v1"
	ZvirtInstanceClassKind       = "ZvirtInstanceClass"
)

// InstanceClassSpec is the spec of a ZvirtInstanceClass created by the migration.
//
// Every field is optional on the wire so that a value absent from the legacy
// ZvirtClusterConfiguration is not emitted as a zero value: the CRD documents defaults for
// rootDiskSizeGb and etcdDiskSizeGb but declares no `default:`, so an emitted 0 would be stored
// verbatim and would not mean "use the default".
type InstanceClassSpec struct {
	NumCPUs             int                  `json:"numCPUs,omitempty"`
	Memory              int                  `json:"memory,omitempty"`
	RootDiskSizeGb      *int                 `json:"rootDiskSizeGb,omitempty"`
	EtcdDiskSizeGb      *int                 `json:"etcdDiskSizeGb,omitempty"`
	Template            string               `json:"template,omitempty"`
	VNICProfileID       string               `json:"vnicProfileID,omitempty"`
	StorageDomainID     string               `json:"storageDomainID,omitempty"`
	CustomNetworkConfig *CustomNetworkConfig `json:"customNetworkConfig,omitempty"`
}

// CustomNetworkConfig describes a static network configuration of the node group interfaces.
//
// DNSServers is a list here, while the legacy ZvirtClusterConfiguration stores a single
// space-separated string — the projection splits it.
type CustomNetworkConfig struct {
	NetworkInterfaceName    string   `json:"networkInterfaceName,omitempty"`
	NetworkInterfaceAddress []string `json:"networkInterfaceAddress,omitempty"`
	NetworkInterfaceNetmask string   `json:"networkInterfaceNetmask,omitempty"`
	NetworkInterfaceGateway string   `json:"networkInterfaceGateway,omitempty"`
	DNSServers              []string `json:"dnsServers,omitempty"`
}
