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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	LBCreationPollInterval = 5 * time.Second
	LBCreationPollTimeout  = 5 * time.Minute
)

func (c *Cloud) GetLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service) (status *corev1.LoadBalancerStatus, exists bool, err error) {
	name := c.GetLoadBalancerName(ctx, clusterName, service)
	svc, err := c.dvpService.LoadBalancerService.GetLoadBalancerByName(ctx, name)
	if err != nil {
		klog.Errorf("Failed to get LoadBalancer service %q in namespace %q: %v", name, c.config.Namespace, err)
		return nil, false, err
	}
	if svc == nil {
		return nil, false, nil
	}
	status = &svc.Status.LoadBalancer
	return status, true, nil
}

func (c *Cloud) GetLoadBalancerName(_ context.Context, clusterName string, service *corev1.Service) string {
	// TODO: replace DefaultLoadBalancerName to generate more meaningful loadbalancer names.
	return fmt.Sprintf("%s-%s", clusterName, cloudprovider.DefaultLoadBalancerName(service))
}

func (c *Cloud) EnsureLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) (*corev1.LoadBalancerStatus, error) {
	name := c.GetLoadBalancerName(ctx, clusterName, service)
	svc, err := c.dvpService.LoadBalancerService.GetLoadBalancerByName(ctx, name)
	klog.Infof("EnsureLoadBalancer: %v+", svc)
	klog.Infof("EnsureLoadBalancer nodes: %v+", nodes)
	if err != nil {
		klog.Errorf("Failed to get LoadBalancer service %q in namespace %q: %v", name, c.config.Namespace, err)
		return nil, err
	}
	ports := c.dvpService.LoadBalancerService.CreateLoadBalancerPorts(service)

	if svc != nil {
		return &svc.Status.LoadBalancer, c.dvpService.LoadBalancerService.UpdateLoadBalancerPorts(ctx, svc, ports)
	}

	// TODO: fix labels.
	vmLabels := map[string]string{
		"cluster.x-k8s.io/cluster-name": clusterName,
	}
	// TODO: fix labels.
	svcLabels := map[string]string{
		"cluster.x-k8s.io/tenant-service-name":      service.Name,
		"cluster.x-k8s.io/tenant-service-namespace": service.Namespace,
		"cluster.x-k8s.io/cluster-name":             clusterName,
	}

	// for k, v := range lb.infraLabels {
	// 	svcLabels[k] = v
	// }

	svc, err = c.dvpService.LoadBalancerService.CreateLoadBalancer(
		ctx,
		name,
		service,
		vmLabels,
		true,
		svcLabels,
		ports,
	)
	if err != nil {
		klog.Errorf("Failed to create LoadBalancer service %q in namespace %q: %v", name, c.config.Namespace, err)
		return nil, err
	}

	err = wait.PollUntilContextTimeout(ctx,
		LBCreationPollInterval,
		LBCreationPollTimeout,
		true,
		func(context.Context) (done bool, err error) {
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				return true, nil
			}
			s, err := c.dvpService.LoadBalancerService.GetLoadBalancerByName(ctx, name)
			if err != nil {
				klog.Errorf("Failed to get LoadBalancer service %q in namespace %q: %v", name, c.config.Namespace, err)
				return false, err
			}
			if s != nil && len(s.Status.LoadBalancer.Ingress) > 0 {
				svc = s
				return true, nil
			}
			return false, nil
		})
	if err != nil {
		klog.Errorf("Failed to poll LoadBalancer service %q in namespace %q: %v", name, c.config.Namespace, err)
		return nil, err
	}

	return &svc.Status.LoadBalancer, nil
}

func (c *Cloud) UpdateLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, _ []*corev1.Node) error {
	name := c.GetLoadBalancerName(ctx, clusterName, service)

	svc, err := c.dvpService.LoadBalancerService.GetLoadBalancerByName(ctx, name)
	if err != nil {
		klog.Errorf("Failed to get LoadBalancer service %q in namespace %q: %v", name, c.config.Namespace, err)
		return err
	}

	ports := c.dvpService.LoadBalancerService.CreateLoadBalancerPorts(service)
	// LoadBalancer already exist, update the ports if changed
	return c.dvpService.LoadBalancerService.UpdateLoadBalancerPorts(ctx, svc, ports)
}

func (c *Cloud) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *corev1.Service) error {
	name := c.GetLoadBalancerName(ctx, clusterName, service)

	return c.dvpService.LoadBalancerService.DeleteLoadBalancerByName(ctx, name)
}
