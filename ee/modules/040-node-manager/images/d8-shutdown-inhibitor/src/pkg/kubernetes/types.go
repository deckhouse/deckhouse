package kubernetes

type NodeGroup struct {
	Status NodeGroupStatus `json:"status,omitempty"`
}

type NodeGroupStatus struct {
	// Number of Kubernetes nodes (in any state) in the group.
	Nodes int32 `json:"nodes,omitempty"`
}

type NodeList struct {
	Items []struct{} `json:"items"`
}
