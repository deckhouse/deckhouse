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

type UpdateMode string

const (
	UpdateModeAutomatic UpdateMode = "Automatic"
	UpdateModeManual    UpdateMode = "Manual"
)

// Configuration is the operator-declared desired state for the cluster's Kubernetes version:
// spec.desiredVersion/spec.updateMode of the ConfigMap this controller owns. It is *resolved* by
// the global discovery hook (ModuleConfig control-plane-manager kubernetesVersion, falling back to
// the deprecated ClusterConfiguration field) and reaches this controller as container environment.
// This controller is the only *writer* of the block; it treats the value purely as external input
// and never computes it itself.
type Configuration struct {
	DesiredVersion string
	UpdateMode     UpdateMode
	// MaxUsedVersion is the highest Kubernetes minor the cluster has ever converged onto, mirrored
	// into spec.maxUsedKubernetesVersion (controller.Spec carries the exact definition). Unlike the
	// other two it is a fact about the past rather than a declaration, and it is monotonic: readers
	// use it as the floor a downgrade may not cross.
	MaxUsedVersion string
}
