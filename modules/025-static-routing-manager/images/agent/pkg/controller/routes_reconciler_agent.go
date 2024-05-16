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
	"syscall"
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

// ============
type workingSubstance struct {
	k8sResources            *v1alpha1.NodeRoutingTable
	nrtWasDeleted           bool
	desiredRoutesToAddByNRT RouteEntryMap
	desiredRoutesToDelByNRT RouteEntryMap
	newReconciliationStatus NRTReconciliationStatus
	specNeedToUpdate        bool
}

type nrtMap map[string]workingSubstance

func (nrtMap *nrtMap) gatheringFacts() {
}

var (
	actualRoutesOnNode         RouteEntryMap
	globalDesiredRoutesForNode RouteEntryMap
)

// ============

type RouteEntry struct {
	destination string
	gateway     string
	table       int
}

func (re *RouteEntry) getHash() string {
	return fmt.Sprintf("%d#%s#%s", re.table, re.destination, re.gateway)
}

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

type NRTReconciliationStatus struct {
	IsSuccess    bool
	ErrorMessage string
}

func (s *NRTReconciliationStatus) AppendError(err error) {
	s.IsSuccess = false
	if s.ErrorMessage == "" {
		s.ErrorMessage = err.Error()
	} else {
		s.ErrorMessage = s.ErrorMessage + "\n" + err.Error()
	}
}

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

			log.Info("[NRTReconciler] starts Reconcile")
			nrt := &v1alpha1.NodeRoutingTable{}
			err := cl.Get(ctx, request.NamespacedName, nrt)
			if err != nil && !errors2.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("[NRTReconciler] unable to get NodeRoutingTable, name: %s", request.Name))
				return reconcile.Result{}, err
			}
			if nrt.Name == "" {
				log.Info(fmt.Sprintf("[NRTReconciler] seems like the NodeRoutingTable for the request %s was deleted. Reconcile retrying will stop.", request.Name))
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
			} else {
				log.Debug(fmt.Sprintf("[NRTReconciler] NodeRoutingTable %v needs to be reconciled", nrt.Name))
				tmpNRT := new(v1alpha1.NodeRoutingTable)
				*tmpNRT = *nrt
				err := SetStatusConditionPending(ctx, cl, log, tmpNRT)
				if err != nil {
					log.Error(err, fmt.Sprintf("[NRTReconciler] error"))
				}
			}

			// ============================= main logic start =============================
			// Declaring variables
			globalDesiredRouteEntryMap := make(RouteEntryMap)
			actualRouteEntryMap := make(RouteEntryMap)
			deletedNRTRouteEntryMaps := make(map[string]RouteEntryMap)
			erasedNRTRouteEntryMaps := make(map[string]RouteEntryMap)
			nrtK8sResourcesMap := make(map[string]*v1alpha1.NodeRoutingTable)
			nrtReconciliationStatusMap := make(map[string]NRTReconciliationStatus)
			specNeedToUpdate := make(map[string]bool)
			shouldRequeue := false

			// Getting all the NodeRoutingTable associated with our node
			nrtList := &v1alpha1.NodeRoutingTableList{}
			err = cl.List(ctx, nrtList, client.MatchingLabels{nodeNameLabel: cfg.NodeName})
			if err != nil && !errors2.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("[NRTReconciler] unable to list NodeRoutingTable for node %s", cfg.NodeName))
				return reconcile.Result{RequeueAfter: cfg.RequeueInterval * time.Second}, err
			}

			// Getting all routes from our node
			actualRouteEntryMap, err = getActualRouteEntryMapFromNode()
			if err != nil {
				log.Error(err, fmt.Sprintf("[NRTReconciler] unable to get Actual routes from node"))
				return reconcile.Result{RequeueAfter: cfg.RequeueInterval * time.Second}, err
			}

			for _, nrt := range nrtList.Items {
				// Gathering facts
				log.Debug(fmt.Sprintf("[NRTReconciler] Starting gather facts about nrt %v", nrt.Name))
				// Filling nrtK8sResourcesMap[nrt.Name] and nrtReconciliationStatusMap[nrt.Name]
				tmpNrt := nrt
				tmpNrt.Status.ObservedGeneration = nrt.Generation
				nrtK8sResourcesMap[nrt.Name] = &tmpNrt
				nrtReconciliationStatusMap[nrt.Name] = NRTReconciliationStatus{IsSuccess: true}
				specNeedToUpdate[nrt.Name] = false

				if nrt.DeletionTimestamp != nil {
					log.Debug(fmt.Sprintf("[NRTReconciler] NRT %v is marked for deletion", nrt.Name))
					tmpREM := make(RouteEntryMap)
					for _, route := range nrt.Spec.Routes {
						tmpREM.AppendR(route, nrt.Spec.IPRoutingTableID)
					}
					deletedNRTRouteEntryMaps[nrt.Name] = tmpREM
					continue
				}

				// Filling nrtDesiredRouteEntryMap and globalDesiredRouteEntryMap
				log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling maps: DesiredRoute and globalDesiredRoute"))
				nrtDesiredRouteEntryMap := make(RouteEntryMap)
				for _, route := range nrt.Spec.Routes {
					nrtDesiredRouteEntryMap.AppendR(route, nrt.Spec.IPRoutingTableID)
					globalDesiredRouteEntryMap.AppendR(route, nrt.Spec.IPRoutingTableID)
				}

				// Filling nrtLastAppliedRouteEntryMap
				log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling maps: LastAppliedRoute"))
				nrtLastAppliedRouteEntryMap := make(RouteEntryMap)
				if nrt.Status.AppliedRoutes != nil && len(nrt.Status.AppliedRoutes) > 0 {
					for _, route := range nrt.Status.AppliedRoutes {
						nrtLastAppliedRouteEntryMap.AppendR(route, nrt.Spec.IPRoutingTableID)
					}
				}

				// Filling routesToAdd
				log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling maps: routesToAdd"))
				routesToAdd := make([]RouteEntry, 0)
				for hash, desiredRoute := range nrtDesiredRouteEntryMap {
					if _, ok := actualRouteEntryMap[hash]; !ok {
						routesToAdd = append(routesToAdd, desiredRoute)
					}
				}

				// Filling erasedNRTRouteEntryMaps[nrt.Name]
				log.Debug(fmt.Sprintf("[NRTReconciler] Starting filling maps: erasedRoute"))
				tmpREM := make(RouteEntryMap)
				for hash, route := range nrtLastAppliedRouteEntryMap {
					if _, ok := nrtDesiredRouteEntryMap[hash]; !ok {
						tmpREM.AppendRE(route)
					}
				}
				erasedNRTRouteEntryMaps[nrt.Name] = tmpREM

				// Actions: add routes
				if len(routesToAdd) > 0 {
					log.Debug(fmt.Sprintf("[NRTReconciler] Starting adding routes to the node"))
					status := nrtReconciliationStatusMap[nrt.Name]
					for _, route := range routesToAdd {
						err := addRouteToNode(route)
						if err == nil {
							actualRouteEntryMap.AppendRE(route)
						} else {
							log.Debug(fmt.Sprintf("err: %v", err))
							status.AppendError(err)
						}
					}
					nrtReconciliationStatusMap[nrt.Name] = status
				}
			}

			// Actions: delete routes because NRT has been deleted
			log.Debug(fmt.Sprintf("[NRTReconciler] Starting deleting routes from the node (because NRT has been deleted)"))
			for nrtName, rem := range deletedNRTRouteEntryMaps {
				status := nrtReconciliationStatusMap[nrtName]
				nrtReconciliationStatusMap[nrtName] = deleteRouteEntriesFromNode(
					rem,
					globalDesiredRouteEntryMap,
					actualRouteEntryMap,
					status,
					log,
				)
				if nrtReconciliationStatusMap[nrtName].IsSuccess {
					removeFinalizer(nrtK8sResourcesMap[nrtName])
					specNeedToUpdate[nrt.Name] = true
				}
			}

			// Actions: delete routes because they were deleted from NRT
			log.Debug(fmt.Sprintf("[NRTReconciler] Starting deleting routes from the node (because they were deleted from NRT)"))
			for nrtName, rem := range erasedNRTRouteEntryMaps {
				status := nrtReconciliationStatusMap[nrtName]
				nrtReconciliationStatusMap[nrtName] = deleteRouteEntriesFromNode(
					rem,
					globalDesiredRouteEntryMap,
					actualRouteEntryMap,
					status,
					log,
				)
			}

			// Generate new condition for each processed nrt
			log.Debug(fmt.Sprintf("[NRTReconciler] Starting generate new conditions"))
			for nrtName, nrtReconciliationStatus := range nrtReconciliationStatusMap {
				newCond := v1alpha1.NodeRoutingTableCondition{}
				t := metav1.NewTime(time.Now())

				if nrtK8sResourcesMap[nrtName].Status.Conditions == nil {
					nrtK8sResourcesMap[nrtName].Status.Conditions = make([]v1alpha1.NodeRoutingTableCondition, 0)
				}

				if nrtReconciliationStatus.IsSuccess {
					nrtK8sResourcesMap[nrtName].Status.AppliedRoutes = nrtK8sResourcesMap[nrtName].Spec.Routes

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
					newCond.Message = nrtReconciliationStatus.ErrorMessage

					shouldRequeue = true
				}
				_ = SetStatusCondition(&nrtK8sResourcesMap[nrtName].Status.Conditions, newCond)
			}

			// Update state in k8s
			log.Debug(fmt.Sprintf("[NRTReconciler] Starting updating resourses in k8s"))
			for _, nrt := range nrtK8sResourcesMap {
				if specNeedToUpdate[nrt.Name] && nrt.DeletionTimestamp != nil {
					// Update spec if we need to remove the finalizer
					log.Debug(fmt.Sprintf("Update of NRT: %v", nrt.Name))
					err = cl.Update(ctx, nrt)
					if err != nil {
						log.Error(err, fmt.Sprintf("unable to update CR NodeRoutingTable %v, err: %v", nrt.Name, err))
					}
				}
				// Update status every time
				log.Debug(fmt.Sprintf("Update status of NRT: %v", nrt.Name))
				err = cl.Status().Update(ctx, nrt)
				if err != nil {
					log.Error(err, fmt.Sprintf("unable to update status for CR NodeRoutingTable %v, err: %v", nrt.Name, err))
				}
			}

			if shouldRequeue {
				log.Warning(fmt.Sprintf("[NRTReconciler] Reconciler will requeue the request, name: %s", request.Name))
				return reconcile.Result{
					RequeueAfter: cfg.RequeueInterval * time.Second,
				}, nil
			}

			// ============================= main logic end =============================

			log.Info("[NRTReconciler] ends Reconcile")
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

	return c, nil
}

func getActualRouteEntryMapFromNode() (RouteEntryMap, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Realm: d8Realm}, netlink.RT_FILTER_REALM)
	if err != nil {
		return nil, fmt.Errorf("failed get routes from node, err: %w", err)
	}
	ar := make(RouteEntryMap)

	for _, route := range routes {
		ar.AppendRE(RouteEntry{
			destination: route.Dst.String(),
			gateway:     route.Gw.String(),
			table:       route.Table,
		})
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
	if err != nil && err.Error() != syscall.EEXIST.Error() {
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
		if _, ok := gdREM[hash]; ok {
			continue
		}
		if _, ok := actREM[hash]; !ok {
			continue
		}
		log.Debug(fmt.Sprintf("Route %v should be deleted", route))
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

func SetStatusConditionPending(
	ctx context.Context,
	cl client.Client,
	log logger.Logger,
	nrt *v1alpha1.NodeRoutingTable,
) error {
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
