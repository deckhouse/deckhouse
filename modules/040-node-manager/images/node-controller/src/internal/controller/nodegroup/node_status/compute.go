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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// nodeConfigRollout is what the group status reads off the NodeConfig of a node.
type nodeConfigRollout struct {
	applied            bool
	behind             bool
	waitingForApproval bool
}

func (s *Service) getNodesForNodeGroup(ctx context.Context, ngName string) ([]corev1.Node, error) {
	return nodecommon.GetNodesForNodeGroup(ctx, s.Client, ngName)
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (s *Service) getConfigurationChecksum(ctx context.Context, ngName string) string {
	checksums, err := nodecommon.GetConfigurationChecksums(ctx, s.Client)
	if err != nil || checksums == nil {
		return ""
	}
	return checksums[ngName]
}

// getNodeConfigRollout reads the NodeConfig named after the node; a node without one
// reports nothing.
func (s *Service) getNodeConfigRollout(ctx context.Context, nodeName string) (nodeConfigRollout, error) {
	nc, err := nodecommon.GetNodeConfig(ctx, s.Client, nodeName)
	if err != nil || nc == nil {
		return nodeConfigRollout{}, err
	}
	return readNodeConfigRollout(nc), nil
}

func readNodeConfigRollout(nc *internalv1alpha1.NodeConfig) nodeConfigRollout {
	reason, _ := nodecommon.DisruptionApprovalWithheld(nc)
	rollout := nodeConfigRollout{
		behind:             nc.Status.AppliedGeneration < nc.Generation,
		waitingForApproval: reason == internalv1alpha1.NodeConfigReasonApprovalPending,
	}

	// Mirrors applied() in internal/controller/nodeconfig/rollout.go.
	applied := meta.FindStatusCondition(nc.Status.Conditions, internalv1alpha1.NodeConfigConditionTypeConfigurationApplied)
	if applied != nil && applied.Status == metav1.ConditionTrue {
		rollout.applied = nc.Status.AppliedGeneration == nc.Generation && applied.ObservedGeneration == nc.Generation
	}
	return rollout
}

// NodeConfigChanged passes a NodeConfig update that moves what the group status reads:
// the agent republishes its status on every pass, and a status pass of the group lists
// every Machine in the cluster.
func NodeConfigChanged(e event.UpdateEvent) bool {
	before, okOld := e.ObjectOld.(*internalv1alpha1.NodeConfig)
	after, okNew := e.ObjectNew.(*internalv1alpha1.NodeConfig)
	if !okOld || !okNew {
		return true
	}
	if before.Labels[nodecommon.NodeGroupLabel] != after.Labels[nodecommon.NodeGroupLabel] {
		return true
	}
	return readNodeConfigRollout(before) != readNodeConfigRollout(after)
}
