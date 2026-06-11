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

const dvpSchemaYAML = `kind: DVPClusterConfiguration
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
    additionalProperties: false
    required: [apiVersion, kind, layout]
    properties:
      apiVersion:
        type: string
      kind:
        type: string
      layout:
        type: string
`

// writeUnpackedDVPBundle lays out what an unpacked provider bundle looks like
// on disk: schemas plus the validator binary marker that makes
// providerCandiPresent treat the bundle as already delivered.
func writeUnpackedDVPBundle(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "openapi"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "openapi", "cluster_configuration.yaml"), []byte(dvpSchemaYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "validator"), []byte("#!/bin/sh\n"), 0o755))
}

func TestValidateProviderSpecificClusterConfig_ExternalProviderSchemaFromBundle(t *testing.T) {
	downloadDir := t.TempDir()
	bundleDir := filepath.Join(downloadDir, "dvp")
	writeUnpackedDVPBundle(t, bundleDir)

	globalOptions := &options.GlobalOptions{
		CandiDir:    t.TempDir(),
		ModulesDir:  t.TempDir(),
		DownloadDir: downloadDir,
	}

	schemaStore := config.NewSchemaStore(nil)
	require.NoError(t, schemaStore.LoadProviderDir("dvp", "sha256:test-bundle", bundleDir))

	s := New(schemaStore, globalOptions)

	// Commander sends only the provider-specific section here; the provider
	// name arrives via ClusterConfig.
	providerSection := `
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
`
	resp, err := s.ValidateProviderSpecificClusterConfig(context.Background(), &pb.ValidateProviderSpecificClusterConfigRequest{
		Config:        providerSection,
		ClusterConfig: `{"clusterType":"Cloud","cloud":{"provider":"DVP"}}`,
		Opts:          &pb.ValidateOptions{CommanderMode: true},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Err, "DVP config must validate once the bundle schemas are loaded")
}

func TestValidateProviderSpecificClusterConfig_InvalidDVPDocFails(t *testing.T) {
	downloadDir := t.TempDir()
	bundleDir := filepath.Join(downloadDir, "dvp")
	writeUnpackedDVPBundle(t, bundleDir)

	globalOptions := &options.GlobalOptions{
		CandiDir:    t.TempDir(),
		ModulesDir:  t.TempDir(),
		DownloadDir: downloadDir,
	}

	schemaStore := config.NewSchemaStore(nil)
	require.NoError(t, schemaStore.LoadProviderDir("dvp", "sha256:test-bundle", bundleDir))

	s := New(schemaStore, globalOptions)

	providerSection := `
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
unknownField: boom
`
	resp, err := s.ValidateProviderSpecificClusterConfig(context.Background(), &pb.ValidateProviderSpecificClusterConfigRequest{
		Config:        providerSection,
		ClusterConfig: `{"clusterType":"Cloud","cloud":{"provider":"DVP"}}`,
		Opts:          &pb.ValidateOptions{CommanderMode: true},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Err, "schema violation must surface via the Err channel")
	require.Contains(t, resp.Err, "unknownField")
}
