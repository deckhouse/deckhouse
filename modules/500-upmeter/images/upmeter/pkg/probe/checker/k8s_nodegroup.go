/*
Copyright 2021 Flant JSC

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

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
	minPerZone            int64
	maxUnavailablePerZone int64
	zones                 []string
}

func (f *nodeGroupFetcher) GetNodeGroup(name string) (nodegroupProps, error) {
	nodegroup, err := f.access.Kubernetes().Dynamic().Resource(schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: "nodegroups",
	}).Get(name, metav1.GetOptions{})
	if err != nil {
		return nodegroupProps{}, err
	}

	return f.parseProps(nodegroup)
}

func (f *nodeGroupFetcher) parseProps(nodegroup *unstructured.Unstructured) (nodegroupProps, error) {
	/*
		  NodeGroup
			spec:
			  cloudInstances:
				zones: []string
				minPerZone: int64
				maxPerZone: int64
				maxUnavailablePerZone: int64
				maxSurgePerZone: int64
	*/

	var (
		props nodegroupProps
		err   error

		name = nodegroup.GetName()
	)

	props.minPerZone, err = f.parseNodegroupInstanceLimit(nodegroup, "minPerZone")
	if err != nil {
		return props, fmt.Errorf("parse minPerZone in nodegroup %q: %v", name, err)
	}

	props.maxUnavailablePerZone, err = f.parseNodegroupInstanceLimit(nodegroup, "maxUnavailablePerZone")
	if err != nil {
		return props, fmt.Errorf("parse maxUnavailablePerZone in nodegroup %q: %v", name, err)
	}

	props.zones, err = f.parseZones(nodegroup)
	if err != nil {
		return props, fmt.Errorf("parse zones in nodegroup %q: %v", name, err)
	}

	return props, nil
}

func (f *nodeGroupFetcher) parseZones(ng *unstructured.Unstructured) ([]string, error) {
	value, _, err := unstructured.NestedStringSlice(ng.UnstructuredContent(), "spec", "cloudInstances", "zones")
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (f *nodeGroupFetcher) parseNodegroupInstanceLimit(ng *unstructured.Unstructured, field string) (int64, error) {
	value, ok, err := unstructured.NestedInt64(ng.UnstructuredContent(), "spec", "cloudInstances", field)
	if err != nil {
		return 0, err
	}
	if !ok {
		value = 0
	}
	return value, nil
}

func (f *nodeGroupFetcher) CountHealthyNodesByZone(nodeGroup string) (map[string]int64, error) {
	timeoutSeconds := int64(f.timeout.Seconds())
	nodeList, err := f.access.Kubernetes().CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector:  "node.deckhouse.io/group=" + nodeGroup,
		TimeoutSeconds: &timeoutSeconds,
	})
	if err != nil {
		return nil, err
	}

	byZone := map[string]int64{}

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
