/*
Copyright 2023 Flant JSC

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
	"fmt"
	"time"

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
	// Zone prefix that can be omitted zone names in nodegroups
	ZonePrefix string

	// RequestTimeout is common for api operations
	RequestTimeout time.Duration

	// PreflightChecker verifies preconditions before running the check
	PreflightChecker check.Checker
}

func (c NodegroupHasDesiredAmountOfNodes) Checker() check.Checker {
	ngFetcher := &nodeGroupFetcher{
		access:     c.Access,
		timeout:    c.RequestTimeout,
		zonePrefix: c.ZonePrefix,
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

	// For Azure, nodegroup contain partial zone names which are used as parameters in Azure
	// tooling. To compare zones in nodes and nodegroups, we need to add this prefix to the zone
	// names taken from nodegroup. Basically we are fixing nodegroup content which has lost the
	// region information.
	zonePrefix string
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
	if f.zonePrefix != "" {
		// Fix for Azure, see the field description
		for i, zone := range props.zones {
			props.zones[i] = fmt.Sprintf("%s-%s", f.zonePrefix, zone)
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
