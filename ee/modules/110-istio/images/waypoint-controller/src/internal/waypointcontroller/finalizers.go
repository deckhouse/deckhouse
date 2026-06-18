/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

const resourceDeleteCheckInterval = 30 * time.Second

func (r *WaypointController) handleFinalizers(ctx context.Context, instance *networkv1alpha1.WaypointInstance) (ctrl.Result, error) {
	if instance.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(instance, WaypointFinalizer) {
			return ctrl.Result{}, nil
		}

		base := instance.DeepCopy()
		controllerutil.AddFinalizer(instance, WaypointFinalizer)
		return ctrl.Result{}, r.Patch(ctx, instance, client.MergeFrom(base))
	}

	if !controllerutil.ContainsFinalizer(instance, WaypointFinalizer) {
		return ctrl.Result{}, nil
	}

	owned, err := r.hasOwnedResources(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	if owned {
		return ctrl.Result{RequeueAfter: resourceDeleteCheckInterval}, nil
	}

	// TODO: do we need to cleanup external stuff? (child objects which ownerRef doesn't touch)
	base := instance.DeepCopy()
	controllerutil.RemoveFinalizer(instance, WaypointFinalizer)
	return ctrl.Result{}, r.Patch(ctx, instance, client.MergeFrom(base))
}

func (r *WaypointController) hasOwnedResources(ctx context.Context, instance *networkv1alpha1.WaypointInstance) (bool, error) {
	opts := listOpts(instance)
	anyOwned := false

	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, opts...); err != nil {
		return false, err
	}
	if len(deployments.Items) > 0 {
		for i := range deployments.Items {
			if err := r.pruneOwnerReference(ctx, instance, &deployments.Items[i]); err != nil {
				return false, err
			}
		}
		anyOwned = true
	}

	services := &corev1.ServiceList{}
	if err := r.List(ctx, services, opts...); err != nil {
		return false, err
	}
	if len(services.Items) > 0 {
		for i := range services.Items {
			if err := r.pruneOwnerReference(ctx, instance, &services.Items[i]); err != nil {
				return false, err
			}
		}
		anyOwned = true
	}

	serviceAccounts := &corev1.ServiceAccountList{}
	if err := r.List(ctx, serviceAccounts, opts...); err != nil {
		return false, err
	}
	if len(serviceAccounts.Items) > 0 {
		for i := range serviceAccounts.Items {
			if err := r.pruneOwnerReference(ctx, instance, &serviceAccounts.Items[i]); err != nil {
				return false, err
			}
		}
		anyOwned = true
	}

	pdbs := &policyv1.PodDisruptionBudgetList{}
	if err := r.List(ctx, pdbs, opts...); err != nil {
		return false, err
	}
	if len(pdbs.Items) > 0 {
		for i := range pdbs.Items {
			if err := r.pruneOwnerReference(ctx, instance, &pdbs.Items[i]); err != nil {
				return false, err
			}
		}
		anyOwned = true
	}

	if r.VPAEnabled {
		vpas := &vpav1.VerticalPodAutoscalerList{}
		if err := r.List(ctx, vpas, opts...); err != nil {
			return false, err
		}
		if len(vpas.Items) > 0 {
			for i := range vpas.Items {
				if err := r.pruneOwnerReference(ctx, instance, &vpas.Items[i]); err != nil {
					return false, err
				}
			}
			anyOwned = true
		}
	}

	hpas := &autoscalingv2.HorizontalPodAutoscalerList{}
	if err := r.List(ctx, hpas, opts...); err != nil {
		return false, err
	}
	if len(hpas.Items) > 0 {
		for i := range hpas.Items {
			if err := r.pruneOwnerReference(ctx, instance, &hpas.Items[i]); err != nil {
				return false, err
			}
		}
		anyOwned = true
	}

	gateways := &gatewayv1.GatewayList{}
	if err := r.List(ctx, gateways, opts...); err != nil {
		return false, err
	}
	if len(gateways.Items) > 0 {
		for i := range gateways.Items {
			if err := r.pruneOwnerReference(ctx, instance, &gateways.Items[i]); err != nil {
				return false, err
			}
		}
		anyOwned = true
	}

	return anyOwned, nil
}

func listOpts(instance client.Object) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(instance.GetNamespace()),
		client.MatchingFields{ownerUIDFieldIndex: string(instance.GetUID())},
	}
}

func (r *WaypointController) pruneOwnerReference(ctx context.Context, instance *networkv1alpha1.WaypointInstance, owned client.Object) error {
	ownerRefs := owned.GetOwnerReferences()
	if len(ownerRefs) == 0 {
		return nil
	}

	hasOwnerRef, err := controllerutil.HasOwnerReference(ownerRefs, instance, r.scheme)
	if err != nil {
		// GVK resolution failure should not block deletion; treat as not owned.
		klog.ErrorS(err, "Could not resolve owner reference, treating resource as not owned",
			"owned", client.ObjectKeyFromObject(owned),
		)
		return nil
	}
	if !hasOwnerRef {
		return nil
	}

	if len(ownerRefs) < 2 {
		err := r.Delete(ctx, owned, client.PropagationPolicy(metav1.DeletePropagationBackground))
		return client.IgnoreNotFound(err)
	}

	// If there are more than one owners, detach this instance to avoid blocking deletion.
	base := owned.DeepCopyObject().(client.Object)
	if err := controllerutil.RemoveOwnerReference(instance, owned, r.scheme); err != nil {
		return err
	}

	return r.Patch(ctx, owned, client.MergeFrom(base))
}
