/*
Copyright 2026 Flant JSC

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

package cluster

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"

	"update-observer/pkg/version"
)

type ControlPlaneState struct {
	DesiredCount           int
	UpToDateCount          int
	DesiredComponentCount  int
	UpToDateComponentCount int
	Phase                  ControlPlanePhase
	MasterNodes            map[string]*MasterNode
	versions               *version.UniqueAggregator
}

type ControlPlanePhase string

const (
	ControlPlaneUpToDate     ControlPlanePhase = "UpToDate"
	ControlPlaneUpdating     ControlPlanePhase = "Updating"
	ControlPlaneInconsistent ControlPlanePhase = "Inconsistent"
	ControlPlaneVersionDrift ControlPlanePhase = "VersionDrift"
)

func GetControlPlaneState(controlPlanePods *corev1.PodList, desiredVersion string) (*ControlPlaneState, error) {
	masterNodes, err := buildControlPlaneTopology(controlPlanePods, desiredVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get components state: %w", err)
	}

	res := &ControlPlaneState{
		DesiredCount: len(masterNodes),
		MasterNodes:  masterNodes,
		versions:     version.NewUniqueAggregator(semver.Sort),
	}

	res.aggregateNodesState()

	return res, nil
}

func (s *ControlPlaneState) aggregateNodesState() {
	var desiredCount, upToDateCount, desiredComponentsCount, upToDateComponentsCount int
	var phase ControlPlanePhase

	for _, masterNode := range s.MasterNodes {
		var failedComponents, updatingComponents int
		var descriptions []string

		desiredCount++
		for componentName, component := range masterNode.Components {
			desiredComponentsCount++
			s.versions.Set(component.Version)

			switch component.State {
			case ControlPlaneComponentFailed:
				failedComponents++
				descriptions = append(descriptions, fmt.Sprintf("%s: %s", componentName, component.Description))
			case ControlPlaneComponentUpdating:
				updatingComponents++
			case ControlPlaneComponentUpToDate:
				upToDateComponentsCount++
			}
		}

		switch {
		case failedComponents > 0:
			masterNode.Phase = MasterNodeFailed
			slices.Sort(descriptions)
			masterNode.Description = strings.Join(descriptions, ", ")
		case updatingComponents > 0:
			masterNode.Phase = MasterNodeUpdating
		default:
			masterNode.Phase = MasterNodeUpToDate
			upToDateCount++
		}
	}

	switch {
	case upToDateCount < desiredCount:
		phase = ControlPlaneUpdating
	case upToDateCount > desiredCount:
		phase = ControlPlaneInconsistent
	default:
		phase = ControlPlaneUpToDate
	}

	s.DesiredCount = desiredCount
	s.UpToDateCount = upToDateCount
	s.DesiredComponentCount = desiredComponentsCount
	s.UpToDateComponentCount = upToDateComponentsCount
	s.Phase = phase
}
