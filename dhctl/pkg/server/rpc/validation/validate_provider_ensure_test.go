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

package validation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

// yandexTestSchema is a self-contained YandexClusterConfiguration schema written
// into a throwaway candi dir, so the test does not depend on the image's real
// candi being present (it is not, on a developer machine). Its required set is
// what an incomplete config below violates.
const yandexTestSchema = `kind: YandexClusterConfiguration
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
    additionalProperties: false
    required: [apiVersion, kind, layout, masterNodeGroup]
    properties:
      apiVersion: {type: string}
      kind: {type: string}
      layout: {type: string}
      masterNodeGroup: {type: object}
`

func TestValidateProviderSpecificClusterConfig_InTreeProviderIsValidated(t *testing.T) {
	// An in-tree provider (schema present in candi) is validated here, not
	// skipped: an incomplete config surfaces schema errors via Err. Counterpart
	// to the external skip below — proves the skip is specific to bundle-only
	// providers, not a blanket bypass.
	candiDir := t.TempDir()
	schemaDir := filepath.Join(candiDir, "cloud-providers", "yandex", "openapi")
	require.NoError(t, os.MkdirAll(schemaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(schemaDir, "cluster_configuration.yaml"), []byte(yandexTestSchema), 0o644))

	globalOptions := &options.GlobalOptions{CandiDir: candiDir, ModulesDir: t.TempDir(), DownloadDir: t.TempDir()}
	s := New(config.NewSchemaStore(globalOptions, candiDir), globalOptions)

	resp, err := s.ValidateProviderSpecificClusterConfig(context.Background(), &pb.ValidateProviderSpecificClusterConfigRequest{
		Config:        "apiVersion: deckhouse.io/v1\nkind: YandexClusterConfiguration\nlayout: Standard\n",
		ClusterConfig: `{"clusterType":"Cloud","cloud":{"provider":"Yandex"}}`,
		Opts:          &pb.ValidateOptions{CommanderMode: true},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Err, "in-tree provider must be validated, not skipped")
	require.Contains(t, resp.Err, "masterNodeGroup")
}

func TestValidateProviderSpecificClusterConfig_ExternalProviderSkipped(t *testing.T) {
	// An external provider (not bundled in candi) validates against a schema
	// that lives in its OCI bundle, which this stateless request cannot fetch;
	// the check is skipped instead of failing. The real operation revalidates
	// after reading the registry from the target cluster.
	globalOptions := &options.GlobalOptions{CandiDir: t.TempDir(), ModulesDir: t.TempDir(), DownloadDir: t.TempDir()}
	s := New(config.NewSchemaStore(globalOptions), globalOptions)

	resp, err := s.ValidateProviderSpecificClusterConfig(context.Background(), &pb.ValidateProviderSpecificClusterConfigRequest{
		Config:        "apiVersion: deckhouse.io/v1\nkind: DVPClusterConfiguration\nlayout: Standard\n",
		ClusterConfig: `{"clusterType":"Cloud","cloud":{"provider":"DVP"}}`,
		Opts:          &pb.ValidateOptions{CommanderMode: true},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Err, "external provider validation must be skipped, not failed")
}
