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

package probe

import (
	"time"

	v1 "k8s.io/api/core/v1"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/monitor/node"
	"d8.io/upmeter/pkg/probe/checker"
)

func initNodeGroups(access kubernetes.Access, nodeGroupNames, knownZones []string, nodeLister node.Lister) []runnerConfig {
	const (
		groupNodeGroups = "nodegroups"
		cpTimeout       = 5 * time.Second
	)

	configs := []runnerConfig{}

	for _, ngName := range nodeGroupNames {
		configs = append(configs,
			nodeGroupChecker(access, nodeLister, groupNodeGroups, cpTimeout, ngName, knownZones),
		)
	}
	return configs
}

func nodeGroupChecker(access kubernetes.Access, nodeLister node.Lister, group string, cpTimeout time.Duration, ngName string, zones []string) runnerConfig {
	ngLister := &nodeGroupLister{
		name:     ngName,
		allNodes: nodeLister,
	}

	return runnerConfig{
		group:  group,
		probe:  ngName,
		check:  "nodes",
		period: 10 * time.Second,
		config: checker.NodegroupHasDesiredAmountOfNodes{
			Access:     access,
			NodeLister: ngLister,

			Name:       ngName,
			KnownZones: zones,

			RequestTimeout: cpTimeout,

			ControlPlaneAccessTimeout: cpTimeout,
		},
	}
}

type nodeGroupLister struct {
	name     string
	allNodes node.Lister
}

func (ng *nodeGroupLister) List() ([]*v1.Node, error) {
	nodes, err := ng.allNodes.List()
	if err != nil {
		return nil, err
	}
	ret := make([]*v1.Node, 0)
	for _, theNode := range nodes {
		ngName, ok := theNode.GetLabels()["node.deckhouse.io/group"]
		if !ok {
			continue
		}
		if ngName != ng.name {
			continue
		}
		ret = append(ret, theNode)
	}
	return ret, nil
}
