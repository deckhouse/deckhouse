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
