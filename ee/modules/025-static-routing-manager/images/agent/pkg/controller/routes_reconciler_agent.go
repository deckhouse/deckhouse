/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/api/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/pkg/config"
	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/pkg/utils"

	"github.com/go-logr/logr"

	"github.com/vishvananda/netlink"
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
	log logr.Logger,
) (controller.Controller, error) {
	cl := mgr.GetClient()

	c, err := controller.New(routeCtrlName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Received a reconcile.Request for CR %v", request.Name))

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
				log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] This request is not intended(by label) for our node (%v)", cfg.NodeName))
				return reconcile.Result{}, nil
			}
			if nrt.Spec.NodeName != cfg.NodeName {
				log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] This request is not intended(by spec.nodeName) for our node (%v)", cfg.NodeName))
				return reconcile.Result{}, nil
			}

			if nrt.Generation == nrt.Status.ObservedGeneration && nrt.DeletionTimestamp == nil {
				cond := utils.FindStatusCondition(nrt.Status.Conditions, v1alpha1.ReconciliationSucceedType)
				if cond != nil && cond.Status == metav1.ConditionTrue {
					log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] There's nothing to do"))
					return reconcile.Result{}, nil
				}
			}
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] SDNInternalNodeRoutingTable %v needs to be reconciled. Set status to Pending", nrt.Name))
			tmpNRT := new(v1alpha1.SDNInternalNodeRoutingTable)
			*tmpNRT = *nrt

			if nrt.Generation != nrt.Status.ObservedGeneration {
				err = utils.SetStatusConditionPendingToNRT(ctx, cl, log, tmpNRT)
				if err != nil {
					log.Error(err, fmt.Sprintf("[NRTReconciler] Unable to set status to Pending for NRT %v", nrt.Name))
				}
			}

			// ============================= main logic start =============================
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starts of the reconciliation (initiated by the k8s-event)"))
			shouldRequeue, err := runEventRouteReconcile(ctx, cl, log, cfg.NodeName)
			if err != nil {
				log.Error(err, fmt.Sprintf("[NRTReconciler] An error occurred while route reconcile"))
			}

			if shouldRequeue {
				log.V(config.WarnLvl).Info(fmt.Sprintf("[NRTReconciler] Reconciler will requeue the request, name: %s", request.Name))
				return reconcile.Result{
					RequeueAfter: cfg.RequeueInterval * time.Second,
				}, nil
			}
			// ============================= main logic end =============================

			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] End of the reconciliation (initiated by the k8s-event)"))
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
	log logr.Logger,
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
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Getting all routes from our node"))
	actualRoutesOnNode, err = getActualRouteEntryMapFromNode()
	if err != nil {
		log.Error(err, fmt.Sprintf("[NRTReconciler] unable to get Actual routes from node"))
		return true, err
	}
	if len(actualRoutesOnNode) == 0 {
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] There are no routes with Realm=" + strconv.Itoa(v1alpha1.D8Realm)))
	}

	for _, nrt := range nrtList.Items {
		nrtSummary := nrtSummaryInit()
		// Gathering facts
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting gather facts about nrt %v", nrt.Name))
		if nrtSummary.discoverFacts(nrt, &globalDesiredRoutesForNode, &actualRoutesOnNode, log) {
			(*nrtMap)[nrt.Name] = nrtSummary
			continue
		}

		// Actions: add routes
		if len(nrtSummary.desiredRoutesToAddByNRT) > 0 {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting adding routes to the node"))
			nrtSummary.addRoutes(&actualRoutesOnNode, log)
		}

		(*nrtMap)[nrt.Name] = nrtSummary
	}

	// Actions: delete routes and finalizers (based on each NRT)
	nrtMap.deleteRoutesAndFinalizers(globalDesiredRoutesForNode, actualRoutesOnNode, log)

	// Actions: Deleting orphan routes (with realm 216) that are not mentioned in any NRT
	deleteOrphanRoutes(globalDesiredRoutesForNode, actualRoutesOnNode, log)

	// Generate new condition for each processed nrt
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting generate new conditions"))
	shouldRequeue := nrtMap.generateNewCondition()

	// Update state in k8s for each processed nrt
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting updating resourses in k8s"))
	nrtMap.updateStateInK8S(ctx, cl, log)

	return shouldRequeue, nil
}

