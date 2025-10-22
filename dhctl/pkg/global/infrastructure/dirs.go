// Copyright 2025 Flant JSC
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

package infrastructure

import "path/filepath"

var (
	dhctlPath              = "/"
	deckhouseDir           = "/deckhouse"
	candiDir               = deckhouseDir + "/candi"
	infrastructureVersions = candiDir + "/terraform_versions.yml"
)

func InitGlobalVars(pwd string) {
	dhctlPath = pwd
	deckhouseDir = pwd + "/deckhouse"
	candiDir = deckhouseDir + "/candi"
	infrastructureVersions = candiDir + "/terraform_versions.yml"
}

func GetDhctlPath() string {
	return dhctlPath
}

func GetInfrastructureProviderDir(provider string) string {
	return filepath.Join(candiDir, "cloud-providers", provider)
}

func GetInfrastructureModulesDir(provider string) string {
	return filepath.Join(GetInfrastructureProviderDir(provider), "terraform-modules")
}

func GetInfrastructureModulesForRunningDir(provider, layout, module string) string {
	return filepath.Join(GetInfrastructureProviderDir(provider), "layouts", layout, module)
}

func GetInfrastructureVersions() string {
	return infrastructureVersions
}
