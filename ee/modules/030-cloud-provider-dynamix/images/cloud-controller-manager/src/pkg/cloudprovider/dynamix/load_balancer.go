/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package dynamix

import (
	"context"
	"fmt"
	"log"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"dynamix-common/api"
)

const (
	externalNetworkNameAnnotation = "dynamix.cpi.flant.com/external-network-name"
	internalNetworkNameAnnotation = "dynamix.cpi.flant.com/internal-network-name"
)

// GetLoadBalancer is an implementation of LoadBalancer.GetLoadBalancer
func (c *Cloud) GetLoadBalancer(ctx context.Context, _ string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	loadBalancerName := defaultLoadBalancerName(service)
	log.Printf("Retrieving LB by name %q", loadBalancerName)

	loadBalancer, err := c.dynamixService.LoadBalancerService.GetLoadBalancerByName(ctx, loadBalancerName)
	if err != nil {
		return &v1.LoadBalancerStatus{}, false, err
	}
	if loadBalancer == nil {
		return &v1.LoadBalancerStatus{}, false, nil
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: loadBalancer.PrimaryNode.FrontendIP,
			},
		},
	}, true, nil
}

// GetLoadBalancerName is an implementation of LoadBalancer.GetLoadBalancerName.
func (c *Cloud) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	return defaultLoadBalancerName(service)
}

// EnsureLoadBalancer is an implementation of LoadBalancer.EnsureLoadBalancer.
func (c *Cloud) EnsureLoadBalancer(ctx context.Context, _ string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	return c.ensureLB(ctx, service, nodes)
}

// UpdateLoadBalancer is an implementation of LoadBalancer.UpdateLoadBalancer.
func (c *Cloud) UpdateLoadBalancer(ctx context.Context, _ string, service *v1.Service, nodes []*v1.Node) error {
	_, err := c.ensureLB(ctx, service, nodes)
	return err
}

// EnsureLoadBalancerDeleted is an implementation of LoadBalancer.EnsureLoadBalancerDeleted.
func (c *Cloud) EnsureLoadBalancerDeleted(ctx context.Context, _ string, service *v1.Service) error {
	lbName := defaultLoadBalancerName(service)

	err := c.dynamixService.LoadBalancerService.RemoveLoadBalancerByName(ctx, lbName)
	if err != nil {
		return err
	}

	return nil
}

func (c *Cloud) ensureLB(ctx context.Context, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no Nodes provided")
	}

	lbName := defaultLoadBalancerName(service)
	lbParams := c.getLoadBalancerParameters(service)

	targets := make([]api.ForwardingRuleTarget, 0)

	for _, node := range nodes {
		vm, err := c.getVMByNodeName(ctx, types.NodeName(node.Name))
		if err != nil {
			return nil, err
		}

		_, localIP, err := c.dynamixService.ComputeService.GetVMIPAddresses(vm)
		if err != nil {
			return nil, err
		}

		for _, address := range localIP {
			target := api.ForwardingRuleTarget{
				Name:    node.Name,
				Address: address,
			}

			targets = append(targets, target)
		}
	}

	loadBalancer := api.LoadBalancer{
		Name:                lbName,
		ExternalNetworkName: lbParams.externalNetworkName,
		InternalNetworkName: lbParams.internalNetworkName,
	}

	for _, svcPort := range service.Spec.Ports {
		if svcPort.Protocol != v1.ProtocolTCP {
			return nil, fmt.Errorf("only TCP protocol is supported")
		}

		forwardingRule := api.ForwardingRule{
			EntryPort: svcPort.Port,
			Targets:   make([]api.ForwardingRuleTarget, len(targets)),
		}

		copy(forwardingRule.Targets, targets)

		for i := range targets {
			forwardingRule.Targets[i].Port = svcPort.NodePort
		}

		loadBalancer.ForwardingRules = append(loadBalancer.ForwardingRules, forwardingRule)
	}

	externalIP, err := c.dynamixService.LoadBalancerService.CreateOrUpdateLoadBalancer(ctx, loadBalancer)
	if err != nil {
		return nil, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: externalIP,
			},
		},
	}, nil
}

func defaultLoadBalancerName(service *v1.Service) string {
	name := "a" + string(service.UID)

	name = strings.Replace(name, "-", "", -1)

	if len(name) > 32 {
		name = name[:32]
	}

	return name
}

type loadBalancerParameters struct {
	externalNetworkName string
	internalNetworkName string
}

func (c *Cloud) getLoadBalancerParameters(service *v1.Service) (params loadBalancerParameters) {
	if value, ok := service.ObjectMeta.Annotations[externalNetworkNameAnnotation]; ok {
		params.externalNetworkName = value
	}

	if value, ok := service.ObjectMeta.Annotations[internalNetworkNameAnnotation]; ok {
		params.internalNetworkName = value
	}

	return
}