func periodicalRunEventReconcile(
	ctx context.Context,
	cfg config.Options,
	cl client.Client,
	log logr.Logger,
	nodeName string,
) {
	ticker := time.NewTicker(cfg.PeriodicReconciliationInterval * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starts a periodic reconciliation (initiated by a timer)"))
			_, err := runEventRouteReconcile(ctx, cl, log, nodeName)
			if err != nil {
				log.Error(err, fmt.Sprintf("[NRTReconciler] an error occurred while route reconcile"))
			}
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Ends a periodic reconciliation (initiated by a timer)"))
		case <-ctx.Done():
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Completion of periodic reconciliations"))
			return
		}
	}
}

// RouteEntry: type, service functions and methods

type RouteEntry struct {
	table       int
	destination string
	gateway     string
	dev         string
	devId       int
}

func (re *RouteEntry) String() string {
	hashRaw := make([]string, 0, 5)
	hashRaw = append(hashRaw, strconv.Itoa(re.table))
	hashRaw = append(hashRaw, re.destination)
	hashRaw = append(hashRaw, re.gateway)
	hashRaw = append(hashRaw, re.dev)
	hashRaw = append(hashRaw, strconv.Itoa(re.devId))
	return strings.Join(hashRaw, "#")
}

func (re *RouteEntry) getHash() string {
	return re.String()
}

func (re *RouteEntry) getRoute() v1alpha1.Route {
	preparedRoute := v1alpha1.Route{}
	if re.destination != "" {
		preparedRoute.Destination = re.destination
	}
	if re.gateway != "" {
		preparedRoute.Gateway = re.gateway
	}
	if re.dev != "" {
		preparedRoute.Dev = re.dev
	}
	return preparedRoute
}

func (re *RouteEntry) getNetlinkRoute() (*netlink.Route, error) {
	// Prepare route for netlink
	preparedNetlinkRoute := new(netlink.Route)

	if re.table > 0 {
		preparedNetlinkRoute.Table = re.table
	}
	if re.destination != "" {
		ip, dstnetIPNet, err := net.ParseCIDR(re.destination)
		if err != nil {
			return nil, fmt.Errorf("unable to parse destination in route %v, err: %w",
				*re,
				err,
			)
		}
		if !ip.Equal(dstnetIPNet.IP) {
			return nil, fmt.Errorf("route %v is incorrect, destination is not a valid network address. Perhaps %v was meant",
				*re,
				dstnetIPNet.String(),
			)
		}
		preparedNetlinkRoute.Dst = dstnetIPNet
	}
	if re.gateway != "" {
		preparedNetlinkRoute.Gw = net.ParseIP(re.gateway)
	}
	if re.dev != "" && re.devId != 0 {
		preparedNetlinkRoute.LinkIndex = re.devId
	}

	return preparedNetlinkRoute, nil
}

func getRouteEntryFromNetlinkRoute(netlinkRoute netlink.Route) (RouteEntry, error) {
	preparedRE := RouteEntry{}

	if netlinkRoute.Dst != nil {
		preparedRE.destination = netlinkRoute.Dst.String()
	}
	if netlinkRoute.Gw != nil {
		preparedRE.gateway = netlinkRoute.Gw.String()
	}
	if netlinkRoute.LinkIndex > 0 {
		preparedRE.devId = netlinkRoute.LinkIndex
		link, err := netlink.LinkByIndex(netlinkRoute.LinkIndex)
		if err != nil {
			return RouteEntry{}, fmt.Errorf("can not find Link by Index %v, err: %w",
				netlinkRoute.LinkIndex,
				err,
			)
		}
		preparedRE.dev = link.Attrs().Name
	}
	if netlinkRoute.Table > 0 {
		preparedRE.table = netlinkRoute.Table
	}

	return preparedRE, nil
}

