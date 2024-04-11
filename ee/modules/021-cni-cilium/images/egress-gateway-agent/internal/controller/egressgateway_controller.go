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

	"egress-gateway-agent/internal/layer2"

	eeCrd "github.com/deckhouse/deckhouse/modules/021-cni-cilium/go_lib/apis/v1alpha1"
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
	var egressGatewayInstance eeCrd.EgressGatewayInstance
	if err := r.Get(ctx, req.NamespacedName, &egressGatewayInstance); err != nil {
		logger.Error(err, "unable to fetch egress gateway instance", "name", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Resource is deleted need to clean up finalizer
	if !egressGatewayInstance.DeletionTimestamp.IsZero() {
		logger.Info("Deletion detected! Proceeding to cleanup the finalizers", "name", egressGatewayInstance.Name)
		err := r.cleanupAnouncerWithFinalizer(ctx, logger, &egressGatewayInstance)
		if err != nil {
			logger.Error(err, "unable to cleanup egress gateway instance", "name", egressGatewayInstance.Name)
			return ctrl.Result{}, err
		}
	}

	desiredVirtualIPsToAnnounce := make(map[string]struct{})

	// Get list of EG by label
	var egressGatewayInstanceList eeCrd.EgressGatewayInstanceList
	if err := r.Client.List(ctx, &egressGatewayInstanceList, client.MatchingLabels{activeNodeLabelKey: r.NodeName}); err != nil {
		logger.Error(err, "failed to list egress gateways")
		return ctrl.Result{}, err
	}

	for _, egressGatewayInstance := range egressGatewayInstanceList.Items {
		if egressGatewayInstance.Spec.SourceIP.Mode != eeCrd.VirtualIPAddress {
			continue
		}
		desiredVirtualIPsToAnnounce[egressGatewayInstance.Spec.SourceIP.VirtualIPAddress.IP] = struct{}{}
	}

	virtualIPsToAdd := make([]string, 0, 4)
	virtualIPsToDel := make([]string, 0, 4)

	actualVirtualIPsToAnnounce := r.VirtualIPAnnounces.GetIPSMap()

	for ip := range desiredVirtualIPsToAnnounce {
		if _, ok := actualVirtualIPsToAnnounce[ip]; ok {
			continue
		}
		virtualIPsToAdd = append(virtualIPsToAdd, ip)
	}
	logger.Info("IPs chosen for announce", "count", len(virtualIPsToAdd))

	for ip := range actualVirtualIPsToAnnounce {
		if _, ok := desiredVirtualIPsToAnnounce[ip]; ok {
			continue
		}
		virtualIPsToDel = append(virtualIPsToDel, ip)
	}
	logger.Info("IPs chosen for deletion", "count", len(virtualIPsToDel))

	for _, ip := range virtualIPsToAdd {
		ipAdvertisement := layer2.NewIPAdvertisement(net.ParseIP(ip), true, sets.Set[string]{})
		r.VirtualIPAnnounces.SetBalancer(ip, ipAdvertisement)
		logger.Info("added virtual IP", "ip", ip)
	}

	for _, ip := range virtualIPsToDel {
		r.VirtualIPAnnounces.DeleteBalancer(ip)
		logger.Info("deleted virtual IP", "ip", ip)
	}

	condition := eeCrd.ExtendedCondition{
		Condition: metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "AnnouncingSucceed",
			Message: fmt.Sprintf("Announcing %d Virtual IPs", len(r.VirtualIPAnnounces.GetIPSMap())),
		},
	}
	if len(r.VirtualIPAnnounces.GetIPSMap()) == 0 {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "AnnouncingFailed"
		condition.Status = "Announcing Virtual IPs failed"
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
		For(&eeCrd.EgressGatewayInstance{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			nodeName, ok := object.GetLabels()[activeNodeLabelKey]
			return ok && nodeName == r.NodeName
		})).
		Complete(r)
}

func (r *EgressGatewayInstanceReconciler) cleanupAnouncerWithFinalizer(ctx context.Context, logger logr.Logger, egressGatewayInstance *eeCrd.EgressGatewayInstance) error {
	if controllerutil.ContainsFinalizer(egressGatewayInstance, finalizerKey) {
		if egressGatewayInstance.Spec.SourceIP.Mode == eeCrd.VirtualIPAddress {
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

func setStatusCondition(conditions *[]eeCrd.ExtendedCondition, newCondition eeCrd.ExtendedCondition) (changed bool) {
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
func findStatusCondition(conditions []eeCrd.ExtendedCondition, conditionType string) *eeCrd.ExtendedCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}
