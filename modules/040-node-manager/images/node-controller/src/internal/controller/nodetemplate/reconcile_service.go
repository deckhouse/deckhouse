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

package nodetemplate

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func (r *Reconciler) reconcileNode(ctx context.Context, node *corev1.Node, ng *v1.NodeGroup) (bool, error) {
	logger := log.FromContext(ctx)

	base := node.DeepCopy()
	working := node.DeepCopy()

	isClusterAPINode := hasKey(working.Annotations, clusterAPIAnnotationKey)

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		fixCloudNodeTaints(working, ng)
		if isClusterAPINode {
			logger.V(1).Info("applying template to ClusterAPI cloud ephemeral node", "node", node.Name, "nodeGroup", ng.Name)
			if err := applyNodeTemplate(working, ng); err != nil {
				return false, err
			}
		} else {
			logger.V(1).Info("cloud ephemeral node (non-CAPI): fixing taints only", "node", node.Name, "nodeGroup", ng.Name)
		}
	} else {
		logger.V(1).Info("applying full template to node", "node", node.Name, "nodeGroup", ng.Name, "nodeType", ng.Spec.NodeType)
		if err := applyNodeTemplate(working, ng); err != nil {
			return false, err
		}
	}

	if ng.Name == "master" {
		logger.V(1).Info("applying master node role labels and fixing master taints", "node", node.Name)
		if working.Labels == nil {
			working.Labels = make(map[string]string)
		}
		working.Labels[controlPlaneTaintKey] = ""
		working.Labels[masterNodeRoleKey] = ""
		if len(working.Spec.Taints) > 0 {
			working.Spec.Taints = fixMasterTaints(working.Spec.Taints, getTemplateTaints(ng))
		}
	}

	if shouldDisableScaleDown(ng.Spec.NodeType) {
		if working.Annotations == nil {
			working.Annotations = make(map[string]string)
		}
		working.Annotations["cluster-autoscaler.kubernetes.io/scale-down-disabled"] = "true"
	}

	if !nodeChanged(base, working) {
		return false, nil
	}

	logger.V(1).Info("patching node with template changes", "node", node.Name, "nodeGroup", ng.Name)
	working.Status = base.Status
	if err := r.Client.Patch(ctx, working, client.MergeFrom(base)); err != nil {
		return false, err
	}

	return true, nil
}