func getRouteEntryFromRouteAndTable(route v1alpha1.Route, tbl int) (RouteEntry, error) {
	preparedRE := RouteEntry{
		destination: route.Destination,
		table:       tbl,
	}

	if route.Gateway != "" {
		preparedRE.gateway = route.Gateway
	}
	if route.Dev != "" {
		link, err := netlink.LinkByName(route.Dev)
		if err != nil {
			return RouteEntry{}, fmt.Errorf("can not find Link by Name %v, err: %w",
				route.Dev,
				err,
			)
		}
		preparedRE.dev = route.Dev
		preparedRE.devId = link.Attrs().Index
	}
	if route.Gateway != "" && route.Dev == "" {
		// gwrt, err := netlink.RouteGetWithOptions(net.ParseIP(re.gateway), nil)
		gwRts, err := netlink.RouteGet(net.ParseIP(preparedRE.gateway))
		if err != nil || len(gwRts) == 0 {
			return RouteEntry{}, fmt.Errorf("can not find egress Link by route to Gateway %v, err: %w",
				route.Gateway,
				err,
			)
		}
		if len(gwRts) > 1 {
			fmt.Errorf("more then one egress Link found for Gateway %v. Used only first: %v",
				route.Gateway,
				gwRts[0].String(),
			)
		}
		preparedRE.devId = gwRts[0].LinkIndex

		link, err := netlink.LinkByIndex(gwRts[0].LinkIndex)
		if err == nil {
			preparedRE.dev = link.Attrs().Name
		} else {
			return RouteEntry{}, fmt.Errorf("can not find Link by Index %v (for Gateway %v), err: %w",
				gwRts[0].LinkIndex,
				route.Gateway,
				err,
			)
		}
	}
	return preparedRE, nil
}

// RouteEntryMap: type, service functions and methods

type RouteEntryMap map[string]RouteEntry

func (rem *RouteEntryMap) getRoutes() v1alpha1.Routes {
	preparedRoutes := make([]v1alpha1.Route, 0)
	for _, re := range *rem {
		re.getRoute()
		preparedRoutes = append(preparedRoutes, re.getRoute())
	}
	return v1alpha1.Routes{
		Routes: preparedRoutes,
	}
}

func (rem *RouteEntryMap) AppendRE(re RouteEntry) {
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
		desiredRoutesByNRT:      make(RouteEntryMap),
		lastAppliedRoutesByNRT:  make(RouteEntryMap),
		desiredRoutesToAddByNRT: make([]RouteEntry, 0),
		desiredRoutesToDelByNRT: make(RouteEntryMap),
		nrtWasDeleted:           false,
		needToWipeFinalizer:     false,
	}
}

