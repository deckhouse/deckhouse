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

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func initNodegroups(access kubernetes.Access, nodeGroupNames, knownZones []string) []runnerConfig {
	const (
		groupNodegroups = "nodegroups"
		cpTimeout       = 5 * time.Second
	)

	configs := []runnerConfig{}

	for _, ngName := range nodeGroupNames {
		configs = append(configs,
			nodeGroupChecker(access, groupNodegroups, cpTimeout, ngName, knownZones),
		)
	}
	return configs
}

func nodeGroupChecker(access kubernetes.Access, groupNodegroups string, cpTimeout time.Duration, nodeGroupName string, zones []string) runnerConfig {
	return runnerConfig{
		group:  groupNodegroups,
		probe:  nodeGroupName,
		check:  "nodes",
		period: 10 * time.Second,
		config: checker.NodegroupHasDesiredAmountOfNodes{
			Access:     access,
			Name:       nodeGroupName,
			KnownZones: zones,

			RequestTimeout: cpTimeout,

			ControlPlaneAccessTimeout: cpTimeout,
		},
	}
}
