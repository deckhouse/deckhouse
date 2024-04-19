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
	"reflect"
	"static-routing-manager-agent/api/v1alpha1"
	"static-routing-manager-agent/pkg/config"
	"static-routing-manager-agent/pkg/logger"
	"static-routing-manager-agent/pkg/monitoring"
	"time"

	"github.com/vishvananda/netlink"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/api/core/v1"

	errors2 "k8s.io/apimachinery/pkg/api/errors"
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

type routes struct {
	route map[string]string
}

type NodeRouteTables struct {
	routeTable         map[int]routes
	status             string
	lastCheckTimestamp string
}

type nodesRoutesMap struct {
	generation int64
	node       map[string]NodeRouteTables
}

func RunRoutesReconcilerAgentController(
	mgr manager.Manager,
	cfg config.Options,
	log logger.Logger,
	metrics monitoring.Metrics,
) (controller.Controller, error) {
	cl := mgr.GetClient()

	c, err := controller.New(CtrlName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			log.Info("[RoutingTableReconciler] starts Reconcile")

			cm := &v1.ConfigMap{}
			err := cl.Get(ctx, request.NamespacedName, cm)
			if err != nil && !errors2.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("[RoutingTableReconciler] unable to get ConfigMap, name: %s", request.Name))
				return reconcile.Result{}, err
			}

			if cm.Name == "" {
				log.Info(fmt.Sprintf("[RoutingTableReconciler] seems like the ConfigMap for the request %s was deleted. Reconcile retrying will stop.", request.Name))
				return reconcile.Result{}, nil
			}

			nrmFromCM, err := getRoutesFromCMbyNodeName(cm, cfg.NodeName)
			if err != nil {
				log.Error(err, fmt.Sprintf("[RoutingTableReconciler] cant get nodeRouteMap from configmap for Node: %v", cfg.NodeName))
			}

			shouldRequeue, err := runEventReconcile(ctx, cl, log, nrmFromCM)
			if err != nil {
				log.Error(err, fmt.Sprintf("[RoutingTableReconciler] an error occured while reconciles the RoutingTable, name: %s", cm.Name))
			}

			if shouldRequeue {
				log.Warning(fmt.Sprintf("[RoutingTableReconciler] Reconciler will requeue the request, name: %s", request.Name))
				return reconcile.Result{
					RequeueAfter: cfg.RequeueInterval * time.Second,
				}, nil
			}

			log.Info("[RoutingTableReconciler] ends Reconcile")
			return reconcile.Result{}, nil
		}),
	})
	if err != nil {
		log.Error(err, "[RunRoutesReconcilerAgentController] unable to create controller")
		return nil, err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: cfg.ConfigmapName}}), &handler.EnqueueRequestForObject{})
	if err != nil {
		log.Error(err, "[RunRoutesReconcilerAgentController] unable to watch the events")
		return nil, err
	}

	return c, nil
}

func runEventReconcile(ctx context.Context, cl client.Client, log logger.Logger, cmNRM *NodeRouteTables) (bool, error) {
	nodeNRM, err := getRoutesFromNode()

	recType, err := identifyReconcileFunc(cmNRM, nodeNRM, log)
	if err != nil {
		log.Error(err, fmt.Sprintf("[runEventReconcile] unable to identify reconcile func"))
		return true, err
	}
	log.Debug(fmt.Sprintf("[runEventReconcile] reconcile operation: %s", recType))
	switch recType {
	case RoutesAreNotEqual:
		log.Debug(fmt.Sprintf("[runEventReconcile] StatusRouteTableIDReconcile starts reconciliataion"))
		return reconcileRoutesOnNodeFunc(cmNRM, nodeNRM, log)
	default:
		log.Debug(fmt.Sprintf("[runEventReconcile] the RoutingTable should not be reconciled"))
	}

	return false, nil
}

func identifyReconcileFunc(cmNRM, nodeNRM *NodeRouteTables, log logger.Logger) (reconcileType, error) {
	should := reflect.DeepEqual(cmNRM, nodeNRM)
	if should {
		return RoutesAreNotEqual, nil
	}
	return "none", nil
}

func reconcileRoutesOnNodeFunc(cmNRM, nodeNRM *NodeRouteTables, log logger.Logger) (bool, error) {
	log.Debug(fmt.Sprintf("[reconcileRoutesOnNodeFunc] Start"))

	appendToNode, deleteFromNode, err := routeTablesDeepEqual(cmNRM, nodeNRM)
	if err != nil {
		return false, fmt.Errorf("can not compare two nodeRouteTables, err: %v", err)
	}

	err = deleteRoutesFromNode(deleteFromNode)
	if err != nil {
		return false, fmt.Errorf("can not delete routes from node, err: %v", err)
	}
	err = addRoutesToNode(appendToNode)
	if err != nil {
		return false, fmt.Errorf("can not add routes to node, err: %v", err)
	}

	return false, nil
}