func (ns *nrtSummary) discoverFacts(nrt v1alpha1.SDNInternalNodeRoutingTable, globalDesiredRoutesForNode, actualRoutesOnNode *RouteEntryMap, log logr.Logger) bool {
	// Filling nrtK8sResourcesMap[nrt.Name] and nrtReconciliationStatusMap[nrt.Name]
	tmpNrt := nrt
	tmpNrt.Status.ObservedGeneration = nrt.Generation
	ns.k8sResources = &tmpNrt
	ns.newReconciliationStatus = utils.ReconciliationStatus{IsSuccess: true}
	ns.needToWipeFinalizer = false

	// Generate REM from CR's routes and table
	remFromNRTSpecRoutes := make(RouteEntryMap)
	for _, route := range nrt.Spec.Routes {
		re, err := getRouteEntryFromRouteAndTable(route, nrt.Spec.IPRoutingTableID)
		if err == nil {
			remFromNRTSpecRoutes.AppendRE(re)
		} else {
			ns.newReconciliationStatus.AppendError(
				fmt.Errorf("the route (%v, tbl %v) could not be processed, err: %w", route, nrt.Spec.IPRoutingTableID, err),
			)
		}
	}
	if !ns.newReconciliationStatus.IsSuccess {
		return true
	}

	// If NRT was deleted filling map desiredRoutesToDelByNRT and set flag nrtWasDeleted
	if nrt.DeletionTimestamp != nil {
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] NRT %v is marked for deletion", nrt.Name))
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting filling map routesToDel, and set flag nrtWasDeleted "))
		ns.desiredRoutesToDelByNRT = remFromNRTSpecRoutes
		ns.nrtWasDeleted = true
		return true
	}

	// Filling desiredRoutesByNRT and globalDesiredRoutesForNode
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting filling maps: desiredRoutes and globalDesiredRoutes"))
	ns.desiredRoutesByNRT = remFromNRTSpecRoutes
	maps.Copy(*globalDesiredRoutesForNode, remFromNRTSpecRoutes)

	// Filling lastAppliedRoutesByNRT
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting filling map lastAppliedRoutes"))
	tmpREM := make(RouteEntryMap)
	if nrt.Status.AppliedRoutes != nil {
		for _, route := range nrt.Status.AppliedRoutes {
			re, err := getRouteEntryFromRouteAndTable(route, nrt.Spec.IPRoutingTableID)
			if err != nil {
				log.V(config.DebugLvl).Info(fmt.Sprintf(
					"[NRTReconciler] Something went wrong while processing lastApplied route (%v, tbl %v), err: %v",
					route,
					nrt.Spec.IPRoutingTableID,
					err,
				))
				tmpREM = RouteEntryMap{}
				break
			}
			tmpREM.AppendRE(re)
		}
	}
	ns.lastAppliedRoutesByNRT = tmpREM

	// Filling desiredRoutesToAddByNRT
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting filling map routesToAdd"))
	for hash, desiredRoute := range ns.desiredRoutesByNRT {

		if _, ok := (*actualRoutesOnNode)[hash]; !ok {
			ns.desiredRoutesToAddByNRT = append(ns.desiredRoutesToAddByNRT, desiredRoute)
		}
	}

	// Filling desiredRoutesToDelByNRT
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting filling map routesToDel"))
	tmpREM = RouteEntryMap{}
	for hash, re := range ns.lastAppliedRoutesByNRT {
		if _, ok := ns.desiredRoutesByNRT[hash]; !ok {
			tmpREM.AppendRE(re)
		}
	}
	ns.desiredRoutesToDelByNRT = tmpREM

	return false
}

func (ns *nrtSummary) addRoutes(actualRoutesOnNode *RouteEntryMap, log logr.Logger) {
	status := ns.newReconciliationStatus
	for _, re := range ns.desiredRoutesToAddByNRT {
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Route %v should be added", re))
		if _, ok := (*actualRoutesOnNode)[re.getHash()]; ok {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] but it is already present on Node"))
			continue
		}
		err := addRouteToNode(re)
		if err == nil {
			actualRoutesOnNode.AppendRE(re)
		} else {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] err: %v", err))
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

func (nm *nrtMap) deleteRoutesAndFinalizers(globalDesiredRoutesForNode, actualRoutesOnNode RouteEntryMap, log logr.Logger) {
	for nrtName, ns := range *nm {
		if len(ns.desiredRoutesToDelByNRT) == 0 && !ns.nrtWasDeleted {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] NRT %v has no entries in desiredRoutesToDelByNRT and DeletionTimestamp is not set", nrtName))
			continue
		}
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting to delete routes deleted from NRT %v from node", nrtName))
		status := ns.newReconciliationStatus
		ns.newReconciliationStatus = deleteRouteEntriesFromNode(
			ns.desiredRoutesToDelByNRT,
			globalDesiredRoutesForNode,
			&actualRoutesOnNode,
			status,
			log,
		)
		if ns.nrtWasDeleted && ns.newReconciliationStatus.IsSuccess {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] NRT %v has been deleted and its routes has been successfully deleted too. The finalizer will be wiped", nrtName))
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

func (nm *nrtMap) updateStateInK8S(ctx context.Context, cl client.Client, log logr.Logger) {
	for nrtName, ns := range *nm {
		// Wipe the finalizer if necessary
		if ns.needToWipeFinalizer && ns.k8sResources.DeletionTimestamp != nil {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] wipe finalizer on NRT: %v", nrtName))

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
				log.Error(err, fmt.Sprintf("[NRTReconciler] unable to marshal patch for finalizers %v, err: %v", tmpNRTFinalizers, err))
			}

			err = cl.Patch(ctx, ns.k8sResources, client.RawPatch(types.MergePatchType, patch))
			if err != nil {
				log.Error(err, fmt.Sprintf("[NRTReconciler] unable to patch CR SDNInternalNodeRoutingTable %v, err: %v", nrtName, err))
			}
		}

		// Update(patch) status every time
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] update status of NRT: %v", nrtName))

		patch, err := json.Marshal(
			map[string]interface{}{
				"status": ns.k8sResources.Status,
			},
		)
		if err != nil {
			log.Error(err, fmt.Sprintf("[NRTReconciler] unable to marshal patch for status %v, err: %v", ns.k8sResources.Status, err))
		}

		err = cl.Status().Patch(ctx, ns.k8sResources, client.RawPatch(types.MergePatchType, patch))
		if err != nil {
			log.Error(err, fmt.Sprintf("[NRTReconciler] unable to patch status for CR SDNInternalNodeIPRuleSet %v, err: %v", nrtName, err))
		}
	}
}

