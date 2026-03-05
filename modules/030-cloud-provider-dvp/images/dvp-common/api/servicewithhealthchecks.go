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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

func (lb *LoadBalancerService) CreateOrUpdateServiceWithHealthchecks(
	ctx context.Context,
	loadBalancer LoadBalancer,
) (*corev1.Service, error) {
	u, err := lb.getServiceWithHealthchecksByName(ctx, loadBalancer.Name)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return lb.updateServiceWithHealthchecks(ctx, u, loadBalancer)
	}
	return lb.createServiceWithHealthchecks(ctx, loadBalancer)
}

func (lb *LoadBalancerService) DeleteServiceWithHealthchecksByName(ctx context.Context, name string) error {
	u := newServiceWithHealthchecksUnstructured(lb.namespace, name)
	if err := lb.client.Get(ctx, types.NamespacedName{Name: name, Namespace: lb.namespace}, u); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := lb.client.Delete(ctx, u); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func (lb *LoadBalancerService) getServiceWithHealthchecksByName(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	u := newServiceWithHealthchecksUnstructured(lb.namespace, name)
	if err := lb.client.Get(ctx, types.NamespacedName{Name: name, Namespace: lb.namespace}, u); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (lb *LoadBalancerService) updateServiceWithHealthchecks(
	ctx context.Context,
	u *unstructured.Unstructured,
	loadBalancer LoadBalancer,
) (*corev1.Service, error) {
	name := loadBalancer.Name
	service := loadBalancer.Service
	serviceLabels := loadBalancer.ServiceLabels

	lbKey := lbLabelKey(name)
	klog.InfoS("updateLoadBalancerService: start", "lbName", name, "lbKey", lbKey)
	nodes := loadBalancer.Nodes
	if filtredNpdes, err := lb.filterHealthyNodes(ctx, service, loadBalancer.Nodes); err == nil {
		nodes = filtredNpdes
		klog.InfoS("updateLoadBalancerService: filtered nodes", "lbName", name, "filteredNodes", nodes)
	}
	if err := lb.ensureNodeLabels(ctx, nodes, lbKey); err != nil {
		klog.Errorf("Failed to ensure node labels for LoadBalancer %q in namespace %q: %v", name, lb.namespace, err)
		return nil, err
	}

	ports := lb.CreateLoadBalancerPorts(service)

	u.SetAnnotations(service.Annotations)
	u.SetLabels(serviceLabels)

	setServiceWithHealthchecksSpec(
		u,
		ports,
		service.Spec.ExternalTrafficPolicy,
		map[string]string{lbKey: "loadbalancer"},
		service.Spec.ExternalIPs,
		service.Spec.LoadBalancerClass,
		service.Spec.LoadBalancerIP,
	)

	if err := lb.client.Update(ctx, u); err != nil {
		return nil, err
	}

	if err := lb.pollSWHCChildService(ctx, name); err != nil {
		return nil, err
	}

	return lb.GetLoadBalancerByName(ctx, name)
}

func (lb *LoadBalancerService) createServiceWithHealthchecks(
	ctx context.Context,
	loadBalancer LoadBalancer,
) (*corev1.Service, error) {
	name := loadBalancer.Name
	service := loadBalancer.Service
	serviceLabels := loadBalancer.ServiceLabels

	lbKey := lbLabelKey(name)
	klog.InfoS("createLoadBalancerService: start", "lbName", name, "lbKey", lbKey)
	nodes := loadBalancer.Nodes
	if filtredNpdes, err := lb.filterHealthyNodes(ctx, service, loadBalancer.Nodes); err == nil {
		nodes = filtredNpdes
		klog.InfoS("updateLoadBalancerService: filtered nodes", "lbName", name, "filteredNodes", nodes)
	}
	if err := lb.ensureNodeLabels(ctx, nodes, lbKey); err != nil {
		klog.Errorf("Failed to ensure node labels for LoadBalancer %q in namespace %q: %v", name, lb.namespace, err)
		return nil, err
	}

	ports := lb.CreateLoadBalancerPorts(service)

	u := newServiceWithHealthchecksUnstructured(lb.namespace, name)
	u.SetAnnotations(service.Annotations)
	u.SetLabels(serviceLabels)

	setServiceWithHealthchecksSpec(
		u,
		ports,
		service.Spec.ExternalTrafficPolicy,
		map[string]string{lbKey: "loadbalancer"},
		service.Spec.ExternalIPs,
		service.Spec.LoadBalancerClass,
		service.Spec.LoadBalancerIP,
	)

	if err := lb.client.Create(ctx, u); err != nil {
		return nil, err
	}

	if err := lb.pollSWHCChildService(ctx, name); err != nil {
		return nil, err
	}

	return lb.GetLoadBalancerByName(ctx, name)
}

func (lb *LoadBalancerService) pollSWHCChildService(ctx context.Context, name string) error {
	return wait.PollUntilContextTimeout(ctx,
		LBCreationPollInterval,
		LBCreationPollTimeout,
		true,
		func(context.Context) (bool, error) {
			s, err := lb.GetLoadBalancerByName(ctx, name)
			if err != nil {
				return false, err
			}
			if s != nil && len(s.Status.LoadBalancer.Ingress) > 0 {
				return true, nil
			}
			return false, nil
		})
}
