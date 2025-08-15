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
	"strings"
	"time"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DeckhouseNetworkLoadBalancerClassType = "network.deckhouse.io/load-balancer-class"
	LBCreationPollInterval                = 5 * time.Second
	LBCreationPollTimeout                 = 5 * time.Minute
)

type LoadBalancerService struct {
	*Service
}

type LoadBalancer struct {
	Name          string
	Service       *corev1.Service
	Nodes         []*corev1.Node
	ServiceLabels map[string]string
}

func NewLoadBalancerService(service *Service) *LoadBalancerService {
	return &LoadBalancerService{service}
}

func (lb *LoadBalancerService) GetLoadBalancerByName(ctx context.Context, name string) (*corev1.Service, error) {
	var svc corev1.Service
	if err := lb.client.Get(ctx, types.NamespacedName{Name: name, Namespace: lb.namespace}, &svc); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &svc, nil
}

func (lb *LoadBalancerService) CreateLoadBalancerPorts(service *corev1.Service) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, len(service.Spec.Ports))
	for i, port := range service.Spec.Ports {
		ports[i].Name = port.Name
		ports[i].Protocol = port.Protocol
		ports[i].Port = port.Port
		ports[i].TargetPort = intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: port.NodePort,
		}
	}
	return ports
}

func (lb *LoadBalancerService) UpdateLoadBalancerPorts(ctx context.Context, service *corev1.Service, ports []corev1.ServicePort) error {
	if service == nil {
		return fmt.Errorf("service is nil")
	}
	if !equality.Semantic.DeepEqual(ports, service.Spec.Ports) {
		service.Spec.Ports = ports
		if err := lb.client.Update(ctx, service); err != nil {
			klog.Errorf("Failed to update LoadBalancer service %q in namespace %q: %v", service.GetName(), lb.namespace, err)
			return err
		}
		return nil
	}
	return nil
}

func (lb *LoadBalancerService) CreateOrUpdateLoadBalancer(
	ctx context.Context,
	loadBalancer LoadBalancer,
) (*corev1.Service, error) {
	var svc *corev1.Service
	svc, err := lb.GetLoadBalancerByName(ctx, loadBalancer.Name)
	if svc != nil && err == nil {
		return lb.updateLoadBalancerService(ctx, svc, loadBalancer)
	}

	return lb.createLoadBalancerService(ctx, loadBalancer)
}

func (lb *LoadBalancerService) updateLoadBalancerService(
	ctx context.Context,
	svc *corev1.Service,
	loadBalancer LoadBalancer,
) (*corev1.Service, error) {
	name := loadBalancer.Name
	service := loadBalancer.Service
	serviceLabels := loadBalancer.ServiceLabels

	lbKey := lbLabelKey(name)
	// DEBUG
	klog.InfoS("updateLoadBalancerService: start", "lbName", name, "lbKey", lbKey)
	if err := lb.ensureNodeLabels(ctx, loadBalancer.Nodes, lbKey); err != nil {
		klog.Errorf("Failed to ensure node labels for LoadBalancer %q in namespace %q: %v", name, lb.namespace, err)
		return nil, err
	}

	ports := lb.CreateLoadBalancerPorts(service)
	// DEBUG
	oldSel := svc.Spec.Selector
	svc.Spec.Selector = map[string]string{lbKey: "loadbalancer"}
	klog.InfoS("updateLoadBalancerService: set external selector",
		"lbName", name, "oldSelector", oldSel, "newSelector", svc.Spec.Selector)
	svc.Labels = map[string]string{}
	svc.Spec.Ports = []corev1.ServicePort{}
	svc.Spec.ExternalIPs = []string{}
	svc.Spec.LoadBalancerClass = nil
	svc.Spec.LoadBalancerIP = ""
	svc.Spec.HealthCheckNodePort = 0

	if len(serviceLabels) > 0 {
		svc.Labels = serviceLabels
	}
	if len(ports) > 0 {
		svc.Spec.Ports = ports
	}
	if len(service.Spec.ExternalIPs) > 0 {
		svc.Spec.ExternalIPs = service.Spec.ExternalIPs
	}
	if service.Spec.LoadBalancerClass != nil {
		svc.Spec.LoadBalancerClass = ptr.To(*service.Spec.LoadBalancerClass)
	}
	if service.Spec.LoadBalancerIP != "" {
		svc.Spec.LoadBalancerIP = service.Spec.LoadBalancerIP
	}
	if service.Spec.HealthCheckNodePort > 0 {
		svc.Spec.HealthCheckNodePort = service.Spec.HealthCheckNodePort
	}

	err := lb.client.Update(ctx, svc)
	if err != nil {
		return nil, err
	}

	err = lb.pollLoadBalancer(ctx, name, svc)
	if err != nil {
		klog.Errorf("Failed to poll LoadBalancer service %q in namespace %q: %v", name, lb.namespace, err)
		return nil, err
	}

	svc, err = lb.GetLoadBalancerByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return svc, nil
}

