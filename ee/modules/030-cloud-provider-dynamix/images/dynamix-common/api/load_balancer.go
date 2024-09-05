/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/klog/v2"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/lb"
)

type LoadBalancerService struct {
	*Service
	externalNetworkService *ExternalNetworkService
	internalNetworkService *InternalNetworkService
}

func NewLoadBalancerService(
	service *Service,
	externalNetworkService *ExternalNetworkService,
	internalNetworkService *InternalNetworkService,
) *LoadBalancerService {
	return &LoadBalancerService{
		Service:                service,
		externalNetworkService: externalNetworkService,
		internalNetworkService: internalNetworkService,
	}
}

func (l *LoadBalancerService) CreateOrUpdateLoadBalancer(ctx context.Context, loadBalancer LoadBalancer) (string, error) {
	externalNetworkID, err := l.getExternalNetworkID(ctx, loadBalancer.ExternalNetworkName)
	if err != nil {
		return "", fmt.Errorf("failed to get external network ID: %v", err)
	}

	klog.V(4).Infof("External network ID: %d", externalNetworkID)

	internalNetworkID, err := l.getInternalNetworkID(ctx, loadBalancer.InternalNetworkName)
	if err != nil {
		return "", fmt.Errorf("failed to get internal network ID: %v", err)
	}

	klog.V(4).Infof("Internal network ID: %d", internalNetworkID)

	loadBalancerInfo, err := l.getOrCreateLoadBalancer(ctx, loadBalancer.Name, externalNetworkID, internalNetworkID)
	if err != nil {
		return "", fmt.Errorf("failed to get or create load balancer: %v", err)
	}

	klog.V(4).Infof("Load balancer ID: %d", loadBalancerInfo.ID)

	addedRules, removedRules := diffForwardingRules(loadBalancer.ForwardingRules, loadBalancerInfo.Frontends, loadBalancerInfo.PrimaryNode.FrontendIP)

	addedRulesJSON, err := json.Marshal(addedRules)
	if err != nil {
		return "", fmt.Errorf("failed to marshal new forwarding rules: %v", err)
	}

	klog.V(4).Infof("Added forwarding rules: %v, removed forwarding rules: %v", string(addedRulesJSON), removedRules)

	err = l.addForwardingRules(ctx, addedRules, loadBalancerInfo, loadBalancerInfo.PrimaryNode.FrontendIP)
	if err != nil {
		return "", fmt.Errorf("failed to add forwarding rules: %v", err)
	}

	err = l.removeForwardingRules(ctx, removedRules, loadBalancerInfo.ID)
	if err != nil {
		return "", fmt.Errorf("failed to remove forwarding rules: %v", err)
	}

	return loadBalancerInfo.PrimaryNode.FrontendIP, nil
}

func (l *LoadBalancerService) getExternalNetworkID(ctx context.Context, externalNetworkName string) (uint64, error) {
	externalNetworks, err := l.externalNetworkService.GetExternalNetworks(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get external networks: %v", err)
	}

	if len(externalNetworks) == 0 {
		return 0, fmt.Errorf("external networks not found")
	}

	if len(externalNetworks) > 1 && externalNetworkName == "" {
		return 0, fmt.Errorf("multiple external networks found and 'dynamix.cpi.flant.com/external-network-name' annotation is not set")
	}

	if externalNetworkName != "" {
		for _, network := range externalNetworks {
			if network.Name == externalNetworkName {
				return network.ID, nil
			}
		}

		return 0, fmt.Errorf("external network with name '%s' not found", externalNetworkName)
	}

	return externalNetworks[0].ID, nil
}

func (l *LoadBalancerService) getInternalNetworkID(ctx context.Context, internalNetworkName string) (uint64, error) {
	internalNetworks, err := l.internalNetworkService.GetInternalNetworks(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get internal networks: %v", err)
	}

	if len(internalNetworks) == 0 {
		return 0, fmt.Errorf("internal networks not found")
	}

	if len(internalNetworks) > 1 && internalNetworkName == "" {
		return 0, fmt.Errorf("multiple internal networks found and 'dynamix.cpi.flant.com/internal-network-name' annotation is not set")
	}

	if internalNetworkName != "" {
		for _, network := range internalNetworks {
			if network.Name == internalNetworkName {
				return network.ID, nil
			}
		}

		return 0, fmt.Errorf("internal network with name '%s' not found", internalNetworkName)
	}

	return internalNetworks[0].ID, nil
}

