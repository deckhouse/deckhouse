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
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/api/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/pkg/config"
	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/pkg/logger"
	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/pkg/utils"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	routeCtrlName = "routing-tables-controller"
)

// Main

func RunRoutesReconcilerAgentController(
	mgr manager.Manager,
	cfg config.Options,
	log logger.Logger,
) (controller.Controller, error) {
	cl := mgr.GetClient()

	c, err := controller.New(routeCtrlName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			log.Debug(fmt.Sprintf("[NRTReconciler] Received a reconcile.Request for CR %v", request.Name))

			nrt := &v1alpha1.SDNInternalNodeRoutingTable{}
			err := cl.Get(ctx, request.NamespacedName, nrt)
			if err != nil && !k8serrors.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("[NRTReconciler] Unable to get SDNInternalNodeRoutingTable, name: %s", request.Name))
				return reconcile.Result{}, err
			}
			if nrt.Name == "" {
				log.Info(fmt.Sprintf("[NRTReconciler] Seems like the SDNInternalNodeRoutingTable for the request %s was deleted. Reconcile retrying will stop.", request.Name))
				return reconcile.Result{}, nil
			}
			labelSelectorSet := map[string]string{v1alpha1.NodeNameLabel: cfg.NodeName}
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
				cond := utils.FindStatusCondition(nrt.Status.Conditions, v1alpha1.ReconciliationSucceedType)
				if cond != nil && cond.Status == metav1.ConditionTrue {
					log.Debug(fmt.Sprintf("[NRTReconciler] There's nothing to do"))
					return reconcile.Result{}, nil
				}
			}
			log.Debug(fmt.Sprintf("[NRTReconciler] SDNInternalNodeRoutingTable %v needs to be reconciled. Set status to Pending", nrt.Name))
			tmpNRT := new(v1alpha1.SDNInternalNodeRoutingTable)
			*tmpNRT = *nrt

			if nrt.Generation != nrt.Status.ObservedGeneration {
				err = utils.SetStatusConditionPendingToNRT(ctx, cl, log, tmpNRT)
				if err != nil {
					log.Error(err, fmt.Sprintf("[NRTReconciler] Unable to set status to Pending for NRT %v", nrt.Name))
				}
			}

			// ============================= main logic start =============================
			log.Debug(fmt.Sprintf("[NRTReconciler] Starts of the reconciliation (initiated by the k8s-event)"))
			shouldRequeue, err := runEventRouteReconcile(ctx, cl, log, cfg.NodeName)
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

			log.Debug(fmt.Sprintf("[NRTReconciler] End of the reconciliation (initiated by the k8s-event)"))
			return reconcile.Result{}, nil
		}),
	})
	if err != nil {
		log.Error(err, "[RunRoutesReconcilerAgentController] unable to create controller")
		return nil, err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &v1alpha1.SDNInternalNodeRoutingTable{}), &handler.EnqueueRequestForObject{})
	if err != nil {
		log.Error(err, "[RunRoutesReconcilerAgentController] unable to watch the events")
		return nil, err
	}

	// trigger reconcile every 30 sec
	ctx := context.Background()
	go periodicalRunEventReconcile(ctx, cfg, cl, log, cfg.NodeName)

	return c, nil
}

