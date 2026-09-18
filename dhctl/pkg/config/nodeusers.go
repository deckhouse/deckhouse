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
	"encoding/json"
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

// ProviderDefaultUser is the account a provider declares for its image. A nil value means
// the file named cloud-init's "default" marker: the image carries a distro user (ubuntu,
// debian, ec2-user) and cloud-init hands the cluster ssh key to it.
type ProviderDefaultUser struct {
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

// sigs.k8s.io/yaml parses by converting YAML to JSON and unmarshalling with
// encoding/json, so these structs use json tags, not yaml tags.
type nodeUsers struct {
	SchemaVersion int             `json:"schemaVersion"`
	DefaultUser   json.RawMessage `json:"defaultUser"`
}

// distroDefaultMarker is what a provider whose image has its own user writes in the file.
// It is cloud-init's own marker, spelled out so that the file says something in every
// provider rather than existing only where an account had to be declared.
const distroDefaultMarker = "default"

// LoadProviderDefaultUser reads the account a provider declares in node-users.yml. The file
// is mandatory for a cloud provider, the way cni-bootstrap.yml is: dhctl assembles the users
// list of every node's cloud-config, and this file is where a provider says what belongs in
// it. A nil user means the file named the distro default.
func LoadProviderDefaultUser(m *MetaConfig, globalOptions *options.GlobalOptions) (*ProviderDefaultUser, error) {
	if m == nil || m.ClusterType != CloudClusterType || m.ProviderName == "" {
		return nil, nil
	}

	path := nodeUsersPath(m, globalOptions)

	raw, err := os.ReadFile(path)
	if err != nil {
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

	return parseDefaultUser(path, declared.DefaultUser)
}

func parseDefaultUser(path string, raw json.RawMessage) (*ProviderDefaultUser, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("node users file %s: defaultUser is required, write %q when the image has its own user",
			path, distroDefaultMarker)
	}

	var marker string
	if err := json.Unmarshal(raw, &marker); err == nil {
		if marker != distroDefaultMarker {
			return nil, fmt.Errorf("node users file %s: defaultUser is %q, want %q or an account with a name",
				path, marker, distroDefaultMarker)
		}

		return nil, nil
	}

	var user ProviderDefaultUser
	if err := json.Unmarshal(raw, &user); err != nil {
		return nil, fmt.Errorf("node users file %s: defaultUser is neither %q nor an account: %w",
			path, distroDefaultMarker, err)
	}

	if user.Name == "" {
		return nil, fmt.Errorf("node users file %s: defaultUser has no name", path)
	}

	return &user, nil
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
