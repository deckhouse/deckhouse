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

package fsstatic

import (
	"path"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
)

const (
	LayoutsDir      = "layouts"
	InfraModulesDir = "terraform-modules"
	VersionsFile    = "versions.tf"
)

func GetPluginDir(root string, settings settings.ProviderSettings, version string, arch string) string {
	registry := "registry.terraform.io"
	if settings.UseOpenTofu() {
		registry = "registry.opentofu.org"
	}

	// /plugins/registry.opentofu.org/{{ $tf.namespace }}/{{ $tf.type }}/{{ $version }}/linux_amd64/{{ $tf.destinationBinary }}
	return path.Join(root, registry, settings.Namespace(), settings.Type(), version, arch, settings.DestinationBinary())
}

func GetInfraUtilPath(root string, settings settings.ProviderSettings) string {
	bin := "terraform"
	if settings.UseOpenTofu() {
		bin = "opentofu"
	}

	return path.Join(root, bin)
}

func GetVersionsFile(root string) string {
	return filepath.Join(root, VersionsFile)
}
