/*
Copyright 2026 Flant JSC

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

package node_status

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	conditionscalc "github.com/deckhouse/node-controller/internal/controller/nodegroup/conditionscalc"
)

type Service struct {
	Client client.Client
}

type Result struct {
	NodesCount         int32
	ReadyCount         int32
	UpToDateCount      int32
	NodesForConditions []*conditionscalc.Node
}

func (s *Service) Compute(ctx context.Context, nodeGroupName string) (Result, error) {
	nodes, err := s.getNodesForNodeGroup(ctx, nodeGroupName)
	if err != nil {
		return Result{}, err
	}

	configChecksum := s.getConfigurationChecksum(ctx, nodeGroupName)
	result := Result{
		NodesForConditions: make([]*conditionscalc.Node, 0, len(nodes)),
	}

	for _, node := range nodes {
		result.NodesCount++
		if isNodeReady(&node) {
			result.ReadyCount++
		}
		result.NodesForConditions = append(result.NodesForConditions, conditionscalc.NodeToConditionsNode(&node))

		if configChecksum != "" && node.Annotations["node.deckhouse.io/configuration-checksum"] == configChecksum {
			result.UpToDateCount++
		}
	}

	return result, nil
}