func runEventRouteReconcile(
	ctx context.Context,
	cl client.Client,
	log logger.Logger,
	nodeName string) (bool, error) {
	// Declaring variables
	var err error
	globalDesiredRoutesForNode := make(RouteEntryMap)
	actualRoutesOnNode := make(RouteEntryMap)
	nrtMap := nrtMapInit()

	// Getting all the SDNInternalNodeRoutingTable associated with our node
	nrtList := &v1alpha1.SDNInternalNodeRoutingTableList{}
	err = cl.List(ctx, nrtList, client.MatchingLabels{v1alpha1.NodeNameLabel: nodeName})
	if err != nil && !k8serrors.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("[NRTReconciler] unable to list SDNInternalNodeRoutingTable for node %s", nodeName))
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
		log.Debug(fmt.Sprintf("[NRTReconciler] There are no routes with Realm=" + strconv.Itoa(v1alpha1.D8Realm)))
	}

	for _, nrt := range nrtList.Items {
		nrtSummary := nrtSummaryInit()
		// Gathering facts
		log.Debug(fmt.Sprintf("[NRTReconciler] Starting gather facts about nrt %v", nrt.Name))
		if nrtSummary.discoverFacts(nrt, &globalDesiredRoutesForNode, &actualRoutesOnNode, log) {
			(*nrtMap)[nrt.Name] = nrtSummary
			continue
		}

		// Actions: add routes
		if len(nrtSummary.desiredRoutesToAddByNRT) > 0 {
			log.Debug(fmt.Sprintf("[NRTReconciler] Starting adding routes to the node"))
			nrtSummary.addRoutes(&actualRoutesOnNode, log)
		}

		(*nrtMap)[nrt.Name] = nrtSummary
	}

	// Actions: delete routes and finalizers (based on each NRT)
	nrtMap.deleteRoutesAndFinalizers(globalDesiredRoutesForNode, actualRoutesOnNode, log)

	// Actions: Deleting orphan routes (with realm 216) that are not mentioned in any NRT
	deleteOrphanRoutes(globalDesiredRoutesForNode, actualRoutesOnNode, log)

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
	cfg config.Options,
	cl client.Client,
	log logger.Logger,
	nodeName string,
) {
	ticker := time.NewTicker(cfg.PeriodicReconciliationInterval * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug(fmt.Sprintf("[NRTReconciler] Starts a periodic reconciliation (initiated by a timer)"))
			_, err := runEventRouteReconcile(ctx, cl, log, nodeName)
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

// RouteEntry: type, service functions and methods

type RouteEntry struct {
	destination string
	gateway     string
	table       int
}

func (re *RouteEntry) getHash() string {
	return fmt.Sprintf("%d#%s#%s", re.table, re.destination, re.gateway)
}

// RouteEntryMap: type, service functions and methods

type RouteEntryMap map[string]RouteEntry

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

// nrtSummary: type, service functions and methods

type nrtSummary struct {
	k8sResources            *v1alpha1.SDNInternalNodeRoutingTable
	newReconciliationStatus utils.ReconciliationStatus
	desiredRoutesByNRT      RouteEntryMap
	lastAppliedRoutesByNRT  RouteEntryMap
	desiredRoutesToAddByNRT []RouteEntry
	desiredRoutesToDelByNRT RouteEntryMap
	nrtWasDeleted           bool
	needToWipeFinalizer     bool
}

func nrtSummaryInit() *nrtSummary {
	return &nrtSummary{
		k8sResources:            new(v1alpha1.SDNInternalNodeRoutingTable),
		newReconciliationStatus: utils.ReconciliationStatus{},
		desiredRoutesByNRT:      RouteEntryMap{},
		lastAppliedRoutesByNRT:  RouteEntryMap{},
		desiredRoutesToAddByNRT: make([]RouteEntry, 0),
		desiredRoutesToDelByNRT: RouteEntryMap{},
		nrtWasDeleted:           false,
		needToWipeFinalizer:     false,
	}
}

func (ns *nrtSummary) discoverFacts(nrt v1alpha1.SDNInternalNodeRoutingTable, globalDesiredRoutesForNode, actualRoutesOnNode *RouteEntryMap, log logger.Logger) bool {
	// Filling nrtK8sResourcesMap[nrt.Name] and nrtReconciliationStatusMap[nrt.Name]
	tmpNrt := nrt
	tmpNrt.Status.ObservedGeneration = nrt.Generation
	ns.k8sResources = &tmpNrt
	ns.newReconciliationStatus = utils.ReconciliationStatus{IsSuccess: true}
	ns.needToWipeFinalizer = false

	// If NRT was deleted filling map desiredRoutesToDelByNRT and set flag nrtWasDeleted
	if nrt.DeletionTimestamp != nil {
		log.Debug(fmt.Sprintf("[NRTReconciler] NRT %v is marked for deletion", nrt.Name))
		log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling map routesToDel, and set flag nrtWasDeleted "))
		tmpREM := make(RouteEntryMap)
		for _, route := range nrt.Spec.Routes {
			tmpREM.AppendR(route, nrt.Spec.IPRoutingTableID)
		}
		ns.desiredRoutesToDelByNRT = tmpREM
		ns.nrtWasDeleted = true
		return true
	}

	// Filling desiredRoutesByNRT and globalDesiredRoutesForNode
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling maps: desiredRoutes and globalDesiredRoutes"))
	for _, route := range nrt.Spec.Routes {
		ns.desiredRoutesByNRT.AppendR(route, nrt.Spec.IPRoutingTableID)
		globalDesiredRoutesForNode.AppendR(route, nrt.Spec.IPRoutingTableID)
	}

	// Filling lastAppliedRoutesByNRT
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling map lastAppliedRoutes"))
	if nrt.Status.AppliedRoutes != nil {
		for _, route := range nrt.Status.AppliedRoutes {
			ns.lastAppliedRoutesByNRT.AppendR(route, nrt.Spec.IPRoutingTableID)
		}
	}

	// Filling desiredRoutesToAddByNRT
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling map routesToAdd"))
	for hash, desiredRoute := range ns.desiredRoutesByNRT {
		if _, ok := (*actualRoutesOnNode)[hash]; !ok {
			ns.desiredRoutesToAddByNRT = append(ns.desiredRoutesToAddByNRT, desiredRoute)
		}
	}

	// Filling desiredRoutesToDelByNRT
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling map routesToDel"))
	tmpREM := make(RouteEntryMap)
	for hash, route := range ns.lastAppliedRoutesByNRT {
		if _, ok := ns.desiredRoutesByNRT[hash]; !ok {
			tmpREM.AppendRE(route)
		}
	}
	ns.desiredRoutesToDelByNRT = tmpREM

	return false
}

func (ns *nrtSummary) addRoutes(actualRoutesOnNode *RouteEntryMap, log logger.Logger) {
	status := ns.newReconciliationStatus
	for _, route := range ns.desiredRoutesToAddByNRT {
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
	ns.newReconciliationStatus = status
}

// nrtMap: type, service functions and methods

type nrtMap map[string]*nrtSummary

func nrtMapInit() *nrtMap {
	newNRTMap := new(nrtMap)
	*newNRTMap = make(map[string]*nrtSummary)
	return newNRTMap
}

func (nm *nrtMap) deleteRoutesAndFinalizers(globalDesiredRoutesForNode, actualRoutesOnNode RouteEntryMap, log logger.Logger) {
	for nrtName, ns := range *nm {
		if len(ns.desiredRoutesToDelByNRT) == 0 && !ns.nrtWasDeleted {
			log.Debug(fmt.Sprintf("[NRTReconciler] NRT %v has no entries in desiredRoutesToDelByNRT and DeletionTimestamp is not set", nrtName))
			continue
		}
		log.Debug(fmt.Sprintf("[NRTReconciler] Starting to delete routes deleted from NRT %v from node", nrtName))
		status := ns.newReconciliationStatus
		ns.newReconciliationStatus = deleteRouteEntriesFromNode(
			ns.desiredRoutesToDelByNRT,
			globalDesiredRoutesForNode,
			&actualRoutesOnNode,
			status,
			log,
		)
		if ns.nrtWasDeleted && ns.newReconciliationStatus.IsSuccess {
			log.Debug(fmt.Sprintf("[NRTReconciler] NRT %v has been deleted and its routes has been successfully deleted too. The finalizer will be wiped", nrtName))
			ns.needToWipeFinalizer = true
		}
	}
}

func (nm *nrtMap) generateNewCondition() bool {
	shouldRequeue := false
	for _, ns := range *nm {
		newCond := v1alpha1.ExtendedCondition{}
		t := metav1.NewTime(time.Now())

		if ns.k8sResources.Status.Conditions == nil {
			ns.k8sResources.Status.Conditions = make([]v1alpha1.ExtendedCondition, 0)
		}

		if ns.newReconciliationStatus.IsSuccess {
			ns.k8sResources.Status.AppliedRoutes = ns.k8sResources.Spec.Routes

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
			newCond.Message = ns.newReconciliationStatus.ErrorMessage

			shouldRequeue = true
		}
		_ = utils.SetStatusCondition(&ns.k8sResources.Status.Conditions, newCond)
	}
	return shouldRequeue
}

func (nm *nrtMap) updateStateInK8S(ctx context.Context, cl client.Client, log logger.Logger) {
	for nrtName, ns := range *nm {
		// Wipe the finalizer if necessary
		if ns.needToWipeFinalizer && ns.k8sResources.DeletionTimestamp != nil {
			log.Debug(fmt.Sprintf("Wipe finalizer on NRT: %v", nrtName))

			tmpNRTFinalizers := make([]string, 0)
			for _, fnlzr := range ns.k8sResources.Finalizers {
				if fnlzr != v1alpha1.Finalizer {
					tmpNRTFinalizers = append(tmpNRTFinalizers, fnlzr)
				}
			}

			patch, err := json.Marshal(
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"finalizers": tmpNRTFinalizers,
					},
				},
			)
			if err != nil {
				log.Error(err, fmt.Sprintf("unable to marshal patch for finalizers %v, err: %v", tmpNRTFinalizers, err))
			}

			err = cl.Patch(ctx, ns.k8sResources, client.RawPatch(types.MergePatchType, patch))
			if err != nil {
				log.Error(err, fmt.Sprintf("unable to patch CR SDNInternalNodeRoutingTable %v, err: %v", nrtName, err))
			}
		}

		// Update(patch) status every time
		log.Debug(fmt.Sprintf("Update status of NRT: %v", nrtName))

		patch, err := json.Marshal(
			map[string]interface{}{
				"status": ns.k8sResources.Status,
			},
		)
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to marshal patch for status %v, err: %v", ns.k8sResources.Status, err))
		}

		err = cl.Status().Patch(ctx, ns.k8sResources, client.RawPatch(types.MergePatchType, patch))
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to patch status for CR SDNInternalNodeIPRuleSet %v, err: %v", nrtName, err))
		}
	}
}

