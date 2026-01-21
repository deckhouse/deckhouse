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
	"update-observer/common"

	corev1 "k8s.io/api/core/v1"
)

type ControlPlaneState struct {
	DesiredCount  int                         `json:"desiredCount" yaml:"desiredCount"`
	UpToDateCount int                         `json:"upToDateCount" yaml:"upToDateCount"`
	Progress      string                      `json:"progress" yaml:"progress"`
	Phase         ControlPlanePhase           `json:"phase" yaml:"phase"`
	NodesState    map[string]*MasterNodeState `json:"nodes" yaml:"nodes"`
}

type ControlPlanePhase string

const (
	ControlPlaneUpToDate     ControlPlanePhase = "UpToDate"
	ControlPlaneUpdating     ControlPlanePhase = "Updating"
	ControlPlaneInconsistent ControlPlanePhase = "Inconsistent"
	ControlPlaneVersionDrift ControlPlanePhase = "VersionDrift"
)

func GetControlPlaneState(controlPlanePods *corev1.PodList, desiredVersion string) (*ControlPlaneState, error) {
	nodesStatus, err := getNodesState(controlPlanePods, desiredVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to build nodes status: %w", err)
	}

	res := &ControlPlaneState{
		DesiredCount: len(nodesStatus),
		NodesState:   nodesStatus,
	}

	res.aggregateNodesState(desiredVersion)

	return res, nil
}

func getNodesState(pods *corev1.PodList, desiredVersion string) (map[string]*MasterNodeState, error) {
	nodesState := make(map[string]*MasterNodeState)

	for _, pod := range pods.Items {
		nodeName := pod.Spec.NodeName
		var nodeState *MasterNodeState
		if _, exists := nodesState[nodeName]; !exists {
			nodesState[nodeName] = &MasterNodeState{
				Phase:           MasterNodeUptoDate,
				ComponentsState: make(map[ControlPlaneComponentType]*ControlPlaneComponentState),
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

		version, err := common.NormalizeVersion(kubeVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize kubernetes-version '%s': %w", kubeVersion, err)
		}

		component := &ControlPlaneComponentState{
			Version: version,
			Phase:   pod.Status.Phase,
		}

		if nodeState.Phase != MasterNodeUpdating && !component.isFullyOperational(desiredVersion) {
			nodeState.Phase = MasterNodeUpdating
		}

		nodeState.ComponentsState[ControlPlaneComponentType(componentLabel)] = component
	}

	return nodesState, nil
}

func (s *ControlPlaneState) aggregateNodesState(desiredVersion string) {
	var desiredCount, desiredComponentCount, upToDateCount, upToDateComponentCount int
	var phase ControlPlanePhase

	for _, nodeState := range s.NodesState {
		desiredCount++
		if nodeState.isUpToDate() {
			upToDateCount++
		}

		for _, componentState := range nodeState.ComponentsState {
			desiredComponentCount++
			if componentState.isFullyOperational(desiredVersion) {
				upToDateComponentCount++
			}
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
	s.Progress = common.CalculateProgress(desiredComponentCount, upToDateComponentCount)
	s.Phase = phase
}

type ControlPlaneComponentsState map[ControlPlaneComponentType]*ControlPlaneComponentState

type ControlPlaneComponentState struct {
	Version string          `json:"version" yaml:"version"`
	Phase   corev1.PodPhase `json:"phase" yaml:"phase"`
}

type ControlPlaneComponentType string

const (
	KubeApiServer         ControlPlaneComponentType = "kube-apiserver"
	KubeScheduler         ControlPlaneComponentType = "kube-scheduler"
	KubeControllerManager ControlPlaneComponentType = "kube-controller-manager"
)

func (s *ControlPlaneComponentState) isFullyOperational(desiredVersion string) bool {
	return s.Version == desiredVersion && s.Phase == corev1.PodRunning
}