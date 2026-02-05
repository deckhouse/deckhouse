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
	podstatus "update-observer/pkg/pod-status"
	"update-observer/pkg/version"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type MasterNode struct {
	Phase      MasterNodePhase
	Components map[string]*ControlPlaneComponent
}

type MasterNodePhase string

const (
	MasterNodeUpToDate MasterNodePhase = "UpToDate"
	MasterNodeUpdating MasterNodePhase = "Updating"
	MasterNodeFailed   MasterNodePhase = "Failed"
)

func buildControlPlaneTopology(pods *corev1.PodList) (map[string]*MasterNode, error) {
	nodesState := make(map[string]*MasterNode)

	for _, pod := range pods.Items {
		nodeName := pod.Spec.NodeName
		var nodeState *MasterNode
		if _, exists := nodesState[nodeName]; !exists {
			nodesState[nodeName] = &MasterNode{
				Components: make(map[string]*ControlPlaneComponent),
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

		component := &ControlPlaneComponent{
			Version: version,
			Pod:     pod,
		}

		nodeState.Components[componentLabel] = component
	}

	return nodesState, nil
}

type ControlPlaneComponent struct {
	Version string
	Pod     corev1.Pod
}

type ControlPlaneComponentState int

const (
	ControlPlaneComponentFailed ControlPlaneComponentState = iota
	ControlPlaneComponentUpdating
	ControlPlaneComponentUpToDate
)

func (s *ControlPlaneComponent) getState(desiredVersion string) ControlPlaneComponentState {
	if s.Pod.Status.Phase != corev1.PodRunning {
		klog.Warningf("Pod is not in Running phase: \n\tName: %s\n\tPodPhase: %s",
			s.Pod.Name,
			s.Pod.Status.Phase)
		return ControlPlaneComponentFailed
	}

	for _, containerStatus := range s.Pod.Status.ContainerStatuses {
		switch {
		case containerStatus.State.Waiting != nil:
			if slices.Contains(podstatus.GetProblematicStatuses(), containerStatus.State.Waiting.Reason) {
				klog.Warningf("Container waiting state has problematic reason: \n\tName: %s\n\tReason: %s",
					containerStatus.Name,
					containerStatus.State.Waiting.Reason,
				)
				return ControlPlaneComponentFailed
			}
			return ControlPlaneComponentUpdating

		case containerStatus.State.Terminated != nil:
			if slices.Contains(podstatus.GetProblematicStatuses(), containerStatus.State.Terminated.Reason) {
				klog.Warningf("Container terminated state has problematic reason: \n\tName: %s\n\tReason: %s",
					containerStatus.Name,
					containerStatus.State.Terminated.Reason,
				)
				return ControlPlaneComponentFailed
			}
			return ControlPlaneComponentUpdating

		case containerStatus.State.Running != nil && !containerStatus.Ready:
			return ControlPlaneComponentUpdating
		}
	}

	if s.Version != desiredVersion {
		return ControlPlaneComponentUpdating
	}

	return ControlPlaneComponentUpToDate
}
