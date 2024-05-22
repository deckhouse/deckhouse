/*
Copyright 2024 Flant JSC

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

package controller

import (
	"context"
	"fmt"
	"net"
	"static-routing-manager-agent/api/v1alpha1"
	"static-routing-manager-agent/pkg/config"
	"static-routing-manager-agent/pkg/logger"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/vishvananda/netlink"

	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	CtrlName      = "static-routing-manager-agent"
	d8Realm       = 216 // d8 hex = 216 dec
	nodeNameLabel = "routing-manager.network.deckhouse.io/node-name"
	finalizer     = "routing-tables-manager.network.deckhouse.io"
)

type RouteEntry struct {
	destination string
	gateway     string
	table       int
}

type RouteEntryMap map[string]RouteEntry

type NRTReconciliationStatus struct {
	IsSuccess    bool
	ErrorMessage string
}

type nrtDeepSpec struct {
	k8sResources            *v1alpha1.NodeRoutingTable
	newReconciliationStatus NRTReconciliationStatus
	desiredRoutesByNRT      RouteEntryMap
	lastAppliedRoutesByNRT  RouteEntryMap
	desiredRoutesToAddByNRT []RouteEntry
	desiredRoutesToDelByNRT RouteEntryMap
	nrtWasDeleted           bool
	specNeedToUpdate        bool
}

type nrtMap map[string]*nrtDeepSpec

// Main

func RunRoutesReconcilerAgentController(
	mgr manager.Manager,
	cfg config.Options,
	log logger.Logger,
) (controller.Controller, error) {
	cl := mgr.GetClient()

	c, err := controller.New(CtrlName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			log.Debug(fmt.Sprintf("[NRTReconciler] Received a reconcile.Request for CR %v", request.Name))

			nrt := &v1alpha1.NodeRoutingTable{}
			err := cl.Get(ctx, request.NamespacedName, nrt)
			if err != nil && !errors2.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("[NRTReconciler] Unable to get NodeRoutingTable, name: %s", request.Name))
				return reconcile.Result{}, err
			}
			if nrt.Name == "" {
				log.Info(fmt.Sprintf("[NRTReconciler] Seems like the NodeRoutingTable for the request %s was deleted. Reconcile retrying will stop.", request.Name))
				return reconcile.Result{}, nil
			}
			labelSelectorSet := map[string]string{nodeNameLabel: cfg.NodeName}
			validatedSelector, _ := labels.ValidatedSelectorFromSet(labelSelectorSet)
			if !validatedSelector.Matches(labels.Set(nrt.Labels)) {
				log.Debug(fmt.Sprintf("[NRTReconciler] This request is not intended(by label) for our node (%v)", cfg.NodeName))
				return reconcile.Result{}, nil
			}
			if nrt.Spec.NodeName != cfg.NodeName {
				log.Debug(fmt.Sprintf("[NRTReconciler] This request is not intended(by spec.nodeName) for our node (%v)", cfg.NodeName))
				return reconcile.Result{}, nil
			}

			if nrt.Generation == nrt.Status.ObservedGeneration && nrt.DeletionTimestamp == nil {
				cond := FindStatusCondition(nrt.Status.Conditions, v1alpha1.ReconciliationSucceedType)
				if cond != nil && cond.Status == metav1.ConditionTrue {
					log.Debug(fmt.Sprintf("[NRTReconciler] There's nothing to do"))
					return reconcile.Result{}, nil
				}
			}
			log.Debug(fmt.Sprintf("[NRTReconciler] NodeRoutingTable %v needs to be reconciled. Set status to Pending", nrt.Name))
			tmpNRT := new(v1alpha1.NodeRoutingTable)
			*tmpNRT = *nrt

			if nrt.Generation != nrt.Status.ObservedGeneration {
				err = SetStatusConditionPending(ctx, cl, log, tmpNRT)
				if err != nil {
					log.Error(err, fmt.Sprintf("[NRTReconciler] Unable to set status to Pending for NRT %v", nrt.Name))
				}
			}

			// ============================= main logic start =============================
			log.Debug(fmt.Sprintf("[NRTReconciler] Starts of the reconciliation (initiated by the k8s-event)"))
			shouldRequeue, err := runEventReconcile(ctx, cl, log, cfg.NodeName)
			if err != nil {
				log.Error(err, fmt.Sprintf("[NRTReconciler] An error occurred while route reconcile"))
			}

			if shouldRequeue {
				log.Warning(fmt.Sprintf("[NRTReconciler] Reconciler will requeue the request, name: %s", request.Name))
				return reconcile.Result{
					RequeueAfter: cfg.RequeueInterval * time.Second,
				}, nil
			}
			// ============================= main logic end =============================

			log.Debug(fmt.Sprintf("[NRTReconciler] Ends of the reconciliation (initiated by the k8s-event)"))
			return reconcile.Result{}, nil
		}),
	})
	if err != nil {
		log.Error(err, "[RunRoutesReconcilerAgentController] unable to create controller")
		return nil, err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &v1alpha1.NodeRoutingTable{}), &handler.EnqueueRequestForObject{})
	if err != nil {
		log.Error(err, "[RunRoutesReconcilerAgentController] unable to watch the events")
		return nil, err
	}

	// trigger reconcile every 30 sec
	ctx := context.Background()
	go periodicalRunEventReconcile(ctx, cl, log, cfg.NodeName)

	return c, nil
}

func runEventReconcile(
	ctx context.Context,
	cl client.Client,
	log logger.Logger,
	nodeName string) (bool, error) {
	// Declaring variables
	var err error
	globalDesiredRoutesForNode := make(RouteEntryMap)
	actualRoutesOnNode := make(RouteEntryMap)
	nrtMap := nrtMapInit()

	// Getting all the NodeRoutingTable associated with our node
	nrtList := &v1alpha1.NodeRoutingTableList{}
	err = cl.List(ctx, nrtList, client.MatchingLabels{nodeNameLabel: nodeName})
	if err != nil && !errors2.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("[NRTReconciler] unable to list NodeRoutingTable for node %s", nodeName))
		return true, err
	}

	// Getting all routes from our node
	log.Debug(fmt.Sprintf("[NRTReconciler] Getting all routes from our node"))
	actualRoutesOnNode, err = getActualRouteEntryMapFromNode()
	if err != nil {
		log.Error(err, fmt.Sprintf("[NRTReconciler] unable to get Actual routes from node"))
		return true, err
	}
	if len(actualRoutesOnNode) == 0 {
		log.Debug(fmt.Sprintf("[NRTReconciler] There are no routes with Realm=" + strconv.Itoa(d8Realm)))
	}

	for _, nrt := range nrtList.Items {
		nrtDeepSpec := nrtDeepSpecInit()
		// Gathering facts
		log.Debug(fmt.Sprintf("[NRTReconciler] Starting gather facts about nrt %v", nrt.Name))
		if nrtDeepSpec.gatheringFacts(nrt, &globalDesiredRoutesForNode, &actualRoutesOnNode, log) {
			(*nrtMap)[nrt.Name] = nrtDeepSpec
			continue
		}

		// Actions: add routes
		if len(nrtDeepSpec.desiredRoutesToAddByNRT) > 0 {
			log.Debug(fmt.Sprintf("[NRTReconciler] Starting adding routes to the node"))
			nrtDeepSpec.addRoutes(&actualRoutesOnNode, log)
		}

		(*nrtMap)[nrt.Name] = nrtDeepSpec
	}

	// Actions: delete routes and finalizers
	nrtMap.deleteRoutesAndFinalizers(globalDesiredRoutesForNode, actualRoutesOnNode, log)

	// Generate new condition for each processed nrt
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting generate new conditions"))
	shouldRequeue := nrtMap.generateNewCondition()

	// Update state in k8s for each processed nrt
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting updating resourses in k8s"))
	nrtMap.updateStateInK8S(ctx, cl, log)

	return shouldRequeue, nil
}

func periodicalRunEventReconcile(
	ctx context.Context,
	cl client.Client,
	log logger.Logger,
	nodeName string,
) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug(fmt.Sprintf("[NRTReconciler] Starts a periodic reconciliation (initiated by a timer)"))
			_, err := runEventReconcile(ctx, cl, log, nodeName)
			if err != nil {
				log.Error(err, fmt.Sprintf("[NRTReconciler] an error occurred while route reconcile"))
			}
			log.Debug(fmt.Sprintf("[NRTReconciler] Ends a periodic reconciliation (initiated by a timer)"))
		case <-ctx.Done():
			log.Debug(fmt.Sprintf("[NRTReconciler] Completion of periodic reconciliations"))
			return
		}
	}
}

// service functions

func getActualRouteEntryMapFromNode() (RouteEntryMap, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Realm: d8Realm}, netlink.RT_FILTER_REALM|netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, fmt.Errorf("failed get routes from node, err: %w", err)
	}
	ar := make(RouteEntryMap)

	for _, route := range routes {
		re := RouteEntry{
			destination: route.Dst.String(),
			gateway:     route.Gw.String(),
			table:       route.Table,
		}
		ar.AppendRE(re)
	}

	return ar, nil
}

func addRouteToNode(route RouteEntry) error {
	_, dstnetIPNet, err := net.ParseCIDR(route.destination)
	if err != nil {
		return fmt.Errorf("unable to parse destination in route %v gw %v tbl %v, err: %w",
			route.destination,
			route.gateway,
			route.table,
			err,
		)
	}
	gwNetIP := net.ParseIP(route.gateway)
	err = netlink.RouteAdd(&netlink.Route{
		Realm: d8Realm,
		Table: route.table,
		Dst:   dstnetIPNet,
		Gw:    gwNetIP,
	})
	if err != nil {
		return fmt.Errorf("unable to add route %v gw %v tbl %v, err: %w",
			route.destination,
			route.gateway,
			route.table,
			err,
		)
	}
	return nil
}

func delRouteFromNode(route RouteEntry) error {
	_, dstnetIPNet, err := net.ParseCIDR(route.destination)
	if err != nil {
		return fmt.Errorf("unable to parse destination in route %v gw %v tbl %v, err: %w",
			route.destination,
			route.gateway,
			route.table,
			err,
		)
	}
	gwNetIP := net.ParseIP(route.gateway)
	err = netlink.RouteDel(&netlink.Route{
		Realm: d8Realm,
		Table: route.table,
		Dst:   dstnetIPNet,
		Gw:    gwNetIP,
	})
	if err != nil {
		return fmt.Errorf("unable to del route %v gw %v tbl %v, err: %w",
			route.destination,
			route.gateway,
			route.table,
			err,
		)
	}
	return nil
}

func deleteRouteEntriesFromNode(delREM, gdREM, actREM RouteEntryMap, status NRTReconciliationStatus, log logger.Logger) NRTReconciliationStatus {
	for hash, route := range delREM {
		log.Debug(fmt.Sprintf("Route %v should be deleted", route))
		if _, ok := (gdREM)[hash]; ok {
			log.Debug(fmt.Sprintf("but it is present in other NRT"))
			continue
		}
		if _, ok := (actREM)[hash]; !ok {
			log.Debug(fmt.Sprintf("but it is not present on Node"))
			continue
		}
		err := delRouteFromNode(route)
		if err != nil {
			log.Debug(fmt.Sprintf("err: %v", err))
			status.AppendError(err)
		}
	}
	return status
}

func removeFinalizer(nrt *v1alpha1.NodeRoutingTable) {
	// tmpNrt.Finalizers = []string{}
	var tmpNRTFinalizers []string
	tmpNRTFinalizers = []string{}
	for _, fnlzr := range nrt.Finalizers {
		if fnlzr != finalizer {
			tmpNRTFinalizers = append(tmpNRTFinalizers, fnlzr)
		}
	}
	nrt.Finalizers = tmpNRTFinalizers
}

func SetStatusCondition(conditions *[]v1alpha1.NodeRoutingTableCondition, newCondition v1alpha1.NodeRoutingTableCondition) (changed bool) {
	if conditions == nil {
		return false
	}

	timeNow := metav1.NewTime(time.Now())

	existingCondition := FindStatusCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		if newCondition.LastTransitionTime.IsZero() {
			newCondition.LastTransitionTime = timeNow
		}
		if newCondition.LastHeartbeatTime.IsZero() {
			newCondition.LastHeartbeatTime = timeNow
		}
		*conditions = append(*conditions, newCondition)
		return true
	}

	if !newCondition.LastHeartbeatTime.IsZero() {
		existingCondition.LastHeartbeatTime = newCondition.LastHeartbeatTime
	} else {
		existingCondition.LastHeartbeatTime = timeNow
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		if !newCondition.LastTransitionTime.IsZero() {
			existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		} else {
			existingCondition.LastTransitionTime = timeNow
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
	return changed
}

func FindStatusCondition(conditions []v1alpha1.NodeRoutingTableCondition, conditionType string) *v1alpha1.NodeRoutingTableCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func SetStatusConditionPending(ctx context.Context, cl client.Client, log logger.Logger, nrt *v1alpha1.NodeRoutingTable) error {
	t := metav1.NewTime(time.Now())
	nrt.Status.ObservedGeneration = nrt.Generation

	newCond := v1alpha1.NodeRoutingTableCondition{}
	newCond.Type = v1alpha1.ReconciliationSucceedType
	newCond.LastHeartbeatTime = t
	newCond.Status = metav1.ConditionFalse
	newCond.Reason = v1alpha1.ReconciliationReasonPending
	newCond.Message = ""

	_ = SetStatusCondition(&nrt.Status.Conditions, newCond)

	err := cl.Status().Update(ctx, nrt)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to update status for CR NodeRoutingTable %v, err: %v", nrt.Name, err))
		return err
	}
	return nil
}

func nrtDeepSpecInit() *nrtDeepSpec {
	return &nrtDeepSpec{
		k8sResources:            new(v1alpha1.NodeRoutingTable),
		newReconciliationStatus: NRTReconciliationStatus{},
		desiredRoutesByNRT:      RouteEntryMap{},
		lastAppliedRoutesByNRT:  RouteEntryMap{},
		desiredRoutesToAddByNRT: make([]RouteEntry, 0),
		desiredRoutesToDelByNRT: RouteEntryMap{},
		nrtWasDeleted:           false,
		specNeedToUpdate:        false,
	}
}

func nrtMapInit() *nrtMap {
	newNRTMap := new(nrtMap)
	*newNRTMap = make(map[string]*nrtDeepSpec)
	return newNRTMap
}

func (re *RouteEntry) getHash() string {
	return fmt.Sprintf("%d#%s#%s", re.table, re.destination, re.gateway)
}

func (rem *RouteEntryMap) AppendRE(re RouteEntry) {
	if len(*rem) == 0 {
		*rem = make(map[string]RouteEntry)
	}
	(*rem)[re.getHash()] = re
}

func (rem *RouteEntryMap) AppendR(route v1alpha1.Route, tbl int) {
	if len(*rem) == 0 {
		*rem = make(map[string]RouteEntry)
	}
	re := RouteEntry{
		destination: route.Destination,
		gateway:     route.Gateway,
		table:       tbl,
	}
	(*rem)[re.getHash()] = re
}

func (s *NRTReconciliationStatus) AppendError(err error) {
	s.IsSuccess = false
	if s.ErrorMessage == "" {
		s.ErrorMessage = err.Error()
	} else {
		s.ErrorMessage = s.ErrorMessage + "\n" + err.Error()
	}
}

func (nds *nrtDeepSpec) gatheringFacts(nrt v1alpha1.NodeRoutingTable, globalDesiredRoutesForNode, actualRoutesOnNode *RouteEntryMap, log logger.Logger) bool {
	// Filling nrtK8sResourcesMap[nrt.Name] and nrtReconciliationStatusMap[nrt.Name]
	tmpNrt := nrt
	tmpNrt.Status.ObservedGeneration = nrt.Generation
	nds.k8sResources = &tmpNrt
	nds.newReconciliationStatus = NRTReconciliationStatus{IsSuccess: true}
	nds.specNeedToUpdate = false

	// If NRT was deleted filling map desiredRoutesToDelByNRT and set flag nrtWasDeleted
	if nrt.DeletionTimestamp != nil {
		log.Debug(fmt.Sprintf("[NRTReconciler] NRT %v is marked for deletion", nrt.Name))
		log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling map routesToDel, and set flag nrtWasDeleted "))
		tmpREM := make(RouteEntryMap)
		for _, route := range nrt.Spec.Routes {
			tmpREM.AppendR(route, nrt.Spec.IPRoutingTableID)
		}
		nds.desiredRoutesToDelByNRT = tmpREM
		nds.nrtWasDeleted = true
		return true
	}

	// Filling desiredRoutesByNRT and globalDesiredRoutesForNode
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling maps: desiredRoutes and globalDesiredRoutes"))
	for _, route := range nrt.Spec.Routes {
		nds.desiredRoutesByNRT.AppendR(route, nrt.Spec.IPRoutingTableID)
		globalDesiredRoutesForNode.AppendR(route, nrt.Spec.IPRoutingTableID)
	}

	// Filling lastAppliedRoutesByNRT
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling map lastAppliedRoutes"))
	if nrt.Status.AppliedRoutes != nil && len(nrt.Status.AppliedRoutes) > 0 {
		for _, route := range nrt.Status.AppliedRoutes {
			nds.lastAppliedRoutesByNRT.AppendR(route, nrt.Spec.IPRoutingTableID)
		}
	}

	// Filling desiredRoutesToAddByNRT
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling map routesToAdd"))
	for hash, desiredRoute := range nds.desiredRoutesByNRT {
		if _, ok := (*actualRoutesOnNode)[hash]; !ok {
			nds.desiredRoutesToAddByNRT = append(nds.desiredRoutesToAddByNRT, desiredRoute)
		}
	}

	// Filling desiredRoutesToDelByNRT
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling map routesToDel"))
	tmpREM := make(RouteEntryMap)
	for hash, route := range nds.lastAppliedRoutesByNRT {
		if _, ok := nds.desiredRoutesByNRT[hash]; !ok {
			tmpREM.AppendRE(route)
		}
	}
	if len(tmpREM) > 0 {
		nds.desiredRoutesToDelByNRT = tmpREM
	}

	return false
}

func (nds *nrtDeepSpec) addRoutes(actualRoutesOnNode *RouteEntryMap, log logger.Logger) {
	status := nds.newReconciliationStatus
	for _, route := range nds.desiredRoutesToAddByNRT {
		log.Debug(fmt.Sprintf("Route %v should be added", route))
		if _, ok := (*actualRoutesOnNode)[route.getHash()]; ok {
			log.Debug(fmt.Sprintf("but it is already present on Node"))
			continue
		}
		err := addRouteToNode(route)
		if err == nil {
			actualRoutesOnNode.AppendRE(route)
		} else {
			log.Debug(fmt.Sprintf("err: %v", err))
			status.AppendError(err)
		}
	}
	nds.newReconciliationStatus = status
}

func (nm *nrtMap) deleteRoutesAndFinalizers(globalDesiredRoutesForNode, actualRoutesOnNode RouteEntryMap, log logger.Logger) {
	for nrtName, nds := range *nm {
		if len(nds.desiredRoutesToDelByNRT) == 0 && !nds.nrtWasDeleted {
			log.Debug(fmt.Sprintf("[NRTReconciler] NRT %v has no entries in desiredRoutesToDelByNRT and DeletionTimestamp is not set", nrtName))
			continue
		}
		log.Debug(fmt.Sprintf("[NRTReconciler] Starting to delete routes deleted from NRT %v from node", nrtName))
		status := nds.newReconciliationStatus
		nds.newReconciliationStatus = deleteRouteEntriesFromNode(
			nds.desiredRoutesToDelByNRT,
			globalDesiredRoutesForNode,
			actualRoutesOnNode,
			status,
			log,
		)
		if nds.nrtWasDeleted && nds.newReconciliationStatus.IsSuccess {
			log.Debug(fmt.Sprintf("[NRTReconciler] NRT %v has been deleted and its routes has been successfully deleted too. Clearing the finalizer in NRT", nrtName))
			removeFinalizer(nds.k8sResources)
			nds.specNeedToUpdate = true
		}
	}
}

func (nm *nrtMap) generateNewCondition() bool {
	shouldRequeue := false
	for _, nds := range *nm {
		newCond := v1alpha1.NodeRoutingTableCondition{}
		t := metav1.NewTime(time.Now())

		if nds.k8sResources.Status.Conditions == nil {
			nds.k8sResources.Status.Conditions = make([]v1alpha1.NodeRoutingTableCondition, 0)
		}

		if nds.newReconciliationStatus.IsSuccess {
			nds.k8sResources.Status.AppliedRoutes = nds.k8sResources.Spec.Routes

			newCond.Type = v1alpha1.ReconciliationSucceedType
			newCond.LastHeartbeatTime = t
			newCond.Status = metav1.ConditionTrue
			newCond.Reason = v1alpha1.ReconciliationReasonSucceed
			newCond.Message = ""
		} else {
			newCond.Type = v1alpha1.ReconciliationSucceedType
			newCond.LastHeartbeatTime = t
			newCond.Status = metav1.ConditionFalse
			newCond.Reason = v1alpha1.ReconciliationReasonFailed
			newCond.Message = nds.newReconciliationStatus.ErrorMessage

			shouldRequeue = true
		}
		_ = SetStatusCondition(&nds.k8sResources.Status.Conditions, newCond)
	}
	return shouldRequeue
}

func (nm *nrtMap) updateStateInK8S(ctx context.Context, cl client.Client, log logger.Logger) {
	var err error
	for nrtName, nds := range *nm {
		if nds.specNeedToUpdate && nds.k8sResources.DeletionTimestamp != nil {
			// Update spec if we need to remove the finalizer
			log.Debug(fmt.Sprintf("Update of NRT: %v", nrtName))
			err = cl.Update(ctx, nds.k8sResources)
			if err != nil {
				log.Error(err, fmt.Sprintf("unable to update CR NodeRoutingTable %v, err: %v", nrtName, err))
			}
		}
		// Update status every time
		log.Debug(fmt.Sprintf("Update status of NRT: %v", nrtName))
		err = cl.Status().Update(ctx, nds.k8sResources)
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to update status for CR NodeRoutingTable %v, err: %v", nrtName, err))
		}
	}
}
