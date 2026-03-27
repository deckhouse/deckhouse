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

package instance

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

func (s *InstanceService) reconcileLinkedSourceExistence(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	source := getInstanceSource(instance)
	logger := log.FromContext(ctx)

	machineExists, machineNotFound, err := s.linkedMachineExists(ctx, source.MachineRef)
	if err != nil {
		return false, err
	}
	if machineExists {
		return false, nil
	}
	nodeExists, nodeNotFound, err := s.linkedNodeExists(ctx, source.NodeName)
	if err != nil {
		return false, err
	}
	if nodeExists {
		return false, nil
	}
	if !machineNotFound || !nodeNotFound {
		logger.V(1).Info(
			"linked resources are not confirmed missing, skip delete",
			"instance", instance.Name,
			"machineNotFound", machineNotFound,
			"nodeNotFound", nodeNotFound,
			"machineRefName", source.MachineRef.Name,
			"nodeName", source.NodeName,
		)
		return false, nil
	}

	// Safety net: this delete path is best-effort garbage collection for orphaned instances.
	// Remove finalizer first to avoid machine deletion side-effects on an erroneous source miss.
	if err := s.removeInstanceFinalizer(ctx, instance); err != nil {
		return false, fmt.Errorf("remove finalizer before deleting instance %q with missing source: %w", instance.Name, err)
	}

	if err := s.client.Delete(ctx, instance); err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("delete instance %q with missing source: %w", instance.Name, err)
	}
	logger.V(1).Info(
		"instance deleted",
		"instance", instance.Name,
		"sourceType", source.Type,
		"machineRefName", source.MachineRef.Name,
		"nodeName", source.NodeName,
		"finalizerRemoved", true,
		"deletedBy", "instance-controller",
		"reason", "linked-source-not-found",
	)
	logger.V(4).Info("tick", "op", "instance.source.delete")

	return true, nil
}

func (s *InstanceService) linkedMachineExists(
	ctx context.Context,
	ref *deckhousev1alpha2.MachineRef,
) (bool, bool, error) {
	if ref == nil || ref.Name == "" {
		return false, false, nil
	}
	logger := log.FromContext(ctx)

	if _, machineErr := s.machineFactory.NewMachineFromRef(ctx, s.client, ref); machineErr != nil {
		if apierrors.IsNotFound(machineErr) {
			logger.V(1).Info(
				"linked source is missing",
				"sourceType", instanceSourceMachine,
				"missingObject", "machine",
				"machineRefName", ref.Name,
			)
			return false, true, nil
		}
		return false, false, machineErr
	}

	return true, false, nil
}

func (s *InstanceService) linkedNodeExists(ctx context.Context, nodeName string) (bool, bool, error) {
	logger := log.FromContext(ctx)
	if nodeName == "" {
		return false, false, nil
	}

	node := &corev1.Node{}
	if err := s.client.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info(
				"linked source is missing",
				"sourceType", instanceSourceNode,
				"missingObject", "node",
				"nodeName", nodeName,
			)
			return false, true, nil
		}
		return false, false, fmt.Errorf("get node %q: %w", nodeName, err)
	}

	return true, false, nil
}
