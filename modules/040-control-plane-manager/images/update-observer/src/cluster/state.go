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

type Spec struct {
	DesiredVersion string     `yaml:"desiredVersion"`
	UpdateMode     UpdateMode `yaml:"updateMode"`
}

type Status struct {
	ControlPlaneState ControlPlaneState `json:"controlPlane" yaml:"controlPlane"`
	NodesState        NodesState        `json:"nodes" yaml:"nodes"`
	Phase             Phase             `json:"phase" yaml:"phase"`
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
