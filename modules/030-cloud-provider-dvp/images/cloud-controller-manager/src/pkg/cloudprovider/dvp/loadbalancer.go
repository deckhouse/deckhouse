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

// nolint:gci
package dvp

import (
	"context"
	"fmt"
	"strings"

	"dvp-common/api"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
)

func (c *Cloud) GetLoadBalancer(
	ctx context.Context,
	clusterName string,
	service *corev1.Service,
) (*corev1.LoadBalancerStatus, bool, error) {
	name := defaultLoadBalancerName(service)
	svc, err := c.dvpService.LoadBalancerService.GetLoadBalancerByName(ctx, name)
	if err != nil {
		klog.Errorf("Failed to get LoadBalancer service %q in namespace %q: %v", name, c.config.Namespace, err)
		return nil, false, err
	}
	if svc == nil {
		return nil, false, nil
	}
	return &svc.Status.LoadBalancer, true, nil
}

func (c *Cloud) GetLoadBalancerName(
	_ context.Context,
	_ string,
	service *corev1.Service,
) string {
	return defaultLoadBalancerName(service)
}

func (c *Cloud) EnsureLoadBalancer(
	ctx context.Context,
	clusterName string,
	service *corev1.Service,
	nodes []*corev1.Node,
) (*corev1.LoadBalancerStatus, error) {
	return c.ensureLB(ctx, service, nodes)
}

func (c *Cloud) UpdateLoadBalancer(
	ctx context.Context,
	clusterName string,
	service *corev1.Service,
	nodes []*corev1.Node,
) error {
	_, err := c.ensureLB(ctx, service, nodes)
	return err
}

func (c *Cloud) EnsureLoadBalancerDeleted(
	ctx context.Context,
	clusterName string,
	service *corev1.Service,
) error {
	name := defaultLoadBalancerName(service)

	return c.dvpService.LoadBalancerService.DeleteLoadBalancerByName(ctx, name)
}

func defaultLoadBalancerName(service *corev1.Service) string {
	name := "a" + string(service.UID)

	name = strings.ReplaceAll(name, "-", "")

	if len(name) > 32 {
		name = name[:32]
	}

	return name
}

func (c *Cloud) ensureLB(ctx context.Context, service *corev1.Service, nodes []*corev1.Node) (*corev1.LoadBalancerStatus, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no Nodes provided")
	}
	if service.Spec.AllocateLoadBalancerNodePorts != nil && !*service.Spec.AllocateLoadBalancerNodePorts {
		return nil, fmt.Errorf("DVP cloud provider does not support Service %q with allocateLoadBalancerNodePorts=false: external load balancers route traffic to source Service NodePorts", service.Name)
	}

	lbName := defaultLoadBalancerName(service)

	lb := api.LoadBalancer{
		Name:    lbName,
		Service: service,
		Nodes:   nodes,
		ServiceLabels: map[string]string{
			"deckhouse.io/managed-by":       "deckhouse",
			"dvp.deckhouse.io/cluster-uuid": c.config.ClusterUUID,
		},
	}

	svc, err := c.dvpService.LoadBalancerService.CreateOrUpdateLoadBalancer(ctx, lb)
	if err != nil {
		klog.Errorf("Failed to CreateOrUpdateLoadBalancer %q in namespace %q: %v", lbName, c.config.Namespace, err)
		return nil, err
	}
	if err := c.ensureLoadBalancerNetworkPolicy(ctx, loadBalancerNetworkPolicyName(service), lbName, service, svc); err != nil {
		return nil, err
	}
	return &svc.Status.LoadBalancer, nil
}

func loadBalancerNetworkPolicyName(service *corev1.Service) string {
	return "lb-" + string(service.UID)
}

// ensureLoadBalancerNetworkPolicy permits traffic from the parent-cluster load
// balancer to the NodePorts of VMs selected for this load balancer. The
// selector is unique per load balancer, including when nodes are shared.
func (c *Cloud) ensureLoadBalancerNetworkPolicy(
	ctx context.Context,
	policyName string,
	loadBalancerName string,
	service *corev1.Service,
	owner *corev1.Service,
) error {
	if c.config.NetworkPolicy != "Isolated" {
		return nil
	}

	ports := make([]networkingv1.NetworkPolicyPort, 0, len(service.Spec.Ports))
	for _, servicePort := range service.Spec.Ports {
		if servicePort.NodePort == 0 {
			continue
		}
		protocol := servicePort.Protocol
		nodePort := intstr.FromInt32(servicePort.NodePort)
		ports = append(ports, networkingv1.NetworkPolicyPort{Protocol: &protocol, Port: &nodePort})
	}
	if len(ports) == 0 {
		return nil
	}

	policy := &networkingv1.NetworkPolicy{}
	key := types.NamespacedName{Namespace: c.config.Namespace, Name: policyName}
	err := c.dvpService.Service.GetClient().Get(ctx, key, policy)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("get NetworkPolicy %q for LoadBalancer %q: %w", policyName, loadBalancerName, err)
	}

	desiredSpec := networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{api.LoadBalancerLabelKey(loadBalancerName): "loadbalancer"}},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		Ingress:     []networkingv1.NetworkPolicyIngressRule{{Ports: ports}},
	}
	ownerRef := *metav1.NewControllerRef(owner, corev1.SchemeGroupVersion.WithKind("Service"))
	if k8serrors.IsNotFound(err) {
		policy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:            policyName,
				Namespace:       c.config.Namespace,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
				Labels: map[string]string{
					"deckhouse.io/managed-by":       "deckhouse",
					"dvp.deckhouse.io/cluster-uuid": c.config.ClusterUUID,
				},
			},
			Spec: desiredSpec,
		}
		if err := c.dvpService.Service.GetClient().Create(ctx, policy); err != nil {
			return fmt.Errorf("create NetworkPolicy %q for LoadBalancer %q: %w", policyName, loadBalancerName, err)
		}
		return nil
	}

	if policy.Labels == nil {
		policy.Labels = make(map[string]string, 2)
	}
	policy.Labels["deckhouse.io/managed-by"] = "deckhouse"
	policy.Labels["dvp.deckhouse.io/cluster-uuid"] = c.config.ClusterUUID
	policy.OwnerReferences = []metav1.OwnerReference{ownerRef}
	policy.Spec = desiredSpec
	if err := c.dvpService.Service.GetClient().Update(ctx, policy); err != nil {
		return fmt.Errorf("update NetworkPolicy %q for LoadBalancer %q: %w", policyName, loadBalancerName, err)
	}
	return nil
}
