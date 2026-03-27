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

package node

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/common"
)

func (r *NodeReconciler) deleteNodeBasedInstanceIfExists(ctx context.Context, name string) (bool, error) {
	instance := &deckhousev1alpha2.Instance{}
	if err := r.Get(ctx, types.NamespacedName{Name: name}, instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, fmt.Errorf("get instance %q: %w", name, err)
	}

	// Delete only instances that are explicitly sourced from Node.
	// This protects machine-backed instances and malformed objects.
	isNodeBased := instance.Spec.MachineRef == nil
	pointsToThisNode := instance.Spec.NodeRef.Name == name
	if !isNodeBased || !pointsToThisNode {
		return false, nil
	}

	if err := common.RemoveInstanceControllerFinalizer(ctx, r.Client, instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, fmt.Errorf("remove finalizer from node based instance %q: %w", name, err)
	}

	if err := r.Delete(ctx, instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, fmt.Errorf("delete node based instance %q: %w", name, err)
	}
	log.FromContext(ctx).V(1).Info(
		"instance deleted",
		"instance", name,
		"deletedBy", "node-controller",
		"reason", "node-not-found-for-node-source",
	)
	log.FromContext(ctx).V(4).Info("tick", "op", "node.instance.delete")

	return true, nil
}

func IsStaticNode(node *corev1.Node) bool {
	if _, hasCAPIMachineAnnotation := node.Annotations[capiMachineAnnotationKey]; hasCAPIMachineAnnotation {
		return false
	}

	nodeType := node.Labels[nodeTypeLabelKey]
	return nodeType == staticNodeTypeValue || nodeType == cloudPermanentNodeTypeValue
}
