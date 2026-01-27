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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type MasterNodeState struct {
	Phase           MasterNodePhase
	ComponentsState map[string]*ControlPlaneComponentState
}

type MasterNodePhase string

const (
	MasterNodeUptoDate MasterNodePhase = "UpToDate"
	MasterNodeUpdating MasterNodePhase = "Updating"
	MasterNodeFailed   MasterNodePhase = "Failed"
)

type ControlPlaneComponentState struct {
	Version   string
	PodStatus corev1.PodStatus
}

func (s *ControlPlaneComponentState) isUpdated(desiredVersion string) bool {
	return s.Version == desiredVersion
}

func (s *ControlPlaneComponentState) isRunningAndReady() bool {
	if s.PodStatus.Phase != corev1.PodRunning {
		return false
	}

	for _, containerStatus := range s.PodStatus.ContainerStatuses {
		if containerStatus.State.Running == nil || !containerStatus.Ready {
			klog.Warningf("Insufficient container state: \n\tName: %s\n\tRunning: %t\n\tReady: %t",
				containerStatus.Name,
				containerStatus.State.Running != nil,
				containerStatus.Ready,
			)
			return false
		}
	}

	return true
}
