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

package immutable

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite the golden payload files")

// TestBuildCloudConfigGolden pins the exact bytes the master VM boots with. The
// on-node agent parses both documents strictly, so a silent rename of any field
// here is a node that refuses to bootstrap.
//
// Both documents are built the way the bootstrap builds them; only the three
// handoff strings are replaced with placeholders afterwards, because they are
// freshly minted on every run. Everything else in the golden file is what
// actually boots the node — the rendered control-plane manifests included, which
// is what makes the absence of any cluster key in them visible here.
func TestBuildCloudConfigGolden(t *testing.T) {
	metaConfig := testMetaConfig(t)
	globalOptions := options.NewGlobalOptions()

	nodeConfig, err := buildNodeConfig(context.Background(), nodeConfigInput{
		NodeName:   "example-master-0",
		MetaConfig: metaConfig,
	})
	require.NoError(t, err)

	controlPlaneConfig, err := buildControlPlaneConfig(context.Background(), MasterPayloadInput{
		NodeName:      "example-master-0",
		MetaConfig:    metaConfig,
		StateCache:    cache.NewTestCache(),
		CandiDir:      testCandiDir(t),
		GlobalOptions: &globalOptions,
	})
	require.NoError(t, err)

	controlPlaneConfig.Spec.Handoff = handoff{
		Token:      "<handoff token>",
		ServerCert: "<handoff server certificate>",
		ServerKey:  "<handoff server key>",
		// Public by design: this is the request, not the key. It is redacted
		// only to keep the golden stable across runs.
		ClientCSR: "<installer certificate request>",
	}

	cloudConfig, err := buildCloudConfig(nodeConfig, controlPlaneConfig)
	require.NoError(t, err)

	goldenPath := filepath.Join("testdata", "master-cloud-init.yaml")
	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(cloudConfig), 0o644))
	}

	golden, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	require.Equal(t, string(golden), cloudConfig)

	// The invariant the whole payload is shaped around. cloud-init ends up in a
	// Secret of somebody else's namespace, in the infrastructure state and in
	// the installer's cache, so no key of this cluster may ever travel in it:
	// the node generates its own PKI and never receives one. The only key that
	// legitimately appears in the real payload is the handoff serving key
	// redacted above, and it protects one read of one file.
	require.NotContains(t, cloudConfig, "PRIVATE KEY",
		"the master payload must carry no private key of the cluster")
}

// TestBuildControlPlaneConfigCarriesOnlyTheHandoffKey checks the same invariant
// on the unredacted payload: the handoff serving key is the one key in it, and
// it is in the handoff section.
func TestBuildControlPlaneConfigCarriesOnlyTheHandoffKey(t *testing.T) {
	metaConfig := testMetaConfig(t)
	globalOptions := options.NewGlobalOptions()

	controlPlaneConfig, err := buildControlPlaneConfig(context.Background(), MasterPayloadInput{
		NodeName:      "example-master-0",
		MetaConfig:    metaConfig,
		StateCache:    cache.NewTestCache(),
		CandiDir:      testCandiDir(t),
		GlobalOptions: &globalOptions,
	})
	require.NoError(t, err)

	document, err := yaml.Marshal(controlPlaneConfig)
	require.NoError(t, err)

	require.Contains(t, controlPlaneConfig.Spec.Handoff.ServerKey, "PRIVATE KEY")
	require.Equal(t,
		strings.Count(controlPlaneConfig.Spec.Handoff.ServerKey, "PRIVATE KEY"),
		strings.Count(string(document), "PRIVATE KEY"),
		"the only private key in the control-plane payload must be the handoff serving key",
	)
}

// TestBuildCloudConfigHasNoConflictingKeys guards the one cloud-init rule the
// provider's terraform imposes: it concatenates this document with a block of
// its own, so a top-level key both emit breaks the whole user-data.
func TestBuildCloudConfigHasNoConflictingKeys(t *testing.T) {
	metaConfig := testMetaConfig(t)

	nodeConfig, err := buildNodeConfig(context.Background(), nodeConfigInput{
		NodeName:   "example-master-0",
		MetaConfig: metaConfig,
	})
	require.NoError(t, err)

	cloudConfig, err := buildCloudConfig(nodeConfig, &controlPlaneConfig{
		APIVersion: payloadAPIVersion,
		Kind:       controlPlaneConfigKind,
	})
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(cloudConfig), &document))

	require.Contains(t, document, "write_files")
	require.Len(t, document, 1)
}
