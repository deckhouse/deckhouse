/*
Copyright 2022 Flant JSC

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

package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kube "github.com/flant/kube-client/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ngv1 "d8.io/upmeter/internal/nodegroups/v1"
	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/monitor/node"
)

// NodegroupHasDesiredAmountOfNodes is a checker constructor and configurator
type NodegroupHasDesiredAmountOfNodes struct {
	Access     kubernetes.Access
	NodeLister node.Lister

	// Name of nodegroup
	Name string
	// Known availability zones in the cloud
	KnownZones []string

	// RequestTimeout is common for api operations
	RequestTimeout time.Duration

	// PreflightChecker verifies preconditions before running the check
	PreflightChecker check.Checker
}

func (c NodegroupHasDesiredAmountOfNodes) Checker() check.Checker {
	ngFetcher := &nodeGroupFetcher{
		access:  c.Access,
		timeout: c.RequestTimeout,
	}

	ngChecker := &nodesByNodegroupCountChecker{
		nodeGroupGetter: ngFetcher,
		nodeLister:      c.NodeLister,
		name:            c.Name,
		requestTimeout:  c.RequestTimeout,
		zones:           c.KnownZones,
	}

	return sequence(
		c.PreflightChecker,
		withTimeout(ngChecker, c.RequestTimeout),
	)
}

// nodesByNodegroupCountChecker checks that nodes number satisfies nodegroup spec
type nodesByNodegroupCountChecker struct {
	nodeGroupGetter *nodeGroupFetcher
	nodeLister      node.Lister

	name  string
	zones []string

	requestTimeout time.Duration
}

func (c *nodesByNodegroupCountChecker) Check() check.Error {
	ng, err := c.nodeGroupGetter.Get(c.name)
	if err != nil {
		return check.ErrUnknown("getting nodegroup: %v", err)
	}

	minReadyExpected := ng.minPerZone - ng.maxUnavailablePerZone
	if minReadyExpected <= 0 {
		return check.ErrUnknown("nodegroup is allowed to be unavailable")
	}

	nodes, err := c.nodeLister.List()
	if err != nil {
		return check.ErrUnknown("listing nodes: %v", err)
	}

	healthyByZone, err := countHealthyNodesByZone(nodes)
	if err != nil {
		return check.ErrUnknown("counting nodes by zone: %v", err)
	}

	zones := ng.zones
	if len(zones) == 0 {
		// Not specified in nodegroup, hence all zones should be checked
		zones = c.zones
	}

	for _, zone := range zones {
		healthy, ok := healthyByZone[zone]
		if !ok {
			return check.ErrFail("no healthy nodes in zone %q", zone)
		}

		if healthy < minReadyExpected {
			return check.ErrFail("expected at least %d healthy, but got %d in zone %q",
				minReadyExpected, healthy, zone)
		}
	}

	return nil
}

type nodeGroupFetcher struct {
	access  kubernetes.Access
	timeout time.Duration
}

type nodeGroupProps struct {
	minPerZone            int32
	maxUnavailablePerZone int32
	zones                 []string
}

var nodeGroupGVR = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "nodegroups",
}

func (f *nodeGroupFetcher) Get(name string) (nodeGroupProps, error) {
	var props nodeGroupProps

	rawNG, err := f.access.Kubernetes().Dynamic().Resource(nodeGroupGVR).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return props, err
	}

	var ng ngv1.NodeGroup
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawNG.UnstructuredContent(), &ng); err != nil {
		return props, err
	}

	props.minPerZone = *ng.Spec.CloudInstances.MinPerZone
	if ng.Spec.CloudInstances.MaxUnavailablePerZone != nil {
		// MaxUnavailablePerZone is zero by default
		props.maxUnavailablePerZone = *ng.Spec.CloudInstances.MaxUnavailablePerZone
	}

	props.zones = ng.Spec.CloudInstances.Zones
	if ng.Spec.CloudInstances.ClassReference.Kind == "AzureInstanceClass" {
		// NodeGroup in Azure can contain zone notation like "1", "2", "3", whereas in nodes,
		// topology label contain full zone name like "westeurope-1", "westeurope-2",
		// "westeurope-3". We have to account that.
		location, err := fetchAzureLocation(f.access.Kubernetes())
		if err != nil {
			return props, err
		}
		for i := range props.zones {
			props.zones[i] = fmt.Sprintf("%s-%s", location, props.zones[i])
		}
	}

	return props, nil
}

// countHealthyNodesByZone returns a map of zone -> healthy nodes count
func countHealthyNodesByZone(nodes []*v1.Node) (map[string]int32, error) {
	byZone := map[string]int32{}

	for _, node := range nodes {
		zone, ok := node.GetLabels()["topology.kubernetes.io/zone"]
		if !ok || zone == "" {
			return nil, fmt.Errorf("node %s has no zone", node.GetName())
		}

		// Instant status is what we seek to catch up with nodegroup desired state
		if isNodeReadyLongEnough(node, 0) {
			byZone[zone]++
		}
	}

	return byZone, nil
}

func fetchAzureLocation(klient kube.Client) (string, error) {
	cloudProviderSettings, err := klient.CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-node-manager-cloud-provider", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	azureValues, ok := cloudProviderSettings.Data["azure"]
	if !ok {
		return "", fmt.Errorf("azure cloud provider settings not found")
	}
	var azureSettings map[string]string
	if err := json.Unmarshal(azureValues, &azureSettings); err != nil {
		return "", fmt.Errorf("failed to unmarshal azure cloud provider settings: %v", err)
	}
	location, ok := azureSettings["location"]
	if !ok {
		return "", fmt.Errorf("azure cloud provider settings does not contain location")
	}
	return string(location), nil
}
