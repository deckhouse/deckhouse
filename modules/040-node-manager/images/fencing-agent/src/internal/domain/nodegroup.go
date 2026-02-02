package domain

type NetworkInterface string

type NodeName string

type NodeGroup struct {
	NodesInNetworks map[NetworkInterface]NodesInNetwork
}

type NodesInNetwork struct {
	Members map[NodeName]Node
	Size    int
}

// v1 hardcode -> v2 should be configurable
func (ng *NodeGroup) HasQuorum() bool {
	return len(ng.NodesInNetworks[InterfaceName].Members) < (ng.NodesInNetworks[InterfaceName].Size/2 + 1)
}
