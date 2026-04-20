/*
Copyright 2025 Flant JSC

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

// This file contains the main reconciliation logic for DVP load balancers.
// It decides whether a regular Kubernetes Service of type LoadBalancer
// or a ServiceWithHealthchecks resource should be created/updated,
// filters healthy backend nodes, and manages VM labels used to select
// load balancer targets.
package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
)

func (lb *LoadBalancerService) CreateOrUpdateLoadBalancer(
	ctx context.Context,
	loadBalancer LoadBalancer,
) (*corev1.Service, error) {
	svc, err := lb.GetLoadBalancerByName(ctx, loadBalancer.Name)
	if err != nil {
		return nil, err
	}
	if svc != nil {
		if isOwnedBySWHC(svc) {
			u, err := lb.getServiceWithHealthchecksByName(ctx, loadBalancer.Name)
			if err != nil {
				return nil, err
			}
			if u != nil {
				return lb.updateServiceWithHealthchecks(ctx, u, loadBalancer)
			}
		}
		return lb.updateLoadBalancerService(ctx, svc, loadBalancer)
	}

	if lb.shouldUseSWHC(ctx) {
		svc, err := lb.CreateOrUpdateServiceWithHealthchecks(ctx, loadBalancer)
		if err == nil {
			return svc, nil
		}
		if !isSWHCUnsupportedErr(err) {
			return nil, err
		}
	}

	return lb.createLoadBalancerService(ctx, loadBalancer)
}

func isOwnedBySWHC(svc *corev1.Service) bool {
	for _, ref := range svc.OwnerReferences {
		if ref.Kind == "ServiceWithHealthchecks" {
			return true
		}
	}
	return false
}

func (lb *LoadBalancerService) DeleteLoadBalancerByName(ctx context.Context, name string) (retErr error) {
	defer func() {
		if err := lb.removeVMLabelsByKey(ctx, lbLabelKey(name)); err != nil {
			klog.Errorf("Failed to remove node labels for LoadBalancer %q in namespace %q: %v", name, lb.namespace, err)
			if retErr == nil {
				retErr = err
			}
		}
	}()

	svc, err := lb.GetLoadBalancerByName(ctx, name)
	if err != nil {
		klog.Errorf("Failed to get LoadBalancer service %q in namespace %q: %v", name, lb.namespace, err)
		return err
	}
	if svc != nil {
		if err = lb.client.Delete(ctx, svc); err != nil && !k8serrors.IsNotFound(err) {
			klog.Errorf("Failed to delete LoadBalancer service %q in namespace %q: %v", name, lb.namespace, err)
			return err
		}
	}

	if err := lb.DeleteServiceWithHealthchecksByName(ctx, name); err != nil {
		return err
	}

	return nil
}

func (lb *LoadBalancerService) filterHealthyNodes(ctx context.Context, svc *corev1.Service, nodes []*corev1.Node) ([]*corev1.Node, error) { // nolint:unparam
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeLocal ||
		svc.Spec.HealthCheckNodePort == 0 {
		return nodes, nil
	}

	cs := &ComputeService{Service: lb.Service}
	client := &http.Client{Timeout: 3 * time.Second}

	type result struct {
		node    *corev1.Node
		healthy bool
	}

	results := make([]result, len(nodes))
	var wg sync.WaitGroup

	for i, n := range nodes {
		wg.Add(1)
		go func(i int, n *corev1.Node) {
			defer wg.Done()
			vm, err := cs.GetVMByHostname(ctx, n.Name)
			if err != nil {
				return
			}
			ips, _, err := cs.GetVMIPAddresses(vm)
			if err != nil || len(ips) == 0 {
				return
			}

			url := "http://" + net.JoinHostPort(ips[0], strconv.Itoa(int(svc.Spec.HealthCheckNodePort))) + "/healthz"
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			resp, err := client.Do(req)
			if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
				_ = resp.Body.Close()
				results[i] = result{node: n, healthy: true}
				return
			}
			if resp != nil {
				_ = resp.Body.Close()
			}
		}(i, n)
	}

	wg.Wait()

	healthy := make([]*corev1.Node, 0, len(nodes))
	for _, r := range results {
		if r.healthy {
			healthy = append(healthy, r.node)
		}
	}
	return healthy, nil
}

func (lb *LoadBalancerService) removeVMLabelsByKey(ctx context.Context, lbKey string) error {
	var vmList v1alpha2.VirtualMachineList
	if err := lb.client.List(ctx, &vmList, &client.ListOptions{
		Namespace:     lb.namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{lbKey: "loadbalancer"}),
	}); err != nil {
		return err
	}

	cs := &ComputeService{Service: lb.Service}
	for i := range vmList.Items {
		vm := &vmList.Items[i]
		hostname := vm.Labels[DVPVMHostnameLabel]
		if hostname == "" {
			continue
		}
		if err := cs.RemoveVMLabelByHostname(ctx, hostname, lbKey); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (lb *LoadBalancerService) ensureNodeLabels(
	ctx context.Context,
	nodes []*corev1.Node,
	lbKey string,
) error {
	desired := make(map[string]struct{}, len(nodes))
	hostnames := make([]string, 0, len(nodes))
	for _, node := range nodes {
		desired[node.Name] = struct{}{}
		hostnames = append(hostnames, node.Name)
	}

	cs := &ComputeService{Service: lb.Service}

	for _, h := range hostnames {
		if err := cs.EnsureVMLabelByHostname(ctx, h, lbKey, "loadbalancer"); err != nil {
			return fmt.Errorf("ensure VM label for hostname %q: %w", h, err)
		}
		klog.V(2).InfoS("ensureNodeLabels: set VM label", "hostname", h, "lbKey", lbKey)
	}

	var vml v1alpha2.VirtualMachineList
	if err := lb.client.List(ctx, &vml, &client.ListOptions{
		Namespace:     lb.namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{lbKey: "loadbalancer"}),
	}); err != nil {
		return fmt.Errorf("list VMs by %s=loadbalancer: %w", lbKey, err)
	}

	for i := range vml.Items {
		vm := &vml.Items[i]
		hostname := vm.Labels[DVPVMHostnameLabel]
		if hostname == "" {
			continue
		}
		if _, ok := desired[hostname]; ok {
			continue
		}
		if err := cs.RemoveVMLabelByHostname(ctx, hostname, lbKey); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("remove VM label for hostname %q: %w", hostname, err)
		}
		klog.V(2).InfoS("ensureNodeLabels: removed stale VM label", "hostname", hostname, "lbKey", lbKey)
	}

	return nil
}

func lbLabelKey(lbName string) string {
	prettified := strings.ToLower(strings.ReplaceAll(lbName, "-", ""))
	max := 63 - len(DVPLoadBalancerLabelPrefix)
	if len(prettified) > max {
		prettified = prettified[:max]
	}
	return DVPLoadBalancerLabelPrefix + prettified
}
