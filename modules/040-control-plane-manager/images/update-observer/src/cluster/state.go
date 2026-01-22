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

type State struct {
	Spec
	Status
}

func GetState(cfg *Configuration, nodes *NodesState, controlPlane *ControlPlaneState) *State {
	state := &State{
		Spec: Spec{
			DesiredVersion: cfg.DesiredVersion,
			UpdateMode:     cfg.UpdateMode,
		},
		Status: Status{
			CurrentVersion:    nodes.CurrentVersion,
			ControlPlaneState: *controlPlane,
			NodesState:        *nodes,
		},
	}

	determineStatePhase(state)

	return state
}

func determineStatePhase(s *State) {
	var phase Phase
	switch s.ControlPlaneState.Phase {
	case ControlPlaneUpdating:
		phase = ClusterControlPlaneUpdating
	case ControlPlaneVersionDrift:
		phase = ClusterControlPlaneVersionDrift
	case ControlPlaneInconsistent:
		phase = ClusterControlPlaneInconsistent
	case ControlPlaneUpToDate:
		if s.Spec.UpdateMode == UpdateModeAutomatic && s.NodesState.CurrentVersion > s.Spec.DesiredVersion {
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

type Spec struct {
	DesiredVersion string
	UpdateMode     UpdateMode
}

type Status struct {
	CurrentVersion    string
	ControlPlaneState ControlPlaneState
	NodesState        NodesState
	Phase             Phase
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
	ClusterUnknown                  Phase = "Unknown"
)
