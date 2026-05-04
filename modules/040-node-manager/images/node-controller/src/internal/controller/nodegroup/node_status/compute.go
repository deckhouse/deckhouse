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

	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

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