// netlink service functions

func getActualRouteEntryMapFromNode() (RouteEntryMap, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Realm: v1alpha1.D8Realm}, netlink.RT_FILTER_REALM|netlink.RT_FILTER_TABLE)
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
	ip, dstnetIPNet, err := net.ParseCIDR(route.destination)
	if err != nil {
		return fmt.Errorf("unable to parse destination in route %v gw %v tbl %v, err: %w",
			route.destination,
			route.gateway,
			route.table,
			err,
		)
	}
	if !ip.Equal(dstnetIPNet.IP) {
		return fmt.Errorf("route %v gw %v tbl %v is incorrect, destination is not a valid network address. perhaps %v was meant",
			route.destination,
			route.gateway,
			route.table,
			dstnetIPNet.String(),
		)
	}
	gwNetIP := net.ParseIP(route.gateway)
	err = netlink.RouteAdd(&netlink.Route{
		Realm: v1alpha1.D8Realm,
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
	ip, dstnetIPNet, err := net.ParseCIDR(route.destination)
	if err != nil {
		return fmt.Errorf("unable to parse destination in route %v gw %v tbl %v, err: %w",
			route.destination,
			route.gateway,
			route.table,
			err,
		)
	}
	if !ip.Equal(dstnetIPNet.IP) {
		return fmt.Errorf("route %v gw %v tbl %v is incorrect, destination is not a valid network address. perhaps %v was meant",
			route.destination,
			route.gateway,
			route.table,
			dstnetIPNet.String(),
		)
	}
	gwNetIP := net.ParseIP(route.gateway)
	err = netlink.RouteDel(&netlink.Route{
		Realm: v1alpha1.D8Realm,
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

// other service functions

func deleteRouteEntriesFromNode(delREM, gdREM RouteEntryMap, actREM *RouteEntryMap, status utils.ReconciliationStatus, log logger.Logger) utils.ReconciliationStatus {
	for hash, route := range delREM {
		log.Debug(fmt.Sprintf("Route %v should be deleted", route))
		if _, ok := (gdREM)[hash]; ok {
			log.Debug(fmt.Sprintf("but it is present in other NRT"))
			continue
		}
		if _, ok := (*actREM)[hash]; !ok {
			log.Debug(fmt.Sprintf("but it is not present on Node"))
			continue
		}
		err := delRouteFromNode(route)
		if err == nil {
			delete(*actREM, hash)
		} else {
			log.Debug(fmt.Sprintf("err: %v", err))
			status.AppendError(err)
		}
	}
	return status
}

func deleteOrphanRoutes(gdREM, actREM RouteEntryMap, log logger.Logger) {
	log.Debug(fmt.Sprintf("[NRTReconciler] Starting to find and delete orphan routes (with realm %v) from node.", v1alpha1.D8Realm))
	for hash, route := range actREM {
		if _, ok := (gdREM)[hash]; ok {
			continue
		}
		log.Debug(fmt.Sprintf("Route %v should be deleted.", route))
		err := delRouteFromNode(route)
		if err != nil {
			log.Debug(fmt.Sprintf("Unable to delete route %v,err: %v", route, err))
		}
	}
}
