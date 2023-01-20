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
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/vishvananda/netlink"
	v1 "k8s.io/api/core/v1"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	log = ctrl.Log.WithName("vmi-router")
)

const (
	VMIRoutesBucket  = "vmi_routes"
	CIDRRoutesBucket = "cidr_routes"
)

type VMIRouterController struct {
	RESTClient     rest.Interface
	NodeName       string
	DB             *bolt.DB
	CIDRs          []*net.IPNet
	RouteLocal     bool
	RouteAdd       func(*netlink.Route) error
	RouteDel       func(*netlink.Route) error
	HostIfaceIndex int
	client.Client
}

type CachedRoute struct {
	IP       string `json:"ip"`
	NodeName string `json:"nodeName"`
	NodeIP   string `json:"nodeIP"`
}

func (a *CachedRoute) Equal(b *CachedRoute) bool {
	if a.IP != b.IP {
		return false
	}
	if a.NodeName != b.NodeName {
		return false
	}
	if a.NodeIP != b.NodeIP {
		return false
	}
	return true
}

func (c VMIRouterController) Start(ctx context.Context) error {
	//update CIDR routes
	if err := c.syncCIDRRoutes(); err != nil {
		return fmt.Errorf("failed to cleanup CIDRs: %w", err)
	}
	if c.RouteLocal {
		// Noting to do
		return nil
	}

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
	vmi := obj.(apiv1.Object)
	dbKey := vmi.GetNamespace() + "/" + vmi.GetName()
	cached := c.getCachedRoute(VMIRoutesBucket, dbKey)
	if cached.IP != "" {
		log.Info(fmt.Sprintf("deleting route for %s/%s (%s) via %s (%s)", vmi.GetNamespace(), vmi.GetName(), cached.IP, cached.NodeName, cached.NodeIP))
		if err := c.deleteCachedRoute(VMIRoutesBucket, dbKey); err != nil {
			log.Error(err, "failed to delete route")
		}
	}
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
		// VMI has no IP for podNetwork
		return
	}

	parsedIP := net.ParseIP(vmiIP)
	if len(parsedIP) == 0 {
		log.Error(fmt.Errorf(vmiIP), "failed to parse IP address")
		return
	}

	dbKey := vmi.GetNamespace() + "/" + vmi.GetName()
	cached := c.getCachedRoute(VMIRoutesBucket, dbKey)

	if !c.ipIsManaged(parsedIP) {
		c.deleteCachedRouteIfExists(VMIRoutesBucket, dbKey, &cached)
		// IP is not managed
		return
	}

	// Fetch the Node IP address
	node := &v1.Node{}
	err := c.Client.Get(context.TODO(), types.NamespacedName{Namespace: "", Name: vmi.Status.NodeName}, node)
	if err != nil {
		log.Error(err, "failed to get node")
		// Error reading the object - requeue the request.
		return
	}
	nodeIP := getNodeInternalIPAddress(node)

	toCache := CachedRoute{
		IP:       vmiIP,
		NodeName: node.GetName(),
		NodeIP:   nodeIP,
	}

	if cached.Equal(&toCache) {
		err = c.addCachedRoute(VMIRoutesBucket, dbKey, toCache)
		if err != nil && !os.IsExist(err) {
			log.Error(err, "failed to add route")
			return
		}
		log.Info(fmt.Sprintf("loaded route for %s/%s (%s) via %s (%s)", vmi.GetNamespace(), vmi.GetName(), vmiIP, node.GetName(), nodeIP))
		// No changes
		return
	}

	// Old route found
	c.deleteCachedRouteIfExists(VMIRoutesBucket, dbKey, &cached)

	log.Info(fmt.Sprintf("adding route for %s/%s (%s) via %s (%s)", vmi.GetNamespace(), vmi.GetName(), vmiIP, node.GetName(), nodeIP))
	err = c.addCachedRoute(VMIRoutesBucket, dbKey, toCache)
	if err != nil && !os.IsExist(err) {
		log.Error(err, "failed to add route")
		return
	}
}

// Returns route from local cache
func (c VMIRouterController) getCachedRoute(dbBucket, dbKey string) CachedRoute {
	var cached CachedRoute
	c.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dbBucket))
		cachedBytes := b.Get([]byte(dbKey))
		if len(cachedBytes) == 0 {
			return nil
		}
		if err := json.Unmarshal(cachedBytes, &cached); err != nil {
			log.Error(err, "failed to unmarshal cached information")
		}
		return nil
	})
	return cached
}

// Deletes route from local cache
func (c VMIRouterController) deleteCachedRoute(dbBucket, dbKey string) error {
	return c.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dbBucket))
		cached := c.getCachedRoute(dbBucket, dbKey)
		route, err := c.NewRoute(cached)
		if err != nil {
			return fmt.Errorf("failed to generate route for delete, %v", err)
		}
		if err := c.RouteDel(&route); err != nil && err.Error() != "no such process" {
			return fmt.Errorf("failed to remove route from node, %v", err)
		}
		if err = b.Delete([]byte(dbKey)); err != nil {
			return fmt.Errorf("failed to remove route from cache, %v", err)
		}
		return nil
	})
}

// Adds route into local cache
func (c VMIRouterController) addCachedRoute(dbBucket, dbKey string, cached CachedRoute) error {
	route, err := c.NewRoute(cached)
	if err != nil {
		return fmt.Errorf("failed to generate route for adding, %v", err)
	}
	cachedBytes, err := json.Marshal(cached)
	if err != nil {
		log.Error(err, "failed to marshal cached information")
		return err
	}
	return c.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dbBucket))
		if err := c.RouteAdd(&route); err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to add route to node, %v", err)
		}
		if err := b.Put([]byte(dbKey), cachedBytes); err != nil {
			return fmt.Errorf("failed to add route to cache, %v", err)
		}
		return nil
	})
}

