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
	"encoding/base64"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite the golden payload files")

// redactedStatusToken stands for the bearer dhctl mints for every document; a
// fresh one on each run is exactly what a golden cannot hold.
const redactedStatusToken = "<status token>"

// TestBuildCloudConfigGolden pins the exact bytes the master VM boots with: the
// on-node agent parses strictly, so a silent field rename refuses to bootstrap.
// Only the freshly-minted handoff strings are replaced with placeholders.
func TestBuildDocumentStreamGolden(t *testing.T) {
	metaConfig := testMetaConfig(t)
	globalOptions := options.NewGlobalOptions()

	nodeConfig, err := buildNodeConfig(context.Background(), nodeConfigInput{
		NodeName:   "example-master-0",
		MetaConfig: metaConfig,
	})
	require.NoError(t, err)
	nodeConfig.Spec.StatusToken = redactedStatusToken

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

	stream, _, err := buildDocumentStream(nodeConfig, controlPlaneConfig)
	require.NoError(t, err)

	goldenPath := filepath.Join("testdata", "master-documents.yaml")
	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(stream), 0o644))
	}

	golden, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	require.Equal(t, string(golden), stream)

	// The invariant the whole payload is shaped around: no cluster key may
	// travel in the documents, which end up in Secrets, state and caches. The only
	// legitimate key is the redacted handoff serving key (one read of one file).
	require.NotContains(t, stream, "PRIVATE KEY",
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

// providerCloudInitTail is what the provider's terraform appends after the
// payload it is handed, byte for byte.
// Mirrors modules/030-cloud-provider-dvp/candi/terraform-modules/master/templates/cloudinit.tftpl.
const providerCloudInitTail = `
#cloud-config
hostname: example-master-0
prefer_fqdn_over_hostname: false
ssh_authorized_keys:
  - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5"

users:
- default
`

// TestCloudPayloadSurvivesTheProviderBlock is the cloud contract: terraform
// hands the machine the payload with its own #cloud-config block glued after
// it, and no key of that block may land on a document the node parses.
func TestCloudPayloadSurvivesTheProviderBlock(t *testing.T) {
	globalOptions := options.NewGlobalOptions()

	master, _, err := BuildMasterPayload(t.Context(), MasterPayloadInput{
		NodeName:      "example-master-0",
		MetaConfig:    testMetaConfig(t),
		StateCache:    cache.NewTestCache(),
		CandiDir:      testCandiDir(t),
		GlobalOptions: &globalOptions,
	})
	require.NoError(t, err)

	join, _, err := BuildJoinPayload(t.Context(), JoinPayloadInput{
		NodeName:           "example-master-1",
		MetaConfig:         testMetaConfig(t),
		CACert:             base64.StdEncoding.EncodeToString([]byte("cluster ca")),
		BootstrapToken:     immutabletest.BootstrapToken,
		APIServerEndpoints: []string{"https://10.0.0.1:6443"},
	})
	require.NoError(t, err)

	t.Run("master", func(t *testing.T) {
		documents := nodeDocuments(t, decodePayload(t, master)+providerCloudInitTail)

		nodeConfigDocument := requireNodeAccepts(t, documents, NodeConfigKind, &nodeConfig{})
		var carried map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(nodeConfigDocument), &carried))
		// Minted per document, so it is the one field the golden cannot pin.
		spec, ok := carried["spec"].(map[string]any)
		require.True(t, ok)
		require.NotEmpty(t, spec["statusToken"], "every document carries the bearer its node reports progress with")
		spec["statusToken"] = redactedStatusToken
		require.Equal(t, goldenNodeConfigDocument(t), carried,
			"the machine has to receive the very documents the golden pins")

		requireNodeAccepts(t, documents, controlPlaneConfigKind, &controlPlaneConfig{})
	})

	t.Run("static keeps the raw stream", func(t *testing.T) {
		static := testMetaConfig(t)
		static.ClusterType = config.StaticClusterType

		payload, _, err := BuildMasterPayload(t.Context(), MasterPayloadInput{
			NodeName:      "example-master-0",
			MetaConfig:    static,
			StateCache:    cache.NewTestCache(),
			CandiDir:      testCandiDir(t),
			GlobalOptions: &globalOptions,
		})
		require.NoError(t, err)

		// dhctl PUTs this one itself, so nothing is appended to it and there is
		// nothing for an envelope to protect the documents from.
		document := decodePayload(t, payload)
		require.NotContains(t, document, "#cloud-config")
		requireNodeAccepts(t, nodeDocuments(t, document), NodeConfigKind, &nodeConfig{})
		requireNodeAccepts(t, nodeDocuments(t, document), controlPlaneConfigKind, &controlPlaneConfig{})
	})

	t.Run("join", func(t *testing.T) {
		documents := nodeDocuments(t, decodePayload(t, join)+providerCloudInitTail)

		requireNodeAccepts(t, documents, NodeConfigKind, &nodeConfig{})
		require.NotContains(t, documents, controlPlaneConfigKind,
			"a joining master gets its control plane from control-plane-manager")
	})
}

func decodePayload(t *testing.T, payload string) string {
	t.Helper()

	document, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err)
	return string(document)
}

// requireNodeAccepts parses one document the way the on-node agent does — the
// same strict parse, against the types this package mirrors it with.
func requireNodeAccepts(t *testing.T, documents map[string]string, kind string, into any) string {
	t.Helper()

	document, filed := documents[kind]
	require.True(t, filed, "the node files the payload by kind and found no %s", kind)
	require.NoError(t, yaml.UnmarshalStrict([]byte(document), into),
		"the node refuses the %s it is handed", kind)
	return document
}

// nodeDocuments files a payload by kind the way the node's loader does: a
// #cloud-config is unwrapped into its write_files, anything else is split at
// the document markers.
// Mirrors documentParts/extractDocuments of images/init/src/0.1/acquire.go.
func nodeDocuments(t *testing.T, raw string) map[string]string {
	t.Helper()

	var parts []string
	if strings.HasPrefix(strings.TrimLeft(raw, " \t\r\n"), "#cloud-config") {
		var wrapper struct {
			WriteFiles []struct {
				Content string `json:"content"`
			} `json:"write_files"`
		}
		require.NoError(t, yaml.Unmarshal([]byte(raw), &wrapper))
		for _, file := range wrapper.WriteFiles {
			parts = append(parts, splitAtDocumentMarkers(file.Content)...)
		}
	} else {
		parts = splitAtDocumentMarkers(raw)
	}

	filed := make(map[string]string, len(parts))
	for _, part := range parts {
		var probe struct {
			Kind string `json:"kind"`
		}
		require.NoError(t, yaml.Unmarshal([]byte(part), &probe))
		if probe.Kind == "" {
			continue
		}
		filed[probe.Kind] = part
	}
	return filed
}

// splitAtDocumentMarkers cuts a stream at the markers at column zero, the way
// splitYAML of images/init/src/0.1/acquire.go does.
func splitAtDocumentMarkers(raw string) []string {
	var parts, document []string

	flush := func() {
		text := strings.Join(document, "\n")
		document = nil
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}

	for _, line := range strings.Split(raw, "\n") {
		if line == "---" || strings.HasPrefix(line, "--- ") {
			flush()
			continue
		}
		document = append(document, line)
	}
	flush()

	return parts
}
