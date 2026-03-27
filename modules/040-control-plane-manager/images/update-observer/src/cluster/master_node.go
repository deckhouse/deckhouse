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

	corev1 "k8s.io/api/core/v1"

	podstatus "update-observer/pkg/pod-status"
	"update-observer/pkg/version"
)

type MasterNode struct {
	Phase       MasterNodePhase
	Description string
	Components  map[string]*ControlPlaneComponent
}

type MasterNodePhase string

const (
	MasterNodeUpToDate MasterNodePhase = "UpToDate"
	MasterNodeUpdating MasterNodePhase = "Updating"
	MasterNodeFailed   MasterNodePhase = "Failed"
)

func buildControlPlaneTopology(pods *corev1.PodList, desiredVersion string) (map[string]*MasterNode, error) {
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

		nodeState.Components[componentLabel] = newControlPlaneComponent(version, pod, desiredVersion)
	}

	return nodesState, nil
}

type ControlPlaneComponent struct {
	Version     string
	State       ControlPlaneComponentState
	Description string
}

type ControlPlaneComponentState int

const (
	ControlPlaneComponentFailed ControlPlaneComponentState = iota
	ControlPlaneComponentUpdating
	ControlPlaneComponentUpToDate
)

func newControlPlaneComponent(version string, pod corev1.Pod, desiredVersion string) *ControlPlaneComponent {
	state, message := determineComponentState(version, pod, desiredVersion)

	return &ControlPlaneComponent{
		Version:     version,
		State:       state,
		Description: message,
	}
}

func determineComponentState(version string, pod corev1.Pod, desiredVersion string) (ControlPlaneComponentState, string) {
	if pod.Status.Phase != corev1.PodRunning {
		return ControlPlaneComponentFailed, fmt.Sprintf("Pod is not Running - %s", pod.Status.Phase)
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		switch {
		case containerStatus.State.Waiting != nil:
			if slices.Contains(podstatus.GetProblematicStatuses(), containerStatus.State.Waiting.Reason) {
				return ControlPlaneComponentFailed, fmt.Sprintf("Container %s is Waiting - %s", containerStatus.Name, containerStatus.State.Waiting.Reason)
			}
			return ControlPlaneComponentUpdating, ""

		case containerStatus.State.Terminated != nil:
			if slices.Contains(podstatus.GetProblematicStatuses(), containerStatus.State.Terminated.Reason) {
				return ControlPlaneComponentFailed, fmt.Sprintf("Container %s is Terminated - %s", containerStatus.Name, containerStatus.State.Terminated.Reason)
			}
			return ControlPlaneComponentUpdating, ""

		case containerStatus.State.Running != nil && !containerStatus.Ready:
			return ControlPlaneComponentUpdating, ""
		}
	}

	if version != desiredVersion {
		return ControlPlaneComponentUpdating, ""
	}

	return ControlPlaneComponentUpToDate, ""
}