// Runs cleanup for removed VMIs
func (c VMIRouterController) cleanupRemovedVMIs(informer cache.SharedIndexInformer) error {
	vmisToDelete := make(map[string]CachedRoute)
	err := c.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(VMIRoutesBucket))
		cur := b.Cursor()
		var cached CachedRoute
		for key, val := cur.First(); key != nil; key, val = cur.Next() {
			if _, ok, _ := informer.GetIndexer().GetByKey(string(key)); !ok {
				if len(val) == 0 {
					return nil
				}
				if err := json.Unmarshal(val, &cached); err != nil {
					log.Error(err, "failed to unmarshal cached information")
				}
				vmisToDelete[string(key)] = cached
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for key, cached := range vmisToDelete {
		log.Info(fmt.Sprintf("deleting route for %s (%s) via %s (%s)", key, cached.IP, cached.NodeName, cached.NodeIP))
		if err := c.deleteCachedRoute(VMIRoutesBucket, key); err != nil {
			return err
		}
	}
	return nil
}

// Update routes for CIDRs
func (c VMIRouterController) syncCIDRRoutes() error {
	var cidrsToDelete []string
	// removing CIDR routes
	err := c.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(CIDRRoutesBucket))
		b.ForEach(func(key, val []byte) error {
			shouldDelete := true
			for _, cidr := range c.CIDRs {
				if c.RouteLocal {
					if string(key) == cidr.String() {
						shouldDelete = false
						break
					}
				}
			}
			if shouldDelete {
				cidrsToDelete = append(cidrsToDelete, string(key))
			}
			return nil
		})
		return nil
	})
	if err != nil {
		return err
	}

	for _, key := range cidrsToDelete {
		log.Info(fmt.Sprintf("deleting route for cidr %s", key))
		if err := c.deleteCachedRoute(CIDRRoutesBucket, string(key)); err != nil {
			return err
		}
	}

	if c.RouteLocal {
		// adding new CIDR routes
		for _, cidr := range c.CIDRs {
			route := CachedRoute{
				IP: cidr.String(),
			}
			cached := c.getCachedRoute(CIDRRoutesBucket, cidr.String())
			if cached.Equal(&route) {
				err = c.addCachedRoute(CIDRRoutesBucket, cidr.String(), route)
				if err != nil && !os.IsExist(err) {
					return fmt.Errorf("failed to add route %v", err)
				}
				log.Info(fmt.Sprintf("loaded route for cidr %s", cidr.String()))
			} else {
				log.Info(fmt.Sprintf("adding route for cidr %s", cidr.String()))
				if err := c.addCachedRoute(CIDRRoutesBucket, cidr.String(), route); err != nil && !os.IsExist(err) {
					return fmt.Errorf("failed to add route %v", err)
				}
			}
		}

		// removing VM routes
		vmisToDelete := make(map[string]CachedRoute)
		err = c.DB.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(VMIRoutesBucket))
			var cached CachedRoute
			b.ForEach(func(key, val []byte) error {
				if err := json.Unmarshal(val, &cached); err != nil {
					log.Error(err, "failed to unmarshal cached information")
				}
				vmisToDelete[string(key)] = cached
				return nil
			})
			return nil
		})
		if err != nil {
			return err
		}

		for key, cached := range vmisToDelete {
			log.Info(fmt.Sprintf("deleting route for %s (%s) via %s (%s)", key, cached.IP, cached.NodeName, cached.NodeIP))
			if err := c.deleteCachedRoute(VMIRoutesBucket, key); err != nil {
				return err
			}
		}
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

func getNodeInternalIPAddress(node *v1.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			return address.Address
		}
	}
	return ""
}

func (c VMIRouterController) NewRoute(r CachedRoute) (netlink.Route, error) {
	var route netlink.Route
	var err error
	_, route.Dst, err = net.ParseCIDR(appendNetmask(r.IP))
	if err != nil {
		return route, err
	}
	if route.Dst == nil {
		return route, fmt.Errorf("Invalid vmi address %s\n" + r.IP)
	}

	//route.Scope = netlink.SCOPE_UNIVERSE

	if c.RouteLocal || c.NodeName == r.NodeName {
		// fmt.Printf("ip route add %s/32 dev cilium_host\n", vmiIP)
		route.LinkIndex = c.HostIfaceIndex
		if err != nil {
			return route, err
		}
	} else {
		// fmt.Printf("ip route add %s/32 via %s\n", vmiIP, nodeIP)
		route.Gw = net.ParseIP(r.NodeIP)
		if route.Gw == nil {
			return route, fmt.Errorf("Invalid node address %s\n" + r.NodeIP)
		}
	}
	return route, nil
}

func (c VMIRouterController) ipIsManaged(ip net.IP) bool {
	for _, cidr := range c.CIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func (c VMIRouterController) deleteCachedRouteIfExists(dbBucket, dbKey string, cached *CachedRoute) {
	if cached.IP != "" {
		log.Info(fmt.Sprintf("deleting route for %s (%s) via %s (%s)", dbKey, cached.IP, cached.NodeName, cached.NodeIP))
		if err := c.deleteCachedRoute(dbBucket, dbKey); err != nil {
			log.Error(err, "failed to delete route")
			return
		}
	}
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
