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
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
)

type sourceStatus string

const (
	sourceStatusFound    sourceStatus = "found"
	sourceStatusNotFound sourceStatus = "not-found"
	sourceStatusSkipped  sourceStatus = "skipped"
)

type SourceExistenceResult struct {
	InstanceDeleted bool
}

func (s *InstanceService) ReconcileSourceExistence(ctx context.Context, instance *deckhousev1alpha2.Instance) (SourceExistenceResult, error) {
	source := getInstanceSource(instance)
	logger := log.FromContext(ctx)

	machineStatus := sourceStatusSkipped
	nodeStatus := sourceStatusSkipped
	var err error
	switch source.Type {
	case instanceSourceMachine:
		machineStatus, err = s.linkedMachineStatus(ctx, source.MachineRef)
	case instanceSourceNode:
		nodeStatus, err = s.linkedNodeStatus(ctx, source.NodeName)
	default:
		return SourceExistenceResult{}, nil
	}
	if err != nil {
		return SourceExistenceResult{}, err
	}

	if machineStatus == sourceStatusFound || nodeStatus == sourceStatusFound {
		return SourceExistenceResult{}, nil
	}

	hasConfirmedMissing := machineStatus == sourceStatusNotFound || nodeStatus == sourceStatusNotFound

	if !hasConfirmedMissing {
		logger.V(1).Info(
			"linked resources are not confirmed missing, skip delete",
			"instance", instance.Name,
			"machineStatus", machineStatus,
			"nodeStatus", nodeStatus,
			"machineRefName", machineRefName(source.MachineRef),
			"nodeName", source.NodeName,
		)
		return SourceExistenceResult{}, nil
	}

	// Safety net: this delete path is best-effort garbage collection for orphaned instances.
	// Remove finalizer first to avoid machine deletion side-effects on an erroneous source miss.
	if err := s.removeInstanceFinalizer(ctx, instance); err != nil {
		return SourceExistenceResult{}, fmt.Errorf("remove finalizer before deleting instance %q with missing source: %w", instance.Name, err)
	}

	err = s.client.Delete(ctx, instance)
	if err != nil && !apierrors.IsNotFound(err) {
		return SourceExistenceResult{}, fmt.Errorf("delete instance %q with missing source: %w", instance.Name, err)
	}

	logger.V(1).Info(
		"instance deleted",
		"instance", instance.Name,
		"sourceType", source.Type,
		"machineRefName", machineRefName(source.MachineRef),
		"nodeName", source.NodeName,
		"finalizerRemoved", true,
		"deletedBy", "instance-controller",
		"reason", "linked-source-not-found",
	)
	logger.V(4).Info("tick", "op", "instance.source.delete")

	return SourceExistenceResult{InstanceDeleted: true}, nil
}

func (s *InstanceService) linkedMachineStatus(ctx context.Context, ref *deckhousev1alpha2.MachineRef) (sourceStatus, error) {
	if ref == nil || ref.Name == "" {
		return sourceStatusSkipped, nil
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
			return sourceStatusNotFound, nil
		}
		return "", fmt.Errorf("get machine %q: %w", ref.Name, machineErr)
	}

	return sourceStatusFound, nil
}

func machineRefName(ref *deckhousev1alpha2.MachineRef) string {
	if ref == nil {
		return ""
	}

	return ref.Name
}

func (s *InstanceService) linkedNodeStatus(ctx context.Context, nodeName string) (sourceStatus, error) {
	logger := log.FromContext(ctx)
	if nodeName == "" {
		return sourceStatusSkipped, nil
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
			return sourceStatusNotFound, nil
		}
		return "", fmt.Errorf("get node %q: %w", nodeName, err)
	}

	if !instancecommon.IsStaticNode(node) {
		logger.V(1).Info(
			"linked source is no longer static",
			"sourceType", instanceSourceNode,
			"nodeName", nodeName,
		)
		return sourceStatusNotFound, nil
	}

	return sourceStatusFound, nil
}
