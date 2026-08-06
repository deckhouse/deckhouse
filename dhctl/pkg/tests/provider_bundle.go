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
func StubDeliveredProviderBundle(t *testing.T, downloadDir, provider string) {
	t.Helper()
	provider = strings.ToLower(provider)

	candiDir := RequireProviderCandiDir(t, provider)

	providerDir := providerdir.ProviderDir(downloadDir, provider)
	openapiDir := filepath.Join(providerDir, "openapi")
	require.NoError(t, os.MkdirAll(openapiDir, 0o755))

	schema, err := os.ReadFile(filepath.Join(candiDir, "openapi", "cluster_configuration.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(openapiDir, "cluster_configuration.yaml"), schema, 0o644))

	// A conformant validator reads the request off stdin and always answers
	// with a JSON object ("{}" on success, see go_lib/dhctl-provider-protocol);
	// empty stdout is treated as a broken binary and fails closed.
	validatorScript := "#!/bin/sh\ncat >/dev/null\nprintf '{}\\n'\n"
	require.NoError(t, os.WriteFile(providerdir.ValidatorPath(downloadDir, provider), []byte(validatorScript), 0o755))
}
