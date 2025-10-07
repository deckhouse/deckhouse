/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

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
