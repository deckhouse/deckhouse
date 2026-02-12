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

package common

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

const InstanceControllerFinalizer = "node-manager.hooks.deckhouse.io/instance-controller"

func EnsureInstanceExists(
	ctx context.Context,
	c client.Client,
	name string,
	spec deckhousev1alpha2.InstanceSpec,
) (*deckhousev1alpha2.Instance, error) {
	instance := &deckhousev1alpha2.Instance{}
	if err := c.Get(ctx, types.NamespacedName{Name: name}, instance); err == nil {
		return instance, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("get instance %q: %w", name, err)
	}

	newInstance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}

	if err := c.Create(ctx, newInstance); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if err := c.Get(ctx, types.NamespacedName{Name: name}, instance); err != nil {
				return nil, fmt.Errorf("get instance %q after already exists: %w", name, err)
			}
			return instance, nil
		}

		return nil, fmt.Errorf("create instance %q: %w", name, err)
	}

	return newInstance, nil
}

func IsInstanceConditionTrue(conditions []deckhousev1alpha2.InstanceCondition, conditionType string) bool {
	for i := range conditions {
		if conditions[i].Type == conditionType && conditions[i].Status == metav1.ConditionTrue {
			return true
		}
	}

	return false
}

func GetInstanceConditionByType(
	conditions []deckhousev1alpha2.InstanceCondition,
	conditionType string,
) (deckhousev1alpha2.InstanceCondition, bool) {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return conditions[i], true
		}
	}

	return deckhousev1alpha2.InstanceCondition{}, false
}

func SetInstancePhase(
	ctx context.Context,
	c client.Client,
	instance *deckhousev1alpha2.Instance,
	phase deckhousev1alpha2.InstancePhase,
) error {
	if instance.Status.Phase == phase {
		return nil
	}

	updated := instance.DeepCopy()
	updated.Status.Phase = phase
	log.FromContext(ctx).V(4).Info("tick", "op", "instance.phase.patch")
	if err := c.Status().Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return fmt.Errorf("patch instance %q phase to %q: %w", instance.Name, phase, err)
	}

	return nil
}

func RemoveInstanceControllerFinalizer(
	ctx context.Context,
	c client.Client,
	instance *deckhousev1alpha2.Instance,
) error {
	if !controllerutil.ContainsFinalizer(instance, InstanceControllerFinalizer) {
		return nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "instance.finalizer.remove.patch")

	updated := instance.DeepCopy()
	controllerutil.RemoveFinalizer(updated, InstanceControllerFinalizer)
	if err := c.Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return fmt.Errorf("remove finalizer from instance %q: %w", instance.Name, err)
	}

	*instance = *updated
	return nil
}
