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
	"errors"
	"fmt"
	"net"
	"reflect"
	"static-routing-manager-agent/api/v1alpha1"
	"static-routing-manager-agent/pkg/config"
	"static-routing-manager-agent/pkg/logger"
	"static-routing-manager-agent/pkg/monitoring"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/vishvananda/netlink"

	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	CtrlName                        = "static-routing-manager-agent"
	d8Realm                         = 216
	RoutesAreNotEqual reconcileType = "RoutesAreNotEqual"
)

type (
	reconcileType string
)

type RouteEntry struct {
	destination string
	gateway     string
	table       int
}

var (
	ErrNetworkIsUnreachable = errors.New("network is unreachable")
)

func RunRoutesReconcilerAgentController(
	mgr manager.Manager,
	cfg config.Options,
	log logger.Logger,
	metrics monitoring.Metrics,
) (controller.Controller, error) {
	eventRecorder := mgr.GetEventRecorderFor(CtrlName)
	cl := mgr.GetClient()

	c, err := controller.New(CtrlName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			log.Debug(fmt.Sprintf("[NodeRoutingTablesReconciler] Received a reconcile.Request for CR %v", request.Name))
			if request.Name != cfg.NodeName {
				log.Debug(fmt.Sprintf("[NodeRoutingTablesReconciler] This request is not intended for our node (%v)", cfg.NodeName))
				return reconcile.Result{}, nil
			}

			log.Info("[NodeRoutingTablesReconciler] starts Reconcile")
			nrt := &v1alpha1.NodeRoutingTables{}

			err := cl.Get(ctx, request.NamespacedName, nrt)
			if err != nil && !errors2.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("[NodeRoutingTablesReconciler] unable to get NodeRoutingTables, name: %s", request.Name))
				return reconcile.Result{}, err
			}

			if nrt.Name == "" {
				log.Info(fmt.Sprintf("[NodeRoutingTablesReconciler] seems like the NodeRoutingTables for the request %s was deleted. Reconcile retrying will stop.", request.Name))
				return reconcile.Result{}, nil
			}

			shouldRequeue, err := runEventReconcile(nrt, log, ctx, cl, eventRecorder)
			if err != nil {
				log.Error(err, fmt.Sprintf("[NodeRoutingTablesReconciler] an error occured while reconciles the NodeRoutingTables, name: %s", nrt.Name))
			}

			if shouldRequeue {
				log.Warning(fmt.Sprintf("[NodeRoutingTablesReconciler] Reconciler will requeue the request, name: %s", request.Name))
				return reconcile.Result{
					RequeueAfter: cfg.RequeueInterval * time.Second,
				}, nil
			}

			log.Info("[NodeRoutingTablesReconciler] ends Reconcile")
			return reconcile.Result{}, nil
		}),
	})
	if err != nil {
		log.Error(err, "[RunRoutesReconcilerAgentController] unable to create controller")
		return nil, err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &v1alpha1.NodeRoutingTables{}), &handler.EnqueueRequestForObject{})
	if err != nil {
		log.Error(err, "[RunRoutesReconcilerAgentController] unable to watch the events")
		return nil, err
	}

	return c, nil
}

func getHash(re RouteEntry) string {
	return fmt.Sprintf("%d:%s:%s", re.table, re.destination, re.gateway)
}

func getDesiredRoutesFromCR(nrt *v1alpha1.NodeRoutingTables) (map[string]RouteEntry, error) {
	dr := make(map[string]RouteEntry)

	if nrt.DeletionTimestamp != nil {
		return dr, nil
	}
	for tblIdRaw, routes := range nrt.Spec.RoutingTables {
		tblId, err := strconv.Atoi(tblIdRaw)
		if err != nil {
			return nil, err
		}
		for _, route := range routes.Routes {
			re := RouteEntry{
				destination: route.Destination,
				gateway:     route.Gateway,
				table:       tblId,
			}
			dr[getHash(re)] = re
		}

	}
	return dr, nil
}

func getActualRoutesFromNode() (map[string]RouteEntry, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Realm: d8Realm}, netlink.RT_FILTER_REALM)
	if err != nil {
		return nil, fmt.Errorf("failed get routes from node, err: %v", err)
	}
	ar := make(map[string]RouteEntry)

	for _, route := range routes {
		re := RouteEntry{
			destination: route.Dst.String(),
			gateway:     route.Gw.String(),
			table:       route.Table,
		}
		ar[getHash(re)] = re
	}

	return ar, nil
}

