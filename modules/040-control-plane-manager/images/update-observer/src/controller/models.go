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

package controller

type ClusterConfiguration struct {
	KubernetesVersion string `yaml:"kubernetesVersion"`
	DesiredVersion    string `yaml:"desiredVersion"`
	UpdateMode        string
}

type NodesStatus struct {
	DesiredCount  int `json:"desiredCount" yaml:"desiredCount"`
	UpToDateCount int `json:"upToDateCount" yaml:"upToDateCount"`
}

type ControlPlaneStatus struct {
	DesiredCount   int    `json:"desiredCount" yaml:"desiredCount"`
	UpToDateCount  int    `json:"upToDateCount" yaml:"upToDateCount"`
	CurrentVersion string `json:"currentVersion" yaml:"currentVersion"`
	Progress       string `json:"progress" yaml:"progress"`
	State          string `json:"state" yaml:"state"`
}

type SpecData struct {
	DesiredVersion string `yaml:"desiredVersion"`
	UpdateMode     string `yaml:"updateMode"`
}

type Status struct {
	ControlPlane ControlPlaneStatus `json:"controlPlane" yaml:"controlPlane"`
	Nodes        NodesStatus        `json:"nodes" yaml:"nodes"`
	Phase        string             `json:"phase" yaml:"phase"`
}
