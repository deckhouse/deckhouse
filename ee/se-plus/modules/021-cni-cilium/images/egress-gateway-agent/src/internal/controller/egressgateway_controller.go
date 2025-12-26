/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/egress-gateway-agent/internal/layer2"
	eeCommon "github.com/deckhouse/deckhouse/egress-gateway-agent/pkg/apis/common"
	eeInternalCrd "github.com/deckhouse/deckhouse/egress-gateway-agent/pkg/apis/internal.network/v1alpha1"
)

const (
	activeNodeLabelKey = "egress-gateway.network.deckhouse.io/node-name"
	finalizerKey       = "egress-gateway.network.deckhouse.io"
)

type EgressGatewayInstanceReconciler struct {
	NodeName string
	client.Client
	VirtualIPAnnounces *layer2.Announce
	Scheme             *runtime.Scheme
}

func (r *EgressGatewayInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get resource
	var egressGatewayInstance eeInternalCrd.SDNInternalEgressGatewayInstance
	if err := r.Get(ctx, req.NamespacedName, &egressGatewayInstance); err != nil {
		logger.Error(err, "unable to fetch egress gateway instance", "name", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Resource is deleted need to clean up finalizer
	if !egressGatewayInstance.DeletionTimestamp.IsZero() {
		logger.Info("Deletion detected! Proceeding to cleanup the finalizers", "name", egressGatewayInstance.Name)
		err := r.cleanupAnnouncerWithFinalizer(ctx, logger, &egressGatewayInstance)
		if err != nil {
			logger.Error(err, "unable to cleanup egress gateway instance", "name", egressGatewayInstance.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Handle Virtual IP Address
	if egressGatewayInstance.Spec.SourceIP.Mode == eeCommon.VirtualIPAddress {
		var ipAdvertisement layer2.IPAdvertisement
		ip := egressGatewayInstance.Spec.SourceIP.VirtualIPAddress.IP
		interfaces := egressGatewayInstance.Spec.SourceIP.VirtualIPAddress.Interfaces

		if len(interfaces) == 0 {
			ipAdvertisement = layer2.NewIPAdvertisement(net.ParseIP(ip), true, sets.Set[string]{})
		} else {
			ipAdvertisement = layer2.NewIPAdvertisement(net.ParseIP(ip), false, sets.New(interfaces...))
		}

		r.VirtualIPAnnounces.SetBalancer(egressGatewayInstance.Name, ipAdvertisement)
		logger.Info("ensured virtual IP announcement", "ip", ip)
	}

	// Update Status
	condition := eeCommon.ExtendedCondition{
		Condition: metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "AnnouncingSucceed",
			Message: fmt.Sprintf("Virtual IP %s is announced on %v", egressGatewayInstance.Spec.SourceIP.VirtualIPAddress.IP, egressGatewayInstance.Spec.SourceIP.VirtualIPAddress.Interfaces),
		},
	}
	if !r.VirtualIPAnnounces.AnnounceName(egressGatewayInstance.Name) &&
		egressGatewayInstance.Spec.SourceIP.Mode == eeCommon.VirtualIPAddress {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "AnnouncingFailed"
		condition.Message = fmt.Sprintf("Virtual IP %s is NOT announced", egressGatewayInstance.Spec.SourceIP.VirtualIPAddress.IP)
	}

	egressGatewayInstance.Status.ObservedGeneration = egressGatewayInstance.Generation
	setStatusCondition(&egressGatewayInstance.Status.Conditions, condition)

	if err := r.Client.Status().Update(ctx, &egressGatewayInstance); err != nil {
		logger.Error(err, "failed to update egress gateway instance status", "name", egressGatewayInstance.Name)
	}

	return ctrl.Result{}, nil
}

func (r *EgressGatewayInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&eeInternalCrd.SDNInternalEgressGatewayInstance{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			nodeName, ok := object.GetLabels()[activeNodeLabelKey]
			return ok && nodeName == r.NodeName
		})).
		Complete(r)
}

func (r *EgressGatewayInstanceReconciler) cleanupAnnouncerWithFinalizer(ctx context.Context, logger logr.Logger, egressGatewayInstance *eeInternalCrd.SDNInternalEgressGatewayInstance) error {
	if controllerutil.ContainsFinalizer(egressGatewayInstance, finalizerKey) {
		if egressGatewayInstance.Spec.SourceIP.Mode == eeCommon.VirtualIPAddress {
			r.VirtualIPAnnounces.DeleteBalancer(egressGatewayInstance.Spec.SourceIP.VirtualIPAddress.IP)
		}

		controllerutil.RemoveFinalizer(egressGatewayInstance, finalizerKey)
		if err := r.Update(ctx, egressGatewayInstance); err != nil {
			return fmt.Errorf("failed to remove finalizer from egress gateway instance: %w", err)
		}
		logger.Info("cleaned up finalizer successfully", "finalizer", finalizerKey)
	}
	return nil
}

func setStatusCondition(conditions *[]eeCommon.ExtendedCondition, newCondition eeCommon.ExtendedCondition) (changed bool) {
	if conditions == nil {
		return false
	}
	existingCondition := findStatusCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		if newCondition.LastTransitionTime.IsZero() {
			newCondition.LastTransitionTime = metav1.NewTime(time.Now())
		}
		newCondition.LastHeartbeatTime = metav1.NewTime(time.Now())
		*conditions = append(*conditions, newCondition)
		return true
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		if !newCondition.LastTransitionTime.IsZero() {
			existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		} else {
			existingCondition.LastTransitionTime = metav1.NewTime(time.Now())
		}
		changed = true
	}

	if existingCondition.Reason != newCondition.Reason {
		existingCondition.Reason = newCondition.Reason
		changed = true
	}
	if existingCondition.Message != newCondition.Message {
		existingCondition.Message = newCondition.Message
		changed = true
	}
	if existingCondition.ObservedGeneration != newCondition.ObservedGeneration {
		existingCondition.ObservedGeneration = newCondition.ObservedGeneration
		changed = true
	}

	existingCondition.LastHeartbeatTime = metav1.NewTime(time.Now())

	return changed
}

// FindStatusCondition finds the conditionType in conditions.
func findStatusCondition(conditions []eeCommon.ExtendedCondition, conditionType string) *eeCommon.ExtendedCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}