func compareDesiredAndActual(desiredRoutes, actualRoutes map[string]RouteEntry) (routesToAdd, routesToDel []RouteEntry) {
	routesToAdd = make([]RouteEntry, 0)
	routesToDel = make([]RouteEntry, 0)

	for hash, desiredRoute := range desiredRoutes {
		if _, ok := actualRoutes[hash]; !ok {
			routesToAdd = append(routesToAdd, desiredRoute)
		}
	}
	for hash, actualRoute := range actualRoutes {
		if _, ok := desiredRoutes[hash]; !ok {
			routesToDel = append(routesToDel, actualRoute)
		}
	}
	return routesToAdd, routesToDel
}

func deleteRoutesFromNode(routesToDel []RouteEntry) error {
	for _, route := range routesToDel {
		_, dstnetIPNet, err := net.ParseCIDR(route.destination)
		if err != nil {
			return fmt.Errorf("can't parse destination in route %v gw %v tbl %v, err: %v",
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
			return fmt.Errorf("can't del route %v gw %v tbl %v, err: %v",
				route.destination,
				route.gateway,
				route.table,
				err,
			)
		}
	}
	return nil
}

func addRoutesToNode(routesToAdd []RouteEntry) error {
	for _, route := range routesToAdd {
		_, dstnetIPNet, err := net.ParseCIDR(route.destination)
		if err != nil {
			return fmt.Errorf("can't parse destination in route %v gw %v tbl %v, err: %v",
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
			return fmt.Errorf("can't add route %v gw %v tbl %v, err: %v",
				route.destination,
				route.gateway,
				route.table,
				err,
			)
		}
	}
	return nil
}

func identifyReconcileFunc(nrt *v1alpha1.NodeRoutingTables, desiredRoutes, actualRoutes map[string]RouteEntry) (reconcileType, error) {
	// should := shouldReconcileByRouteUnequalFunc(nrt, desiredRoutes, actualRoutes)
	should := !reflect.DeepEqual(desiredRoutes, actualRoutes)
	if should {
		return RoutesAreNotEqual, nil
	}
	return "none", nil
}

func reconcileRoutesOnNodeFunc(desiredRoutes, actualRoutes map[string]RouteEntry) (bool, error) {
	routesToAdd, routesToDel := compareDesiredAndActual(desiredRoutes, actualRoutes)

	err := deleteRoutesFromNode(routesToDel)
	if err != nil {
		return true, fmt.Errorf("[NodeRoutingTablesReconciler] unable to del routes from node, err: %v", err)
	}
	err = addRoutesToNode(routesToAdd)
	if err != nil {
		if errors.Is(err, ErrNetworkIsUnreachable) {
			return false, fmt.Errorf("[NodeRoutingTablesReconciler] unable to add routes to node, err: %v", err)
		}
		return true, fmt.Errorf("[NodeRoutingTablesReconciler] unable to add routes to node, err: %v", err)
	}
	return false, nil
}

func runEventReconcile(
	nrt *v1alpha1.NodeRoutingTables,
	log logger.Logger,
	ctx context.Context,
	cl client.Client,
	eventRecorder record.EventRecorder,
) (bool, error) {
	desiredRoutes, err := getDesiredRoutesFromCR(nrt)
	if err != nil {
		return true, fmt.Errorf("[runEventReconcile] unable to get desired routes from CR %v", nrt.Name)
	}
	actualRoutes, err := getActualRoutesFromNode()
	if err != nil {
		return true, fmt.Errorf("[runEventReconcile] unable to get Actual routes from node")
	}

	recType, err := identifyReconcileFunc(nrt, desiredRoutes, actualRoutes)
	if err != nil {
		return true, fmt.Errorf("[runEventReconcile] unable to identify reconcile func")
	}
	log.Debug(fmt.Sprintf("[runEventReconcile] reconcile operation: %s", recType))
	switch recType {
	case RoutesAreNotEqual:
		log.Debug(fmt.Sprintf("[runEventReconcile] StatusRouteTableIDReconcile starts reconciliataion"))

		shouldRequeue, err := reconcileRoutesOnNodeFunc(desiredRoutes, actualRoutes)
		if err != nil {
			log.Error(err, fmt.Sprintf("[runEventReconcile] an error occured while reconciles the NodeRoutingTables, name: %s", nrt.Name))
			err2 := updateCRStatus(ctx, cl, nrt, "Failed", err.Error())
			if err2 != nil {
				log.Debug(fmt.Sprintf("[runEventReconcile] unable to update status of CR NodeRoutingTables %v, err: %v", nrt.Name, err2))
			}
			err3 := generateEvent(eventRecorder, nrt, corev1.EventTypeWarning, "RouteReconcilingFailed", err.Error())
			if err3 != nil {
				log.Debug(fmt.Sprintf("[runEventReconcile] unable to create event for CR NodeRoutingTables %v, err: %v", nrt.Name, err3))
			}
		} else {
			if nrt.DeletionTimestamp != nil {
				// delete finalizer
				err3 := generateEvent(eventRecorder, nrt, corev1.EventTypeNormal, "NodeRoutingTablesDeletionSucceed", "")
				if err3 != nil {
					log.Debug(fmt.Sprintf("[runEventReconcile] unable to create event for CR NodeRoutingTables %v, err: %v", nrt.Name, err3))
				}
			} else {
				err2 := updateCRStatus(ctx, cl, nrt, "Succeed", "")
				if err2 != nil {
					log.Debug(fmt.Sprintf("[runEventReconcile] unable to update status of CR NodeRoutingTables %v, err: %v", nrt.Name, err2))
				}
				err3 := generateEvent(eventRecorder, nrt, corev1.EventTypeWarning, "NodeRoutingTablesReconcilationSucceed", "")
				if err3 != nil {
					log.Debug(fmt.Sprintf("[runEventReconcile] unable to create event for CR NodeRoutingTables %v, err: %v", nrt.Name, err3))
				}
			}

		}
		return shouldRequeue, err
	default:
		log.Debug(fmt.Sprintf("[runEventReconcile] the RoutingTable should not be reconciled"))
	}

	return false, nil
}

func updateCRStatus(
	ctx context.Context,
	cl client.Client,
	nrt *v1alpha1.NodeRoutingTables,
	status, message string,
) error {
	if &nrt.Status == nil {
		nrt.Status = v1alpha1.NodeRoutingTablesStatus{}
	}

	if status != "" {
		nrt.Status.ReconcileStatus = status
	}
	if message != "" {
		nrt.Status.ReconcileMessage = message
	}

	err := cl.Status().Update(ctx, nrt)
	if err != nil {
		return err
	}

	return nil
}

func generateEvent(
	eventRecorder record.EventRecorder,
	nrt *v1alpha1.NodeRoutingTables,
	eventtype, reason, message string,
) error {
	if eventtype != corev1.EventTypeNormal && eventtype != corev1.EventTypeWarning {
		return fmt.Errorf("event type %v not supported", eventtype)
	}
	eventRecorder.Event(nrt, eventtype, reason, message)
	return nil
}

// The ymiralnaya yama
func shouldReconcileByEmptyStatusRouteTableIDFunc(rt *v1alpha1.RoutingTable, log logger.Logger) bool {
	if rt.DeletionTimestamp != nil {
		log.Debug(fmt.Sprintf("[shouldReconcileBy] In the RoutingTable %s DurationTimestamp(%v) is exist", rt.Name, rt.DeletionTimestamp.String()))
		return false
	}

	if &rt.Status == nil {
		log.Debug(fmt.Sprintf("[shouldReconcileBy] In the RoutingTable %s Status is not exist", rt.Name))
		return true
	}

	if &rt.Status.IPRouteTableID == nil {
		log.Debug(fmt.Sprintf("[shouldReconcileBy] In the RoutingTable %s Status.IPRouteTableID is not exist", rt.Name))
		return true
	}

	if rt.Status.IPRouteTableID == 0 {
		log.Debug(fmt.Sprintf("[shouldReconcileBy] In the RoutingTable %s Status.IPRouteTableID is set to 0", rt.Name))
		return true
	}

	if &rt.Spec.IPRouteTableID == nil || (&rt.Spec.IPRouteTableID != nil && rt.Spec.IPRouteTableID == 0) {
		log.Debug(fmt.Sprintf("[shouldReconcileBy] In the RoutingTable %s Status.IPRouteTableID(%v) is present but Spec.IPRouteTableID is not exist or eq 0", rt.Name, rt.Status.IPRouteTableID))
		return false
	}

	if rt.Status.IPRouteTableID == rt.Spec.IPRouteTableID {
		log.Debug(fmt.Sprintf("[shouldReconcileBy] In the RoutingTable %s Status.IPRouteTableID(%v) and Spec.IPRouteTableID(%v) are both present, they have the same value, and it is not equil to 0", rt.Name, rt.Status.IPRouteTableID, rt.Spec.IPRouteTableID))
		return false
	}

	log.Debug(fmt.Sprintf("[shouldReconcileBy] Reconcile by default"))
	return true
}
