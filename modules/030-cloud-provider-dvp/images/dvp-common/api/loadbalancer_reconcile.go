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

package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateOrUpdateLoadBalancer creates or updates a LoadBalancer.
// If the cluster supports ServiceWithHealthchecks (SWHC), it is preferred over a plain Service.
// On create: SWHC is tried first when supported; falls back to a plain Service otherwise.
// On update: the existing resource type is preserved — SWHC is updated if it already exists,
// plain Service is updated if it already exists, and SWHC creation is attempted for new resources.
func (lb *LoadBalancerService) CreateOrUpdateLoadBalancer(
	ctx context.Context,
	loadBalancer LoadBalancer,
) (*corev1.Service, error) {
	// Prefer SWHC when supported.
	if lb.shouldUseSWHC(ctx) {
		svc, err := lb.CreateOrUpdateServiceWithHealthchecks(ctx, loadBalancer)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			return svc, nil
		}
	}

	// Fall back to plain Service.
	svc, err := lb.GetLoadBalancerByName(ctx, loadBalancer.Name)
	if err != nil {
		return nil, err
	}
	if svc != nil {
		return lb.updateLoadBalancerService(ctx, svc, loadBalancer)
	}
	return lb.createLoadBalancerService(ctx, loadBalancer)
}

// DeleteLoadBalancerByName deletes the LoadBalancer service (plain or SWHC) and cleans up VM labels.
func (lb *LoadBalancerService) DeleteLoadBalancerByName(ctx context.Context, name string) (retErr error) {
	defer func() {
		if err := lb.removeVMLabelsByKey(ctx, lbLabelKey(name)); err != nil {
			klog.Errorf("Failed to remove node labels for LoadBalancer %q in namespace %q: %v", name, lb.namespace, err)
			if retErr == nil {
				retErr = err
			}
		}
	}()

	// Try to delete the plain Service first.
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

	// Always attempt to delete the SWHC resource as well — it may exist even when
	// no plain child Service is visible (e.g. partial creation).
	if err := lb.DeleteServiceWithHealthchecksByName(ctx, name); err != nil {
		return err
	}

	return nil
}

func (lb *LoadBalancerService) filterHealthyNodes(ctx context.Context, svc *corev1.Service, nodes []*corev1.Node) ([]*corev1.Node, error) { //nolint:unparam
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeLocal ||
		svc.Spec.HealthCheckNodePort == 0 {
		return nodes, nil
	}

	cs := &ComputeService{Service: lb.Service}
	httpClient := &http.Client{Timeout: 3 * time.Second}

	healthy := make([]*corev1.Node, 0, len(nodes))
	for _, n := range nodes {
		vm, err := cs.GetVMByHostname(ctx, n.Name)
		if err != nil {
			continue
		}
		ips, _, err := cs.GetVMIPAddresses(vm)
		if err != nil || len(ips) == 0 {
			continue
		}

		url := "http://" + net.JoinHostPort(ips[0], strconv.Itoa(int(svc.Spec.HealthCheckNodePort))) + "/healthz"
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := httpClient.Do(req)
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			healthy = append(healthy, n)
			continue
		}
		if resp != nil {
			_ = resp.Body.Close()
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
	cs := &ComputeService{Service: lb.Service}

	for _, node := range nodes {
		desired[node.Name] = struct{}{}
		if err := cs.EnsureVMLabelByHostname(ctx, node.Name, lbKey, "loadbalancer"); err != nil {
			return fmt.Errorf("ensure VM label for hostname %q: %w", node.Name, err)
		}
		klog.V(2).InfoS("ensureNodeLabels: set VM label", "hostname", node.Name, "lbKey", lbKey)
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

// lbLabelKey builds a Kubernetes-label-safe key for a LoadBalancer with the given name.
func lbLabelKey(lbName string) string {
	prettified := strings.ToLower(strings.ReplaceAll(lbName, "-", ""))
	max := 63 - len(DVPLoadBalancerLabelPrefix)
	if len(prettified) > max {
		prettified = prettified[:max]
	}
	return DVPLoadBalancerLabelPrefix + prettified
}
