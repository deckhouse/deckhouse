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

import (
	"os"
	"path/filepath"
)

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

// GetInfrastructureProviderDir returns the directory containing the cloud
// provider's terraform/tofu modules. Falls back to <downloadDir>/deckhouse/candi
// if the bundled candiDir does not exist.
func GetInfrastructureProviderDir(provider, downloadDir string) string {
	_, err := os.Stat(filepath.Join(candiDir, "cloud-providers", provider))
	if err == nil {
		return filepath.Join(candiDir, "cloud-providers", provider)
	}

	return filepath.Join(downloadDir, "deckhouse", "candi", "cloud-providers", provider)
}

func GetInfrastructureModulesDir(provider, downloadDir string) string {
	return filepath.Join(GetInfrastructureProviderDir(provider, downloadDir), "terraform-modules")
}

func GetInfrastructureModulesForRunningDir(provider, layout, module, downloadDir string) string {
	return filepath.Join(GetInfrastructureProviderDir(provider, downloadDir), "layouts", layout, module)
}

// GetInfrastructureVersions returns the path to the infrastructure-utility
// versions file. Falls back to <downloadDir>/deckhouse/candi if the bundled
// path does not exist.
func GetInfrastructureVersions(downloadDir string) string {
	_, err := os.Stat(infrastructureVersions)
	if err == nil {
		return infrastructureVersions
	}

	return filepath.Join(downloadDir, "deckhouse", "candi", "terraform_versions.yml")
}
