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
	EnvEnablePackageSystem  = "DECKHOUSE_ENABLE_PACKAGE_SYSTEM"
	EnvEnableModulePackages = "DECKHOUSE_ENABLE_MODULE_PACKAGES"
)

// PackageSystemEnabled reports whether the package-system controllers
// (PackageRepository, Application, ApplicationPackageVersion) are enabled.
func PackageSystemEnabled() bool { return os.Getenv(EnvEnablePackageSystem) == "true" }

// ModulePackagesEnabled reports whether the module-package controllers
// (ModulePackage, ModulePackageVersion, Module v2) are enabled.
func ModulePackagesEnabled() bool { return os.Getenv(EnvEnableModulePackages) == "true" }
