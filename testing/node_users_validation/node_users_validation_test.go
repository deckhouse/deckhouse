//go:build validation
// +build validation

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

package node_users_validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const nodeUsersFile = "node-users.yml"

// dhctl assembles the users list of every node's cloud-config, so a cloud provider has to
// say what belongs in it: the distro user its image already carries, or the account it
// creates instead. A provider without the file leaves that unsaid, and a terraform module
// then adds a users key of its own — which cloud-init resolves by keeping one key and
// dropping the rest, account of converge included.
func TestValidationNodeUsersDeclared(t *testing.T) {
	root := repoRoot(t)

	providers := providerCandiDirs(t, root)
	require.NotEmpty(t, providers, "no cloud provider found, the search is looking in the wrong place")

	for _, candiDir := range providers {
		name := providerName(candiDir)

		t.Run(name, func(t *testing.T) {
			path := filepath.Join(candiDir, nodeUsersFile)

			raw, err := os.ReadFile(path)
			require.NoErrorf(t, err, "every cloud provider declares its node users in candi/%s, next to cni-bootstrap.yml", nodeUsersFile)

			var declared struct {
				SchemaVersion int `json:"schemaVersion"`
				DefaultUser   any `json:"defaultUser"`
			}
			require.NoError(t, yaml.Unmarshal(raw, &declared), path)

			require.Equal(t, 1, declared.SchemaVersion, path)
			require.NotNil(t, declared.DefaultUser,
				`%s: defaultUser is required, write "default" when the image carries its own user`, path)

			switch user := declared.DefaultUser.(type) {
			case string:
				require.Equal(t, "default", user, `%s: the only marker is "default"`, path)
			case map[string]any:
				require.NotEmpty(t, user["name"], "%s: the declared account has no name", path)
			default:
				t.Fatalf(`%s: defaultUser is %T, want "default" or an account`, path, user)
			}
		})
	}
}

func providerCandiDirs(t *testing.T, root string) []string {
	t.Helper()

	var dirs []string

	for _, base := range []string{"modules", "ee/modules", "ee/se-plus/modules"} {
		entries, err := os.ReadDir(filepath.Join(root, base))
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() || !strings.Contains(entry.Name(), "cloud-provider-") {
				continue
			}

			candi := filepath.Join(root, base, entry.Name(), "candi")
			if _, err := os.Stat(filepath.Join(candi, "cni-bootstrap.yml")); err != nil {
				// A provider without cni-bootstrap.yml ships no candi tree of its own.
				continue
			}

			dirs = append(dirs, candi)
		}
	}

	return dirs
}

func providerName(candiDir string) string {
	name := filepath.Base(filepath.Dir(candiDir))

	return strings.TrimPrefix(name[strings.Index(name, "cloud-provider-"):], "cloud-provider-")
}

func repoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	root := filepath.Clean(filepath.Join(wd, "../.."))
	if _, err := os.Stat(filepath.Join(root, "modules")); err != nil {
		t.Fatal(fmt.Sprintf("repository root not found from %s", wd))
	}

	return root
}
