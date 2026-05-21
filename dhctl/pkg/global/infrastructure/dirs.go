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
// provider's terraform/tofu modules and openapi schemas.
// External provider images unpack into <downloadDir>/<provider>/ directly.
// Bundled providers live under candiDir/cloud-providers/<provider>.
func GetInfrastructureProviderDir(provider, downloadDir string) string {
	if _, err := os.Stat(filepath.Join(candiDir, "cloud-providers", provider)); err == nil {
		return filepath.Join(candiDir, "cloud-providers", provider)
	}
	if p := filepath.Join(downloadDir, provider); dirExists(p) {
		return p
	}
	return filepath.Join(downloadDir, "candi", "cloud-providers", provider)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func GetInfrastructureModulesDir(provider, downloadDir string) string {
	return filepath.Join(GetInfrastructureProviderDir(provider, downloadDir), "terraform-modules")
}

func GetInfrastructureModulesForRunningDir(provider, layout, module, downloadDir string) string {
	return filepath.Join(GetInfrastructureProviderDir(provider, downloadDir), "layouts", layout, module)
}

// GetInfrastructureVersions returns the path to the infrastructure-utility
// versions file. External provider images place it at <provider>/terraform-manager/terraform_versions.yml.
func GetInfrastructureVersions(downloadDir string) string {
	if _, err := os.Stat(infrastructureVersions); err == nil {
		return infrastructureVersions
	}
	matches, _ := filepath.Glob(filepath.Join(downloadDir, "*", "terraform-manager", "terraform_versions.yml"))
	if len(matches) > 0 {
		return matches[0]
	}
	return filepath.Join(downloadDir, "candi", "terraform_versions.yml")
}