func (l *LoadBalancerService) getOrCreateLoadBalancer(
	ctx context.Context,
	loadBalancerName string,
	externalNetworkID uint64,
	internalNetworkID uint64,
) (*lb.RecordLB, error) {
	var loadBalancerInfo *lb.RecordLB

	err := l.retryer.Do(ctx, func() (bool, error) {
		loadBalancers, err := l.client.CloudAPI().LB().List(ctx, lb.ListRequest{
			Name: loadBalancerName,
		})
		if err != nil {
			return false, fmt.Errorf("failed to list load balancers: %v", err)
		}
		if len(loadBalancers.Data) > 0 {
			loadBalancerInfo = &loadBalancers.Data[0].RecordLB

			return false, nil
		}

		_, err = l.client.CloudAPI().LB().Create(ctx, lb.CreateRequest{
			RGID:     l.resourceGroupID,
			Name:     loadBalancerName,
			ExtNetID: externalNetworkID,
			VINSID:   internalNetworkID,
			Start:    true,
		})
		if err != nil {
			return false, fmt.Errorf("failed to create load balancer: %v", err)
		}

		loadBalancers, err = l.client.CloudAPI().LB().List(ctx, lb.ListRequest{
			Name: loadBalancerName,
		})
		if err != nil {
			return false, fmt.Errorf("failed to list load balancers: %v", err)
		}
		if len(loadBalancers.Data) > 0 {
			loadBalancerInfo = &loadBalancers.Data[0].RecordLB

			return false, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return loadBalancerInfo, nil
}

func (l *LoadBalancerService) createBackend(ctx context.Context, loadBalancer *lb.RecordLB, name string) (bool, *lb.ItemBackend, error) {
	for _, backend := range loadBalancer.Backends {
		if backend.Name == name {
			return true, &backend, nil
		}
	}

	err := l.retryer.Do(ctx, func() (bool, error) {
		_, err := l.client.CloudAPI().LB().BackendCreate(ctx, lb.BackendCreateRequest{
			LBID:        loadBalancer.ID,
			BackendName: name,
		})
		if err != nil {
			return false, err
		}

		return false, nil
	})
	if err != nil {
		return false, nil, err
	}

	return false, nil, nil
}

func (l *LoadBalancerService) createFrontend(ctx context.Context, loadBalancer *lb.RecordLB, name string, backendName string) (bool, *lb.ItemFrontend, error) {
	for _, frontend := range loadBalancer.Frontends {
		if frontend.Name == name {
			return true, &frontend, nil
		}
	}

	err := l.retryer.Do(ctx, func() (bool, error) {
		_, err := l.client.CloudAPI().LB().FrontendCreate(ctx, lb.FrontendCreateRequest{
			LBID:         loadBalancer.ID,
			FrontendName: name,
			BackendName:  backendName,
		})
		if err != nil {
			return false, err
		}

		return false, nil
	})
	if err != nil {
		return false, nil, err
	}

	return false, nil, nil
}

func (l *LoadBalancerService) createFrontendBind(
	ctx context.Context,
	loadBalancer *lb.RecordLB,
	frontendName string,
	bindingName string,
	entryIP string,
	entryPort uint64,
) error {
	err := l.retryer.Do(ctx, func() (bool, error) {
		_, err := l.client.CloudAPI().LB().FrontendBind(ctx, lb.FrontendBindRequest{
			LBID:           loadBalancer.ID,
			FrontendName:   frontendName,
			BindingName:    bindingName,
			BindingAddress: entryIP,
			BindingPort:    entryPort,
		})
		if err != nil {
			return false, err
		}

		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

type frontendIndexKey struct {
	Address string
	Port    uint64
}

func diffForwardingRules(
	rules []ForwardingRule,
	frontends []lb.ItemFrontend,
	entryIP string,
) (added map[int32]*ForwardingRule, removed []uint64) {
	added = make(map[int32]*ForwardingRule)

	frontendsIndex := make(map[frontendIndexKey]struct{}, len(frontends))

	for _, frontend := range frontends {
		if len(frontend.Bindings) == 0 {
			continue
		}

		frontendsIndex[frontendIndexKey{
			Address: frontend.Bindings[0].Address,
			Port:    frontend.Bindings[0].Port,
		}] = struct{}{}
	}

	for i, rule := range rules {
		if _, ok := frontendsIndex[frontendIndexKey{
			Address: entryIP,
			Port:    uint64(rule.EntryPort),
		}]; !ok {
			added[rule.EntryPort] = &rules[i]
		}
	}

	rulesIndex := make(map[frontendIndexKey]int32, len(rules))

	for _, rule := range rules {
		rulesIndex[frontendIndexKey{
			Address: entryIP,
			Port:    uint64(rule.EntryPort),
		}] = rule.EntryPort
	}

	for _, frontend := range frontends {
		if _, ok := rulesIndex[frontendIndexKey{
			Address: frontend.Bindings[0].Address,
			Port:    frontend.Bindings[0].Port,
		}]; !ok {
			removed = append(removed, frontend.Bindings[0].Port)
		}
	}

	return
}

func (l *LoadBalancerService) addForwardingRules(
	ctx context.Context,
	rules map[int32]*ForwardingRule,
	loadBalancer *lb.RecordLB,
	entryIP string,
) error {
	for _, rule := range rules {
		frontendName := fmt.Sprintf("frontend-%d", rule.EntryPort)
		backendName := fmt.Sprintf("backend-%d", rule.EntryPort)
		bindingName := fmt.Sprintf("binding-%d", rule.EntryPort)

		var backendServers []lb.ItemServer

		ok, backend, err := l.createBackend(ctx, loadBalancer, backendName)
		if err != nil {
			return fmt.Errorf("failed to create backend: %v", err)
		}
		if ok {
			backendServers = backend.Servers
		}

		addedTargets, removedTargets := diffForwardingRuleTargets(rule.Targets, backendServers)

		addedTargetsJSON, err := json.Marshal(addedTargets)
		if err != nil {
			return fmt.Errorf("failed to marshal new forwarding rule targets: %v", err)
		}

		klog.V(4).Infof("Added forwarding rule targets: %v, removed forwarding rule targets: %v, port %d", string(addedTargetsJSON), removedTargets, rule.EntryPort)

		err = l.addForwardingRuleTargets(ctx, addedTargets, loadBalancer.ID, backendName)
		if err != nil {
			return fmt.Errorf("failed to add forwarding rule targets: %v", err)
		}

		err = l.removeForwardingRuleTargets(ctx, removedTargets, loadBalancer.ID, backendName)
		if err != nil {
			return fmt.Errorf("failed to remove forwarding rule targets: %v", err)
		}

		var frontendBindings []lb.ItemBinding

		ok, frontend, err := l.createFrontend(ctx, loadBalancer, frontendName, backendName)
		if err != nil {
			return fmt.Errorf("failed to create frontend: %v", err)
		}
		if ok {
			frontendBindings = frontend.Bindings
		}

		if len(frontendBindings) == 0 {
			err = l.createFrontendBind(ctx, loadBalancer, frontendName, bindingName, entryIP, uint64(rule.EntryPort))
			if err != nil {
				return fmt.Errorf("failed to create frontend bind: %v", err)
			}
		}
	}

	return nil
}

func (l *LoadBalancerService) removeForwardingRules(
	ctx context.Context,
	entryPorts []uint64,
	loadBalancerID uint64,
) error {
	for _, entryPort := range entryPorts {
		frontendName := fmt.Sprintf("frontend-%d", entryPort)
		backendName := fmt.Sprintf("backend-%d", entryPort)
		bindingName := fmt.Sprintf("binding-%d", entryPort)

		err := l.retryer.Do(ctx, func() (bool, error) {
			_, err := l.client.CloudAPI().LB().FrontendBindDelete(ctx, lb.FrontendBindDeleteRequest{
				LBID:         loadBalancerID,
				FrontendName: frontendName,
				BindingName:  bindingName,
			})
			if err != nil {
				return false, err
			}

			return false, nil
		})
		if err != nil {
			return err
		}

		err = l.retryer.Do(ctx, func() (bool, error) {
			_, err := l.client.CloudAPI().LB().FrontendDelete(ctx, lb.FrontendDeleteRequest{
				LBID:         loadBalancerID,
				FrontendName: frontendName,
			})
			if err != nil {
				return false, err
			}

			return false, nil
		})
		if err != nil {
			return err
		}

		err = l.retryer.Do(ctx, func() (bool, error) {
			_, err := l.client.CloudAPI().LB().BackendDelete(ctx, lb.BackendDeleteRequest{
				LBID:        loadBalancerID,
				BackendName: backendName,
			})
			if err != nil {
				return false, err
			}

			return false, nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

type backendServerIndexKey struct {
	ServerName string
	Address    string
	Port       uint64
}

func diffForwardingRuleTargets(
	targets []ForwardingRuleTarget,
	servers []lb.ItemServer,
) (added []ForwardingRuleTarget, removed []string) {
	serversIndex := make(map[backendServerIndexKey]struct{}, len(servers))

	for _, server := range servers {
		serversIndex[backendServerIndexKey{
			Address: server.Address,
			Port:    server.Port,
		}] = struct{}{}
	}

	for _, target := range targets {
		if _, ok := serversIndex[backendServerIndexKey{
			ServerName: target.Name,
			Address:    target.Address,
			Port:       uint64(target.Port),
		}]; !ok {
			added = append(added, target)
		}
	}

	targetsIndex := make(map[backendServerIndexKey]struct{})

	for _, target := range targets {
		targetsIndex[backendServerIndexKey{
			ServerName: target.Name,
			Address:    target.Address,
			Port:       uint64(target.Port),
		}] = struct{}{}
	}

	for _, server := range servers {
		if _, ok := targetsIndex[backendServerIndexKey{
			ServerName: server.Name,
			Address:    server.Address,
			Port:       server.Port,
		}]; !ok {
			removed = append(removed, server.Name)
		}
	}

	return
}

func (l *LoadBalancerService) addForwardingRuleTargets(
	ctx context.Context,
	targets []ForwardingRuleTarget,
	loadBalancerID uint64,
	backendName string,
) error {
	for _, target := range targets {
		err := l.retryer.Do(ctx, func() (bool, error) {
			_, err := l.client.CloudAPI().LB().BackendServerAdd(ctx, lb.BackendServerAddRequest{
				LBID:        loadBalancerID,
				BackendName: backendName,
				ServerName:  target.Name,
				Address:     target.Address,
				Port:        uint64(target.Port),
			})
			if err != nil {
				return false, err
			}

			return false, nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *LoadBalancerService) removeForwardingRuleTargets(
	ctx context.Context,
	targets []string,
	loadBalancerID uint64,
	backendName string,
) error {
	for _, name := range targets {
		err := l.retryer.Do(ctx, func() (bool, error) {
			_, err := l.client.CloudAPI().LB().BackendServerDelete(ctx, lb.BackendServerDeleteRequest{
				LBID:        loadBalancerID,
				BackendName: backendName,
				ServerName:  name,
			})
			if err != nil {
				return false, err
			}

			return false, nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *LoadBalancerService) GetLoadBalancerByName(ctx context.Context, name string) (*lb.RecordLB, error) {
	var loadBalancer *lb.RecordLB

	err := l.retryer.Do(ctx, func() (bool, error) {
		loadBalancers, err := l.client.CloudAPI().LB().List(ctx, lb.ListRequest{
			Name: name,
		})
		if err != nil {
			return false, err
		}

		if len(loadBalancers.Data) == 0 {
			return true, ErrNotFound
		}

		loadBalancer = &loadBalancers.Data[0].RecordLB

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return loadBalancer, nil
}

func (l *LoadBalancerService) RemoveLoadBalancerByName(ctx context.Context, name string) error {
	loadBalancer, err := l.GetLoadBalancerByName(ctx, name)
	if err != nil {
		return err
	}

	err = l.retryer.Do(ctx, func() (bool, error) {
		_, err := l.client.CloudAPI().LB().Delete(ctx, lb.DeleteRequest{
			LBID:        loadBalancer.ID,
			Permanently: true,
		})
		if err != nil {
			return false, err
		}

		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

type LoadBalancer struct {
	Name                string
	ExternalNetworkName string
	InternalNetworkName string
	ForwardingRules     []ForwardingRule
}

type ForwardingRule struct {
	EntryPort int32
	Targets   []ForwardingRuleTarget
}

type ForwardingRuleTarget struct {
	Name    string
	Address string
	Port    int32
}
