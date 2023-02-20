/*
Copyright 2023 Flant JSC

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

package controllers

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/node/addressing"
	"github.com/vishvananda/netlink"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	virtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log    = ctrl.Log.WithName("vmi-router")
	vmiIPs = map[string]string{}
)

const table = 1490

type VMIRouterController struct {
	RESTClient        rest.Interface
	CIDRs             []*net.IPNet
	RouteGet          func(net.IP) ([]netlink.Route, error)
	RouteDel          func(*netlink.Route) error
	RouteReplace      func(*netlink.Route) error
	RouteListFiltered func(int, *netlink.Route, uint64) ([]netlink.Route, error)
	RuleAdd           func(*netlink.Rule) error
	RuleDel           func(*netlink.Rule) error
	RuleListFiltered  func(int, *netlink.Rule, uint64) ([]netlink.Rule, error)
	client.Client
}

func (c VMIRouterController) Start(ctx context.Context) error {
	log.Info("starting vmi routes controller")

	lw := cache.NewListWatchFromClient(c.RESTClient, "virtualmachineinstances", v1.NamespaceAll, fields.Everything())
	informer := cache.NewSharedIndexInformer(lw, &virtv1.VirtualMachineInstance{}, 12*time.Hour,
		cache.Indexers{
			"namespace_name": func(obj interface{}) ([]string, error) {
				return []string{obj.(*virtv1.VirtualMachineInstance).GetName()}, nil
			},
		},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addFunc,
		DeleteFunc: c.deleteFunc,
		UpdateFunc: c.updateFunc,
	})

	stopper := make(chan struct{})
	defer close(stopper)
	defer utilruntime.HandleCrash()
	go informer.Run(stopper)
	log.Info("syncronizing")

	//syncronize the cache before starting to process
	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		log.Info("syncronization failed")
		return fmt.Errorf("syncronization failed")
	}
	log.Info("syncronization completed")

	log.Info("create routing rules")
	if err := c.setupRules(); err != nil {
		return fmt.Errorf("failed to create routing rules: %w", err)
	}

	log.Info("running cleanup for removed VMIs")
	if err := c.cleanupRemovedVMIs(informer); err != nil {
		return fmt.Errorf("failed to cleanup removed VMIs: %w", err)
	}
	log.Info("cleanup of removed VMIs completed")

	<-ctx.Done()
	log.Info("shutting down vmi router controller")

	return nil
}

func (c VMIRouterController) addFunc(obj interface{}) {
	vmi, ok := obj.(*virtv1.VirtualMachineInstance)
	if !ok {
		// object is not VMI
		return
	}
	c.updateRoute(vmi)
}
func (c VMIRouterController) deleteFunc(obj interface{}) {
	vmi, ok := obj.(*virtv1.VirtualMachineInstance)
	if !ok {
		// object is not VMI
		return
	}

	vmiKey := fmt.Sprintf("%s/%s", vmi.GetNamespace(), vmi.GetName())
	vmiIP, ok := vmiIPs[vmiKey]
	if !ok {
		// VMI already removed
		return
	}
	_, vmiNet, err := net.ParseCIDR(appendNetmask(vmiIP))
	if err != nil {
		log.Error(err, "failed to parse CIDR for vmi")
		return
	}

	route := netlink.Route{
		Dst:   vmiNet,
		Table: table,
	}

	log.Info(fmt.Sprintf("deleting route for %s %s", vmiKey, route))
	if err := c.RouteDel(&route); err != nil && !os.IsNotExist(err) {
		log.Error(err, "failed to delete route")
	}

	// Delete IP from in-memory map
	delete(vmiIPs, vmiKey)
}

func (c VMIRouterController) updateFunc(oldObj, newObj interface{}) {
	newVMI, ok := newObj.(*virtv1.VirtualMachineInstance)
	if !ok {
		// object is not VMI
		return
	}
	c.updateRoute(newVMI)
}

func (c VMIRouterController) updateRoute(vmi *virtv1.VirtualMachineInstance) {
	if vmi.Status.NodeName == "" {
		// VMI has no node assigned
		return
	}
	vmiIP := getVMIPodNetworkIPAddress(vmi)
	if vmiIP == "" {
		// VMI has no IP address assigned
		return
	}
	vmiIPx := net.ParseIP(vmiIP)
	if len(vmiIPx) == 0 {
		log.Error(fmt.Errorf(vmiIP), "failed to parse IP address")
		return
	}
	if !c.ipIsManaged(vmiIPx) {
		return
	}
	_, vmiNet, err := net.ParseCIDR(appendNetmask(vmiIP))
	if err != nil {
		log.Error(err, "failed to parse CIDR for vmi")
		return
	}

	// Save IP to in-memory map to have an oportunity remove it later
	vmiKey := fmt.Sprintf("%s/%s", vmi.GetNamespace(), vmi.GetName())
	vmiIPs[vmiKey] = vmiIP

	ciliumNode := &ciliumv2.CiliumNode{}
	err = c.Client.Get(context.TODO(), types.NamespacedName{Namespace: "", Name: vmi.Status.NodeName}, ciliumNode)
	if err != nil {
		log.Error(err, "failed to get cilium node for vmi")
	}
	nodeIP := getCiliumInternalIPAddress(ciliumNode)
	if nodeIP == "" {
		log.Error(nil, "CiliumNode has no %s specified\n", addressing.NodeCiliumInternalIP)
		return
	}
	nodeIPx := net.ParseIP(nodeIP)
	if len(nodeIPx) == 0 {
		log.Error(fmt.Errorf(nodeIP), "failed to parse IP address")
		return
	}

	// Get route for specific nodeIP and create similar for our VMI
	routes, err := c.RouteGet(nodeIPx)
	if err != nil || len(routes) == 0 {
		log.Error(err, "failed to get route for node")
	}
	route := routes[0]

	// If table is `local`
	if route.Table == 255 {
		iface, err := netlink.LinkByName("cilium_host")
		if err != nil {
			log.Error(err, "failed to get interface")
			os.Exit(1)
		}
		// Overwrite `lo` interface with `cilium_host`
		route.LinkIndex = iface.Attrs().Index
	}

	route.Dst = vmiNet
	route.Table = table
	route.Type = 1

	log.Info(fmt.Sprintf("updating route for %s %s", vmiKey, route))
	if err := c.RouteReplace(&route); err != nil {
		log.Error(err, "failed to update route")
	}
}

// Runs cleanup for removed VMIs
func (c VMIRouterController) cleanupRemovedVMIs(informer cache.SharedIndexInformer) error {
	var existingIPs []string
	// Collect all IPs in a cluster
	for _, obj := range informer.GetIndexer().List() {
		vmi, ok := obj.(*virtv1.VirtualMachineInstance)
		if !ok {
			return fmt.Errorf("failed to cast obj to vmi: %v", obj)
		}
		vmiIP := getVMIPodNetworkIPAddress(vmi)
		if vmiIP == "" {
			continue
		}
		vmiIPx := net.ParseIP(vmiIP)
		if len(vmiIPx) == 0 {
			log.Error(fmt.Errorf(vmiIP), "failed to parse IP address")
			continue
		}
		if !c.ipIsManaged(vmiIPx) {
			continue
		}
		existingIPs = append(existingIPs, vmiIP)
	}

	nodeRoutes, err := c.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{Table: table}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return fmt.Errorf("failed to list node routes: %v", err)
	}

	// Remove all routes for non existingIPs
LOOP:
	for _, route := range nodeRoutes {
		for _, vmiIP := range existingIPs {
			if route.Dst != nil && route.Dst.IP.String() == vmiIP {
				continue LOOP
			}
		}
		if err := c.RouteDel(&route); err != nil {
			return fmt.Errorf("failed to delete route: %v", err)
		}
		log.Info(fmt.Sprintf("deleted route %s", route.String()))
	}
	return nil
}

func getVMIPodNetworkIPAddress(vmi *virtv1.VirtualMachineInstance) string {
	for _, network := range vmi.Spec.Networks {
		if network.Multus != nil {
			continue
		}
		for _, iface := range vmi.Status.Interfaces {
			if iface.Name == network.Name {
				return iface.IP
			}
		}
	}
	return ""
}

func getCiliumInternalIPAddress(node *ciliumv2.CiliumNode) string {
	for _, address := range node.Spec.Addresses {
		if address.Type == addressing.NodeCiliumInternalIP {
			return address.IP
		}
	}
	return ""
}

func (c VMIRouterController) ipIsManaged(ip net.IP) bool {
	for _, cidr := range c.CIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func (c VMIRouterController) setupRules() error {
	rules, err := c.RuleListFiltered(netlink.FAMILY_ALL, &netlink.Rule{Table: table}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return fmt.Errorf("failed to list rules: %v", err)
	}

	// Configure new CIDRs
	for _, cidr := range c.CIDRs {
		rule := netlink.NewRule()
		rule.Table = table
		rule.Priority = table
		rule.Dst = cidr
		if err := c.RuleAdd(rule); err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to add rule: %v", err)
		}
		log.Info(fmt.Sprintf("loaded %s", rule.String()))
	}

	// Remove old CIDRs
LOOP:
	for _, rule := range rules {
		for _, cidr := range c.CIDRs {
			if rule.Dst != nil && rule.Dst.String() == cidr.String() {
				// Rule already exists
				continue LOOP
			}
		}
		c.RuleDel(&rule)
		log.Info(fmt.Sprintf("deleted %s", rule.String()))
	}

	return nil
}

func appendNetmask(ip string) string {
	if strings.Contains(ip, "/") {
		// IP already contains netmask
		return ip
	}
	if strings.Contains(ip, ":") {
		// IPv6
		return ip + "/128"
	} else {
		// IPv4
		return ip + "/32"
	}
}
