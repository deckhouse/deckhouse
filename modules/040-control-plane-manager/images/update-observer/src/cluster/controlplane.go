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
	"update-observer/pkg/version"

	corev1 "k8s.io/api/core/v1"
)

type ControlPlaneState struct {
	DesiredCount           int
	UpToDateCount          int
	DesiredComponentCount  int
	UpToDateComponentCount int
	Phase                  ControlPlanePhase
	NodesState             map[string]*MasterNodeState
}

type ControlPlanePhase string

const (
	ControlPlaneUpToDate     ControlPlanePhase = "UpToDate"
	ControlPlaneUpdating     ControlPlanePhase = "Updating"
	ControlPlaneInconsistent ControlPlanePhase = "Inconsistent"
	ControlPlaneVersionDrift ControlPlanePhase = "VersionDrift"
)

func GetControlPlaneState(controlPlanePods *corev1.PodList, desiredVersion string) (*ControlPlaneState, error) {
	componentsByNode, err := getComponentsStateByNode(controlPlanePods)
	if err != nil {
		return nil, fmt.Errorf("failed to get components state: %w", err)
	}

	res := &ControlPlaneState{
		DesiredCount: len(componentsByNode),
		NodesState:   componentsByNode,
	}

	res.aggregateNodesState(desiredVersion)

	return res, nil
}

func getComponentsStateByNode(pods *corev1.PodList) (map[string]*MasterNodeState, error) {
	nodesState := make(map[string]*MasterNodeState)

	for _, pod := range pods.Items {
		nodeName := pod.Spec.NodeName
		var nodeState *MasterNodeState
		if _, exists := nodesState[nodeName]; !exists {
			nodesState[nodeName] = &MasterNodeState{
				ComponentsState: make(map[string]*ControlPlaneComponentState),
			}
		}
		nodeState = nodesState[nodeName]

		componentLabel, exists := pod.GetLabels()[componentLabelKey]
		if !exists {
			return nil, fmt.Errorf("%s label are missing", componentLabelKey)
		}

		kubeVersion, exists := pod.GetAnnotations()[kubeVersionAnnotation]
		if !exists {
			return nil, fmt.Errorf("%s annotation are missing", kubeVersionAnnotation)
		}

		version, err := version.NormalizeAndTrimPatch(kubeVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize kubernetes-version '%s': %w", kubeVersion, err)
		}

		component := &ControlPlaneComponentState{
			Version:   version,
			PodStatus: pod.Status,
		}

		nodeState.ComponentsState[componentLabel] = component
	}

	return nodesState, nil
}

func (s *ControlPlaneState) aggregateNodesState(desiredVersion string) {
	var desiredCount, upToDateCount, desiredComponentCount, upToDateComponentCount int
	var phase ControlPlanePhase

	for _, nodeState := range s.NodesState {
		var hasComponentUpdating, hasComponentFailed bool

		desiredCount++
		for _, componentState := range nodeState.ComponentsState {
			desiredComponentCount++

			if !componentState.isRunningAndReady() {
				hasComponentFailed = true
				continue
			}

			if !componentState.isUpdated(desiredVersion) {
				hasComponentUpdating = true
				continue
			}

			upToDateComponentCount++
		}

		if hasComponentFailed {
			nodeState.Phase = MasterNodeFailed
			continue
		}

		if hasComponentUpdating {
			nodeState.Phase = MasterNodeUpdating
			continue
		}

		nodeState.Phase = MasterNodeUptoDate
		upToDateCount++
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
	s.DesiredComponentCount = desiredComponentCount
	s.UpToDateComponentCount = upToDateComponentCount
	s.Phase = phase
}
