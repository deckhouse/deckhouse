// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package meta holds shared identity constants for the cloud-provider-dvp module.
package meta

const (
	// ModuleName is the cloud-provider-dvp ModuleConfig name.
	ModuleName = "cloud-provider-dvp"
	// Namespace is the Kubernetes namespace of the cloud-provider-dvp module.
	Namespace = "d8-cloud-provider-dvp"
	// InstanceClassKind is the DVP InstanceClass resource kind.
	InstanceClassKind = "DVPInstanceClass"
)
