/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"bytes"
	"crypto/sha256"
	"sort"

	"k8s.io/apimachinery/pkg/types"
)

type egressGatewayState struct {
	Generation                  int64             `json:"generation"`
	Name                        string            `json:"name"`
	Mode                        string            `json:"mode"`
	IP                          string            `json:"ip"`
	RoutingTableName            string            `json:"routingTableName"`
	UID                         types.UID         `json:"uid,omitempty"`
	DesiredActiveNode           string            `json:"desiredActiveNode"`
	AllNodes                    []string          `json:"allNodes"`
	ReadyNodes                  []string          `json:"readyNodes"`
	HealthyNodes                []string          `json:"healthyNodes"`
	HealthyNodesWithEgressAgent []string          `json:"healthyNodesWithEgressAgent"`
	CurrentActiveNodes          []string          `json:"currentActiveNodes"`
	NodeSelector                map[string]string `json:"nodeSelector"`
}

func (eg *egressGatewayState) electDesiredActiveNode() string {
	if len(eg.ReadyNodes) == 0 {
		return ""
	}

	// Algorithm: If the readyNodes slice is empty, then there is no sense to calculate desiredMaster.
	// For each of readyNode, calculate the SHA256 hash of (EgressGatewayGroup.name + Node.Name).
	// Add the hashes to an array and sort it. Select the first element from the resulting slice.
	// Thus we get a load distribution between the ready nodes.

	sort.Slice(eg.ReadyNodes, func(i, j int) bool {
		hi := shaBasedHashFunc(eg.Name + "#" + eg.ReadyNodes[i])
		hj := shaBasedHashFunc(eg.Name + "#" + eg.ReadyNodes[j])
		return bytes.Compare(hi[:], hj[:]) < 0
	})

	eg.DesiredActiveNode = eg.ReadyNodes[0]
	return eg.DesiredActiveNode
}

func shaBasedHashFunc(key string) [32]byte {
	return sha256.Sum256([]byte(key))
}
