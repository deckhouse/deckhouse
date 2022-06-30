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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ngv1 "d8.io/upmeter/internal/nodegroups/v1"
	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// NodegroupHasDesiredAmountOfNodes is a checker constructor and configurator
type NodegroupHasDesiredAmountOfNodes struct {
	Access kubernetes.Access

	// Name of nodegroup
	Name string

	// Known availability zones in the cloud
	KnownZones []string

	// RequestTimeout is common for api operations
	RequestTimeout time.Duration

	// ControlPlaneAccessTimeout is the timeout to verify apiserver availability
	ControlPlaneAccessTimeout time.Duration
}

func (c NodegroupHasDesiredAmountOfNodes) Checker() check.Checker {
	ngChecker := &nodesByNodegroupCountChecker{
		access:         c.Access,
		name:           c.Name,
		requestTimeout: c.RequestTimeout,
		zones:          c.KnownZones,
	}

	return sequence(
		newControlPlaneChecker(c.Access, c.ControlPlaneAccessTimeout),
		withTimeout(ngChecker, c.RequestTimeout),
	)
}

// nodesByNodegroupCountChecker checks that nodes number satisfies nodegroup spec
type nodesByNodegroupCountChecker struct {
	access         kubernetes.Access
	name           string
	requestTimeout time.Duration
	zones          []string
}

func (c *nodesByNodegroupCountChecker) Check() check.Error {
	fetcher := &nodeGroupFetcher{
		access:  c.access,
		timeout: c.requestTimeout,
	}

	ng, err := fetcher.GetNodeGroup(c.name)
	if err != nil {
		return check.ErrUnknown("getting nodegroup: %v", err)
	}
	minReadyExpected := ng.minPerZone - ng.maxUnavailablePerZone
	if minReadyExpected <= 0 {
		return check.ErrUnknown("nodegroup is allowed to be unavailable")
	}

	healthyByZone, err := fetcher.CountHealthyNodesByZone(c.name)
	if err != nil {
		return check.ErrUnknown("getting nodes: %v", err)
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

type nodegroupProps struct {
	minPerZone            int32
	maxUnavailablePerZone int32
	zones                 []string
}

func (f *nodeGroupFetcher) GetNodeGroup(name string) (nodegroupProps, error) {
	var props nodegroupProps

	rawNG, err := f.access.Kubernetes().Dynamic().Resource(schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: "nodegroups",
	}).Get(name, metav1.GetOptions{})
	if err != nil {
		return props, err
	}

	var ng ngv1.NodeGroup
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawNG.UnstructuredContent(), &ng); err != nil {
		return props, err
	}

	if ng.Spec.CloudInstances.MinPerZone == nil {
		return props, fmt.Errorf("minPerZone is not specified")
	}
	props.minPerZone = *ng.Spec.CloudInstances.MinPerZone

	if ng.Spec.CloudInstances.MaxUnavailablePerZone == nil {
		return props, fmt.Errorf("maxUnavailablePerZone is not specified")
	}
	props.maxUnavailablePerZone = *ng.Spec.CloudInstances.MaxUnavailablePerZone

	props.zones = ng.Spec.CloudInstances.Zones
	return props, nil
}

func (f *nodeGroupFetcher) CountHealthyNodesByZone(nodeGroup string) (map[string]int32, error) {
	timeoutSeconds := int64(f.timeout.Seconds())
	nodeList, err := f.access.Kubernetes().CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector:  "node.deckhouse.io/group=" + nodeGroup,
		TimeoutSeconds: &timeoutSeconds,
	})
	if err != nil {
		return nil, err
	}

	byZone := map[string]int32{}

	for _, node := range nodeList.Items {
		zone, ok := node.GetLabels()["topology.kubernetes.io/zone"]
		if !ok || zone == "" {
			return nil, fmt.Errorf("node %q without zone", node.GetName())
		}

		if isNodeReady(&node) {
			byZone[zone]++
		}
	}

	return byZone, nil
}