func getRoutesFromCMbyNodeName(cm *v1.ConfigMap, nodeName string) (*NodeRouteTables, error) {
	nrm := new(NodeRouteTables)
	if cm.Data[nodeName] == "" || cm.DeletionTimestamp != nil {
		return new(NodeRouteTables), nil
	}
	err := json.Unmarshal([]byte(cm.Data[nodeName]), &nrm)
	if err != nil {
		return nil, fmt.Errorf("invalid ConfigMap, err: %v", err)
	}
	return nrm, nil
}

func getRoutesFromNode() (*NodeRouteTables, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Realm: d8Realm}, netlink.RT_FILTER_REALM)
	if err != nil {
		return nil, fmt.Errorf("failed get routes from node, err: %v", err)
	}
	ndRrTbls := new(NodeRouteTables)

	for _, route := range routes {
		ndRrTbls.routeTable[route.Table].route[route.Dst.String()] = route.Gw.String()
	}

	return ndRrTbls, nil
}

func routeTablesDeepEqual(cm, node *NodeRouteTables) (*NodeRouteTables, *NodeRouteTables, error) {
	appendToNode := new(NodeRouteTables)
	deleteFromNode := new(NodeRouteTables)
	// deleteFromNode, err := DeepCopyNRT(node)
	// if err != nil {
	//	return nil, nil, err
	//}
	for tblId, routes := range cm.routeTable {
		if _, ok := node.routeTable[tblId]; ok {
			for dst, gw := range routes.route {
				if ndgw, ok := node.routeTable[tblId].route[dst]; ok {
					if gw != ndgw {
						deleteFromNode.routeTable[tblId].route[dst] = ndgw
						appendToNode.routeTable[tblId].route[dst] = gw
					}
				} else {
					appendToNode.routeTable[tblId].route[dst] = gw
				}
			}
		} else {
			appendToNode.routeTable[tblId] = routes
		}
	}

	for tblId, routes := range node.routeTable {
		if _, ok := cm.routeTable[tblId]; ok {
			for dst, gw := range routes.route {
				if cmgw, ok := cm.routeTable[tblId].route[dst]; ok {
					if gw != cmgw {
						deleteFromNode.routeTable[tblId].route[dst] = gw
						appendToNode.routeTable[tblId].route[dst] = cmgw
					}
				} else {
					deleteFromNode.routeTable[tblId].route[dst] = gw
				}
			}

		} else {
			deleteFromNode.routeTable[tblId] = routes
		}
	}
	return appendToNode, deleteFromNode, nil
}

func deleteRoutesFromNode(rtToDel *NodeRouteTables) error {
	for tblId, routes := range rtToDel.routeTable {
		for dst, gw := range routes.route {
			_, dstnetIPNet, err := net.ParseCIDR(dst)
			if err != nil {
				return fmt.Errorf("can't parse dst in route %v gw %v tbl %v, err: %v", tblId, dst, gw, err)
			}
			gwNetIP := net.ParseIP(gw)
			err = netlink.RouteDel(&netlink.Route{
				Realm: d8Realm,
				Table: tblId,
				Dst:   dstnetIPNet,
				Gw:    gwNetIP,
			})
			if err != nil {
				return fmt.Errorf("can't del route %v gw %v tbl %v, err: %v", tblId, dst, gw, err)
			}
		}
	}
	return nil
}

func addRoutesToNode(rtToAdd *NodeRouteTables) error {
	for tblId, routes := range rtToAdd.routeTable {
		for dst, gw := range routes.route {
			_, dstnetIPNet, err := net.ParseCIDR(dst)
			if err != nil {
				return fmt.Errorf("can't parse dst in route %v gw %v tbl %v, err: %v", tblId, dst, gw, err)
			}
			gwNetIP := net.ParseIP(gw)
			err = netlink.RouteAdd(&netlink.Route{
				Realm: d8Realm,
				Table: tblId,
				Dst:   dstnetIPNet,
				Gw:    gwNetIP,
			})
			if err != nil {
				return fmt.Errorf("can't add route %v gw %v tbl %v, err: %v", tblId, dst, gw, err)
			}
		}
	}
	return nil
}

// The dying hole

func parseRoutesCM(cm *v1.ConfigMap) (*nodesRoutesMap, error) {
	nsrm := new(nodesRoutesMap)
	for k, v := range cm.Data {
		var nrm interface{}
		err := json.Unmarshal([]byte(v), &nrm)
		if err == nil {
			switch value := nrm.(type) {
			case NodeRouteTables:
				nsrm.node[k] = value
			default:
				return nil, fmt.Errorf("invalid ConfigMap")
			}
		}
	}
	return nsrm, nil
}

func DeepCopyNRT(in *NodeRouteTables) (*NodeRouteTables, error) {
	if in == nil {
		return nil, nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	out := new(NodeRouteTables)
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

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
