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

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
)

const (
	nodeUsersFileName   = "node-users.yml"
	nodeUsersSupportedV = 1
)

// ProviderDefaultUser is the account a provider declares for its image, standing in for
// cloud-init's "default". An image with no distro default user has nobody for cloud-init
// to hand the cluster ssh key to, so the provider names the account it does create.
type ProviderDefaultUser struct {
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

// sigs.k8s.io/yaml parses by converting YAML to JSON and unmarshalling with
// encoding/json, so these structs use json tags, not yaml tags.
type nodeUsers struct {
	SchemaVersion int                  `json:"schemaVersion"`
	DefaultUser   *ProviderDefaultUser `json:"defaultUser"`
}

// LoadProviderDefaultUser reads the account a provider declares in node-users.yml. Ten
// providers out of eleven ship no such file, and that is the answer rather than an error:
// their images carry a distro default user and cloud-init's "default" reaches it.
func LoadProviderDefaultUser(m *MetaConfig, globalOptions *options.GlobalOptions) (*ProviderDefaultUser, error) {
	if m == nil || m.ClusterType != CloudClusterType || m.ProviderName == "" {
		return nil, nil
	}

	path := nodeUsersPath(m, globalOptions)

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read node users file %s: %w", path, err)
	}

	var declared nodeUsers
	if err := yaml.Unmarshal(raw, &declared); err != nil {
		return nil, fmt.Errorf("parse node users file %s: %w", path, err)
	}

	if declared.SchemaVersion != nodeUsersSupportedV {
		return nil, fmt.Errorf("node users file %s: unsupported schemaVersion %d, want %d",
			path, declared.SchemaVersion, nodeUsersSupportedV)
	}

	if declared.DefaultUser == nil {
		return nil, nil
	}

	if declared.DefaultUser.Name == "" {
		return nil, fmt.Errorf("node users file %s: defaultUser has no name", path)
	}

	return declared.DefaultUser, nil
}

// nodeUsersPath prefers the bundled candi tree and falls back to the unpacked provider
// bundle, the way cniBootstrapPath does: external providers ship their files there.
func nodeUsersPath(m *MetaConfig, globalOptions *options.GlobalOptions) string {
	path := filepath.Join(resolveCandiDir(globalOptions), "cloud-providers", m.ProviderName, nodeUsersFileName)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	downloadRoot := m.DownloadRootDir
	if downloadRoot == "" && globalOptions != nil {
		downloadRoot = globalOptions.DownloadDir
	}

	if downloadRoot == "" {
		return path
	}

	return filepath.Join(providerdir.ProviderDir(downloadRoot, m.ProviderName), nodeUsersFileName)
}
