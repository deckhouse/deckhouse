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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
)

func (r *InstanceReconciler) createInstance(ctx context.Context, machine *mcmv1alpha1.Machine, ng *deckhousev1.NodeGroup) error {
	ngName := machine.Spec.NodeTemplateSpec.Labels[nodeGroupLabel]

	instance := &deckhousev1alpha1.Instance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Instance",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: machine.Name,
			Labels: map[string]string{
				nodeGroupLabel: ngName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "deckhouse.io/v1",
					BlockOwnerDeletion: ptr.To(false),
					Controller:         ptr.To(false),
					Kind:               "NodeGroup",
					Name:               ng.Name,
					UID:                ng.UID,
				},
			},
			Finalizers: []string{finalizerName},
		},
	}

	if err := r.Client.Create(ctx, instance); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create instance %s: %w", machine.Name, err)
	}

	instance.Status = buildInitialStatus(machine, ng)
	if err := r.Client.Status().Update(ctx, instance); err != nil {
		return fmt.Errorf("set initial instance status %s: %w", machine.Name, err)
	}

	return nil
}

func buildInitialStatus(machine *mcmv1alpha1.Machine, ng *deckhousev1.NodeGroup) deckhousev1alpha1.InstanceStatus {
	status := deckhousev1alpha1.InstanceStatus{
		MachineRef: deckhousev1alpha1.InstanceMachineRef{
			APIVersion: machine.APIVersion,
			Kind:       machine.Kind,
			Name:       machine.Name,
			Namespace:  machineNamespace,
		},
		CurrentStatus: deckhousev1alpha1.InstanceCurrentStatus{
			Phase:          deckhousev1alpha1.InstancePhase(machine.Status.CurrentStatus.Phase),
			LastUpdateTime: machine.Status.CurrentStatus.LastUpdateTime,
		},
		NodeRef: deckhousev1alpha1.InstanceNodeRef{
			Name: machine.GetLabels()["node"],
		},
	}

	if ng.Spec.CloudInstances != nil {
		status.ClassReference = deckhousev1alpha1.InstanceClassReference{
			Kind: ng.Spec.CloudInstances.ClassReference.Kind,
			Name: ng.Spec.CloudInstances.ClassReference.Name,
		}
	}

	status.LastOperation = buildLastOperation(&machine.Status.LastOperation)

	return status
}

func buildLastOperation(machineOp *mcmv1alpha1.MachineLastOperation) deckhousev1alpha1.InstanceLastOperation {
	if machineOp == nil {
		return deckhousev1alpha1.InstanceLastOperation{}
	}

	return deckhousev1alpha1.InstanceLastOperation{
		Description:    machineOp.Description,
		LastUpdateTime: machineOp.LastUpdateTime,
		State:          deckhousev1alpha1.InstanceState(machineOp.State),
		Type:           deckhousev1alpha1.InstanceOperationType(machineOp.Type),
	}
}

func (r *InstanceReconciler) handleInstanceDeletion(ctx context.Context, instance *deckhousev1alpha1.Instance, machine *mcmv1alpha1.Machine) error {
	if instance.DeletionTimestamp == nil || instance.DeletionTimestamp.IsZero() {
		return nil
	}

	nodeName := machine.GetLabels()["node"]
	if nodeName == "" {
		nodeName = instance.Status.NodeRef.Name
	}

	if nodeName != "" {
		r.Logger.Info("Setting draining annotation on node due to Instance deletion",
			"instance", instance.Name, "node", nodeName)
		if err := r.setDrainingAnnotation(ctx, nodeName); err != nil {
			return fmt.Errorf("set draining annotation on node %s: %w", nodeName, err)
		}
	} else {
		r.Logger.Info("Cannot set draining annotation: node name is empty",
			"instance", instance.Name, "machine", machine.Name)
	}

	if machine.DeletionTimestamp == nil || machine.DeletionTimestamp.IsZero() {
		if err := r.Client.Delete(ctx, machine, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("delete machine %s/%s: %w", machine.Namespace, machine.Name, err)
		}
	}

	return nil
}

func (r *InstanceReconciler) handleOrphanedInstance(ctx context.Context, instance *deckhousev1alpha1.Instance) error {
	if instance.DeletionTimestamp != nil && !instance.DeletionTimestamp.IsZero() {
		nodeName := instance.Status.NodeRef.Name
		if nodeName != "" {
			r.Logger.Info("Setting draining annotation on node due to Instance deletion (no machine found)",
				"instance", instance.Name, "node", nodeName)
			if err := r.setDrainingAnnotation(ctx, nodeName); err != nil {
				return fmt.Errorf("set draining annotation on node %s: %w", nodeName, err)
			}
		} else {
			r.Logger.Info("Cannot set draining annotation: node name is empty in Instance status",
				"instance", instance.Name)
		}
	}

	if err := r.removeFinalizer(ctx, instance); err != nil {
		return fmt.Errorf("remove finalizer from orphaned instance: %w", err)
	}

	if instance.DeletionTimestamp == nil || instance.DeletionTimestamp.IsZero() {
		if err := r.Client.Delete(ctx, instance); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("delete orphaned instance %s: %w", instance.Name, err)
		}
	}

	return nil
}

func (r *InstanceReconciler) setDrainingAnnotation(ctx context.Context, nodeName string) error {
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get node %s: %w", nodeName, err)
	}

	if node.Annotations != nil && node.Annotations["update.node.deckhouse.io/draining"] == "instance-deletion" {
		return nil
	}

	patch := client.MergeFrom(node.DeepCopy())
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations["update.node.deckhouse.io/draining"] = "instance-deletion"

	if err := r.Client.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("patch node %s draining annotation: %w", nodeName, err)
	}

	return nil
}
