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

package app

import "os"

// Feature-gate environment variables. Each turns on a block of controllers in
// pkg/controller when set to the literal "true".
const (
	EnvEnablePackageSystem    = "DECKHOUSE_ENABLE_PACKAGE_SYSTEM"
	EnvEnableResourceRequests = "DECKHOUSE_ENABLE_RESOURCE_REQUESTS"
	EnvEnableModuleV2         = "DECKHOUSE_ENABLE_MODULE_V2"
)

// PackageSystemEnabled reports whether the package-system controllers
// (PackageRepository, Application, ApplicationPackageVersion) are enabled.
func PackageSystemEnabled() bool { return os.Getenv(EnvEnablePackageSystem) == "true" }

// ResourceRequestsEnabled reports whether spec.resourceRequests is honoured.
// When on, the nelm service renders the package chart, overlays the per-workload
// replicas and container resources onto the rendered manifests, and installs the
// release from the patched manifests instead of the original chart.
//
// Gated because that install path replaces the chart nelm renders with a
// fully-rendered synthetic one: charts whose behaviour depends on anything
// beyond the manifests they produce are affected.
func ResourceRequestsEnabled() bool { return os.Getenv(EnvEnableResourceRequests) == "true" }

// ModuleV2Enabled reports whether the Module v2 controller is enabled.
func ModuleV2Enabled() bool { return os.Getenv(EnvEnableModuleV2) == "true" }