func (lb *LoadBalancerService) createLoadBalancerService(
	ctx context.Context,
	loadBalancer LoadBalancer,
) (*corev1.Service, error) {
	name := loadBalancer.Name
	service := loadBalancer.Service
	serviceLabels := loadBalancer.ServiceLabels
	lbKey := lbLabelKey(name)
	// DEBUG
	klog.InfoS("createLoadBalancerService: start", "lbName", name, "lbKey", lbKey)
	if err := lb.ensureNodeLabels(ctx, loadBalancer.Nodes, lbKey); err != nil {
		klog.Errorf("Failed to ensure node labels for LoadBalancer %q in namespace %q: %v", name, lb.namespace, err)
		return nil, err
	}

	ports := lb.CreateLoadBalancerPorts(service)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   lb.namespace,
			Annotations: service.Annotations,
			Labels:      serviceLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports:                 ports,
			Type:                  corev1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: service.Spec.ExternalTrafficPolicy,
			Selector:              map[string]string{lbKey: "loadbalancer"},
		},
	}
	// DEBUG
	klog.InfoS("createLoadBalancerService: creating Service",
		"lbName", name, "selector", svc.Spec.Selector)
	if len(service.Spec.ExternalIPs) > 0 {
		svc.Spec.ExternalIPs = service.Spec.ExternalIPs
	}
	if service.Spec.LoadBalancerClass != nil {
		svc.Spec.LoadBalancerClass = ptr.To(*service.Spec.LoadBalancerClass)
	}
	if service.Spec.LoadBalancerIP != "" {
		svc.Spec.LoadBalancerIP = service.Spec.LoadBalancerIP
	}
	if service.Spec.HealthCheckNodePort > 0 {
		svc.Spec.HealthCheckNodePort = service.Spec.HealthCheckNodePort
	}

	err := lb.client.Create(ctx, svc)
	if err != nil {
		return nil, err
	}

	err = lb.pollLoadBalancer(ctx, name, svc)
	if err != nil {
		klog.Errorf("Failed to poll LoadBalancer service %q in namespace %q: %v", name, lb.namespace, err)
		return nil, err
	}

	svc, err = lb.GetLoadBalancerByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return svc, nil
}

func (lb *LoadBalancerService) pollLoadBalancer(ctx context.Context, name string, svc *corev1.Service) error {
	return wait.PollUntilContextTimeout(ctx,
		LBCreationPollInterval,
		LBCreationPollTimeout,
		true,
		func(context.Context) (done bool, err error) {
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				return true, nil
			}
			s, err := lb.GetLoadBalancerByName(ctx, name)
			if err != nil {
				klog.Errorf("Failed to get LoadBalancer service %q in namespace %q: %v", name, lb.namespace, err)
				return false, err
			}
			if s != nil && len(s.Status.LoadBalancer.Ingress) > 0 {
				svc = s
				return true, nil
			}
			return false, nil
		})
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
	if svc == nil {
		return nil
	}
	if err = lb.client.Delete(ctx, svc); err != nil {
		klog.Errorf("Failed to delete LoadBalancer service %q in namespace %q: %v", name, lb.namespace, err)
		return err
	}
	return nil
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
	for _, in := range nodes {
		desired[in.Name] = struct{}{}
		hostnames = append(hostnames, in.Name)
	}
	// DEBUG
	klog.InfoS("ensureNodeLabels: input",
		"lbKey", lbKey,
		"nodesCount", len(nodes),
		"nodes", hostnames)

	cs := &ComputeService{Service: lb.Service}

	for _, h := range hostnames {
		if err := cs.EnsureVMLabelByHostname(ctx, h, lbKey, "loadbalancer"); err != nil {
			return fmt.Errorf("ensure VM label for hostname %q: %w", h, err)
		}
		// DEBUG
		klog.V(2).InfoS("ensureNodeLabels: set VM label OK", "hostname", h, "lbKey", lbKey)
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
		// DEBUG
		klog.V(2).InfoS("ensureNodeLabels: removed VM label", "hostname", hostname, "lbKey", lbKey)
	}

	return nil
}

func lbLabelKey(lbName string) string {
	prittyfied := strings.ToLower(strings.ReplaceAll(lbName, "-", ""))
	max := 63 - len(DVPLoadBalancerLabelPrefix)
	if len(prittyfied) > max {
		prittyfied = prittyfied[:max]
	}
	return DVPLoadBalancerLabelPrefix + prittyfied
}
