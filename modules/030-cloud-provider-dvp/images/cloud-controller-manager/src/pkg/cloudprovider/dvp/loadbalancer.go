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

package dvp

import (
	"context"
	"dvp-common/api"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func (c *Cloud) GetLoadBalancer(
	ctx context.Context,
	clusterName string,
	service *corev1.Service,
) (status *corev1.LoadBalancerStatus, exists bool, err error) {
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

func defaultLoadBalancerName(service *v1.Service) string {
	name := "a" + string(service.UID)

	name = strings.Replace(name, "-", "", -1)

	if len(name) > 32 {
		name = name[:32]
	}

	return name
}

func (c *Cloud) ensureLB(ctx context.Context, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no Nodes provided")
	}

	lbName := defaultLoadBalancerName(service)

	lb := api.LoadBalancer{
		Name:    lbName,
		Service: service,
		Nodes:   nodes,
	}

	svc, err := c.dvpService.LoadBalancerService.CreateOrUpdateLoadBalancer(ctx, lb)
	if err != nil {
		klog.Errorf("Failed to CreateOrUpdateLoadBalancer %q in namespace %q: %v", lbName, c.config.Namespace, err)
		return nil, err
	}
	return &svc.Status.LoadBalancer, nil
}
