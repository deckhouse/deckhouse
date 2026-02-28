/*
Copyright 2025 Flant JSC

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

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	nodecontroller "github.com/deckhouse/node-controller/internal/controller/node"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (r *InstanceReconciler) reconcileLinkedSourceExistence(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	var err error
	var exists bool
	source := getInstanceSource(instance)

	switch source.Type {
	case instanceSourceMachine:
		if _, machineErr := r.machineFactory.NewMachineFromRef(ctx, r.Client, source.MachineRef); machineErr != nil {
			if apierrors.IsNotFound(machineErr) {
				exists = false
				break
			}
			return false, machineErr
		}
		exists = true
	case instanceSourceNode:
		exists, err = r.linkedNodeExists(ctx, source.NodeName)
	default:
		return false, nil
	}

	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	if err := r.Delete(ctx, instance); err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("delete instance %q with missing source: %w", instance.Name, err)
	}

	return true, nil
}

func (r *InstanceReconciler) linkedNodeExists(ctx context.Context, nodeName string) (bool, error) {
	node := &corev1.Node{}
	if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get node %q: %w", nodeName, err)
	}

	if !nodecontroller.IsStaticNode(node) {
		return false, nil
	}

	return true, nil
}