// netlink service functions

func getActualRouteEntryMapFromNode() (RouteEntryMap, error) {
	netlinkRoutes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Realm: v1alpha1.D8Realm}, netlink.RT_FILTER_REALM|netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, fmt.Errorf("failed get routes from node, err: %w", err)
	}
	ar := make(RouteEntryMap)

	for _, route := range netlinkRoutes {
		re, err := getRouteEntryFromNetlinkRoute(route)
		if err != nil {
			return nil, fmt.Errorf("the route (%v) could not be processed, err: %w", route.String(), err)
		}
		ar.AppendRE(re)
	}
	return ar, nil
}

func addRouteToNode(re RouteEntry) error {
	preparedRoute, err := re.getNetlinkRoute()
	if err != nil {
		return fmt.Errorf("unable to parse Route %v, err: %w",
			re,
			err,
		)
	}
	preparedRoute.Realm = v1alpha1.D8Realm

	err = netlink.RouteAdd(preparedRoute)
	if err != nil {
		return fmt.Errorf("unable to add route %v, err: %w",
			re,
			err,
		)
	}
	return nil
}

func delRouteFromNode(re RouteEntry) error {
	preparedRoute, err := re.getNetlinkRoute()
	if err != nil {
		return fmt.Errorf("unable to parse Route %v, err: %w",
			re,
			err,
		)
	}
	preparedRoute.Realm = v1alpha1.D8Realm

	err = netlink.RouteDel(preparedRoute)
	if err != nil {
		return fmt.Errorf("unable to del route %v, err: %w",
			re,
			err,
		)
	}
	return nil
}

// other service functions

func deleteRouteEntriesFromNode(delREM, gdREM RouteEntryMap, actREM *RouteEntryMap, status utils.ReconciliationStatus, log logr.Logger) utils.ReconciliationStatus {
	for hash, re := range delREM {
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] route %v should be deleted", re))
		if _, ok := (gdREM)[hash]; ok {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] but it is present in other NRT"))
			continue
		}
		if _, ok := (*actREM)[hash]; !ok {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] but it is not present on Node"))
			continue
		}
		err := delRouteFromNode(re)
		if err == nil {
			delete(*actREM, hash)
		} else {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] err: %v", err))
			status.AppendError(err)
		}
	}
	return status
}

func deleteOrphanRoutes(gdREM, actREM RouteEntryMap, log logr.Logger) {
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] Starting to find and delete orphan routes (with realm %v) from node.", v1alpha1.D8Realm))
	for hash, re := range actREM {
		if _, ok := (gdREM)[hash]; ok {
			continue
		}
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] route %v should be deleted.", re))
		err := delRouteFromNode(re)
		if err != nil {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NRTReconciler] unable to delete route %v,err: %v", re, err))
		}
	}
}
