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
	selectorValue := getNodesSelectorLabels(lbKey)

	if err := lb.ensureNodeLabels(ctx, loadBalancer.Nodes, lbKey); err != nil {
		klog.Errorf("Failed to ensure node labels for LoadBalancer %q in namespace %q: %v", name, lb.namespace, err)
		return nil, err
	}

	ports := lb.CreateLoadBalancerPorts(service)

	svc.Spec.Selector = map[string]string{}
	if svc.Spec.Selector == nil {
		svc.Spec.Selector = map[string]string{}
	}
	svc.Spec.Selector[lbKey] = selectorValue[lbKey]
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
	selectorValue := getNodesSelectorLabels(lbKey)

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
			Selector: map[string]string{
				lbKey: selectorValue[lbKey],
			},
		},
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

func (lb *LoadBalancerService) DeleteLoadBalancerByName(ctx context.Context, name string) error {
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

func getNodesSelectorLabels(lbName string) map[string]string {
	return map[string]string{lbLabelKey(lbName): "loadbalancer"}
}

func (lb *LoadBalancerService) ensureNodeLabels(
	ctx context.Context,
	nodes []*corev1.Node,
	lbKey string,
) error {
	desired := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		desired[node.Name] = struct{}{}
		if node.Labels == nil {
			node.Labels = make(map[string]string)
		}
		if node.Labels[lbKey] != "loadbalancer" {
			labledNode := node.DeepCopy()
			labledNode.Labels[lbKey] = "loadbalancer"
			if err := lb.client.Update(ctx, labledNode); err != nil {
				return err
			}
		}
	}

	var list corev1.NodeList
	if err := lb.client.List(ctx, &list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			lbKey: "loadbalancer",
		}),
	}); err != nil {
		klog.Errorf("Failed to list nodes: %v", err)
		return err
	}

	for _, node := range list.Items {
		if _, ok := desired[node.Name]; ok {
			continue
		}
		unlabledNode := node.DeepCopy()
		delete(unlabledNode.Labels, lbKey)
		if err := lb.client.Update(ctx, unlabledNode); err != nil {
			return err
		}
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
