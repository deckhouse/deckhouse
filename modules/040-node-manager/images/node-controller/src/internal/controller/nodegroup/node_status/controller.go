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

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
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

func (s *Service) Compute(ctx context.Context, ng *v1.NodeGroup) (Result, error) {
	nodes, err := s.getNodesForNodeGroup(ctx, ng.Name)
	if err != nil {
		return Result{}, err
	}

	// An Immutable node reports its rollout in its NodeConfig and never carries the
	// bashible checksum annotation.
	immutable := ng.Spec.SystemType == v1.SystemTypeImmutable
	configChecksum := ""
	if !immutable {
		configChecksum = s.getConfigurationChecksum(ctx, ng.Name)
	}
	result := Result{
		NodesForConditions: make([]*conditionscalc.Node, 0, len(nodes)),
	}

	for _, node := range nodes {
		result.NodesCount++
		if isNodeReady(&node) {
			result.ReadyCount++
		}
		conditionsNode := conditionscalc.NodeToConditionsNode(&node)
		result.NodesForConditions = append(result.NodesForConditions, conditionsNode)

		if !immutable {
			if configChecksum != "" && node.Annotations[nodecommon.ConfigurationChecksumAnnotation] == configChecksum {
				result.UpToDateCount++
			}
			continue
		}

		rollout, err := s.getNodeConfigRollout(ctx, node.Name)
		if err != nil {
			return Result{}, err
		}
		if rollout.applied {
			result.UpToDateCount++
		}
		if rollout.behind {
			conditionsNode.Updating = true
		}
		if rollout.waitingForApproval {
			conditionsNode.WaitingDisruptiveApproval = true
			conditionsNode.Updating = true
		}
	}

	return result, nil
}
