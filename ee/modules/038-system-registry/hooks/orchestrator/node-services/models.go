/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

type Inputs struct {
	Nodes map[string]Node
}

type Params struct {
}

type State struct {
}

type Pod struct {
	Ready   bool
	Version string
}

type hookPod struct {
	Pod
	Node string
}

type NodePods map[string]Pod

type Node struct {
	IP    string   `json:"ip,omitempty"`
	Ready bool     `json:"ready,omitempty"`
	Pods  NodePods `json:"pods,omitempty"`
}
