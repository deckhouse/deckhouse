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
	"update-observer/common"
	"update-observer/pkg/version"
)

type State struct {
	Spec
	Status
}

func GetState(cfg *Configuration, nodes *NodesState, controlPlane *ControlPlaneState, versionSettings VersionSettings, maxUsedVersion, sourceVersion string, downgradeInProgress bool) *State {
	currentVersion := determineCurrentVersion(nodes.versions, controlPlane.versions, downgradeInProgress)

	state := &State{
		Spec: Spec{
			DesiredVersion: cfg.DesiredVersion,
			UpdateMode:     cfg.UpdateMode,
		},
		Status: Status{
			CurrentVersion:    currentVersion,
			SupportedVersions: versionSettings.Supported,
			AvailableVersions: versionSettings.Available(version.GetMax(maxUsedVersion, currentVersion)), // prevent stale list when maxUsedVersion updates post-calculation
			AutomaticVersion:  versionSettings.Automatic,
			ControlPlaneState: *controlPlane,
			NodesState:        *nodes,
		},
	}

	state.determineStatePhase()
	state.calculateProgress(sourceVersion)

	return state
}

func (s *State) determineStatePhase() {
	var phase Phase
	switch s.ControlPlaneState.Phase {
	case ControlPlaneUpdating:
		phase = ClusterControlPlaneUpdating
	case ControlPlaneVersionDrift:
		phase = ClusterControlPlaneVersionDrift
	case ControlPlaneInconsistent:
		phase = ClusterControlPlaneInconsistent
	case ControlPlaneUpToDate:
		if s.Spec.UpdateMode == UpdateModeAutomatic && version.Compare(s.CurrentVersion, s.Spec.DesiredVersion) > 0 {
			phase = ClusterVersionDrift
			break
		}
		if s.NodesState.UpToDateCount < s.NodesState.DesiredCount {
			phase = ClusterNodesUpdating
			break
		}
		if s.NodesState.UpToDateCount == s.NodesState.DesiredCount {
			phase = ClusterUpToDate
			break
		}
		if s.NodesState.UpToDateCount > s.NodesState.DesiredCount {
			phase = ClusterInconsistent
			break
		}
	}

	s.Phase = phase
}

func (s *State) calculateProgress(sourceVersion string) {
	totalComponents := s.ControlPlaneState.DesiredComponentCount + s.NodesState.DesiredCount

	src := sourceVersion
	if src == "" {
		src = s.CurrentVersion
	}

	hops := version.Hops(src, s.Spec.DesiredVersion)
	if hops == 0 {
		s.Progress = common.CalculateProgress(
			s.ControlPlaneState.UpToDateComponentCount+s.NodesState.UpToDateCount,
			totalComponents)
		return
	}

	totalSteps := hops * totalComponents
	completedSteps := s.ControlPlaneState.StepsCompleted + s.NodesState.StepsCompleted

	s.Progress = common.CalculateProgress(completedSteps, totalSteps)
}

func determineCurrentVersion(nodes *version.UniqueAggregator, controlPlane *version.UniqueAggregator, downgradeInProgress bool) string {
	if downgradeInProgress {
		return version.GetMax(nodes.GetMax(), controlPlane.GetMax())
	}
	return version.GetMin(nodes.GetMin(), controlPlane.GetMin())
}

type Spec struct {
	DesiredVersion string
	UpdateMode     UpdateMode
}

type Status struct {
	CurrentVersion    string
	SupportedVersions []string
	AvailableVersions []string
	AutomaticVersion  string
	ControlPlaneState ControlPlaneState
	NodesState        NodesState
	Phase             Phase
	Progress          string
}

type Phase string

const (
	ClusterControlPlaneUpdating     Phase = "ControlPlaneUpdating"
	ClusterControlPlaneVersionDrift Phase = "ControlPlaneVersionDrift"
	ClusterControlPlaneInconsistent Phase = "ControlPlaneInconsistent"
	ClusterUpToDate                 Phase = "UpToDate"
	ClusterNodesUpdating            Phase = "NodesUpdating"
	ClusterVersionDrift             Phase = "VersionDrift"
	ClusterInconsistent             Phase = "Inconsistent"
)
