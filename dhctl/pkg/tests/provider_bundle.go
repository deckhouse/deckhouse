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

package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
)

// StubDeliveredProviderBundle makes dhctl/pkg/config.providerCandiPresent treat
// provider's bundle as already delivered under downloadDir, without a
// registry pull. Providers whose validator ships externally (e.g. yandex)
// require a downloaded bundle — this fakes the validator binary and copies the
// real ClusterConfiguration schema alongside it, so callers keep validating
// against the actual OpenAPI spec instead of a synthetic one. The schema comes
// from RequireProviderCandiDir, i.e. from the provider module when the candi
// bundle is not baked into the image.
// The stub validator answers "{}" to everything, so anything that finds it passes every
// preflight check. Some callers have to place it under options.DefaultTmpDir(), the very
// directory a real dhctl run reads, because the code under test resolves the download dir
// itself. Leaving it behind would make the next real bootstrap or converge on this machine
// silently skip provider validation, so every artefact this helper creates is removed again
// through t.Cleanup, innermost first, and pre-existing paths are left untouched.
func StubDeliveredProviderBundle(t *testing.T, downloadDir, provider string) {
	t.Helper()
	provider = strings.ToLower(provider)

	candiDir := RequireProviderCandiDir(t, provider)

	providerDir := providerdir.ProviderDir(downloadDir, provider)
	openapiDir := filepath.Join(providerDir, "openapi")

	// Remember which directories already existed: only the ones this helper creates may be
	// removed afterwards, and only once they are empty again.
	createdDirs := make([]string, 0, 2)
	for _, dir := range []string{providerDir, openapiDir} {
		if _, err := os.Lstat(dir); os.IsNotExist(err) {
			createdDirs = append(createdDirs, dir)
		}
	}

	require.NoError(t, os.MkdirAll(openapiDir, 0o755))

	schemaPath := filepath.Join(openapiDir, "cluster_configuration.yaml")
	validatorPath := providerdir.ValidatorPath(downloadDir, provider)

	t.Cleanup(func() {
		// Files first, then the directories this helper created, deepest last-created first.
		// os.Remove on a directory only succeeds while it is empty, so a directory another
		// test still populates survives.
		for _, path := range []string{validatorPath, schemaPath} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Errorf("cleanup stub provider bundle: remove %s: %v", path, err)
			}
		}
		for i := len(createdDirs) - 1; i >= 0; i-- {
			if err := os.Remove(createdDirs[i]); err != nil && !os.IsNotExist(err) {
				t.Logf("cleanup stub provider bundle: keep %s: %v", createdDirs[i], err)
			}
		}
	})

	schema, err := os.ReadFile(filepath.Join(candiDir, "openapi", "cluster_configuration.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(schemaPath, schema, 0o644))

	// A conformant validator reads the request off stdin and always answers
	// with a JSON object ("{}" on success, see go_lib/dhctl-provider-protocol);
	// empty stdout is treated as a broken binary and fails closed.
	validatorScript := "#!/bin/sh\ncat >/dev/null\nprintf '{}\\n'\n"
	require.NoError(t, os.WriteFile(validatorPath, []byte(validatorScript), 0o755))
}
