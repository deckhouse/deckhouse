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
	Version string
	Phase   corev1.PodPhase
}

func (s *ControlPlaneComponentState) isUpdated(desiredVersion string) bool {
	return s.Version == desiredVersion
}

func (s *ControlPlaneComponentState) isRunning() bool {
	return s.Phase == corev1.PodRunning
}
