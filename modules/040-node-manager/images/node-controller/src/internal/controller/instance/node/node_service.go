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
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
)

func ReconcileNode(ctx context.Context, c client.Client, name string) error {
	logger := log.FromContext(ctx).WithValues("node", name)
	logger.V(4).Info("tick", "op", "node.reconcile.start")

	node := &corev1.Node{}

	err := c.Get(ctx, types.NamespacedName{Name: name}, node)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		result, err := deleteNodeBasedInstanceIfExists(ctx, c, name)
		if err != nil {
			return err
		}

		logger.V(1).Info("node not found, node based instance delete handled", "instance", name, "deleted", result.InstanceDeleted)
		return nil
	}

	if !instancecommon.IsStaticNode(node) {
		logger.V(4).Info("node is not static, skipping instance reconcile")
		return nil
	}

	logger.V(4).Info("tick", "op", "node.instance.ensure")
	if err := instancecommon.EnsureInstanceExists(ctx, c, node.Name, deckhousev1alpha2.InstanceSpec{
		NodeRef: deckhousev1alpha2.NodeRef{Name: node.Name},
	}); err != nil {
		return fmt.Errorf("ensure instance for static node %q: %w", node.Name, err)
	}

	if err := instancecommon.ApplyInstancePhase(ctx, c, node.Name, deckhousev1alpha2.InstancePhaseRunning); err != nil {
		return fmt.Errorf("set instance phase for static node %q: %w", node.Name, err)
	}

	logger.V(1).Info("instance ensured for static node")
	return nil
}

type nodeBasedInstanceDeletionResult struct {
	InstanceDeleted bool
}

func deleteNodeBasedInstanceIfExists(ctx context.Context, c client.Client, name string) (nodeBasedInstanceDeletionResult, error) {
	instance := &deckhousev1alpha2.Instance{}

	err := c.Get(ctx, types.NamespacedName{Name: name}, instance)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nodeBasedInstanceDeletionResult{}, nil
		}
		return nodeBasedInstanceDeletionResult{}, fmt.Errorf("get instance %q: %w", name, err)
	}

	// Delete only instances that are explicitly sourced from Node.
	// This protects machine-backed instances and malformed objects.
	isNodeBased := instance.Spec.MachineRef == nil
	pointsToThisNode := instance.Spec.NodeRef.Name == name
	if !isNodeBased || !pointsToThisNode {
		return nodeBasedInstanceDeletionResult{}, nil
	}

	err = instancecommon.RemoveInstanceControllerFinalizer(ctx, c, instance)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nodeBasedInstanceDeletionResult{}, nil
		}

		return nodeBasedInstanceDeletionResult{}, fmt.Errorf("remove finalizer from node based instance %q: %w", name, err)
	}

	err = c.Delete(ctx, instance)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nodeBasedInstanceDeletionResult{}, nil
		}

		return nodeBasedInstanceDeletionResult{}, fmt.Errorf("delete node based instance %q: %w", name, err)
	}

	log.FromContext(ctx).V(1).Info(
		"instance deleted",
		"instance", name,
		"deletedBy", "node-controller",
		"reason", "node-not-found-for-node-source",
	)
	log.FromContext(ctx).V(4).Info("tick", "op", "node.instance.delete")

	return nodeBasedInstanceDeletionResult{InstanceDeleted: true}, nil
}
