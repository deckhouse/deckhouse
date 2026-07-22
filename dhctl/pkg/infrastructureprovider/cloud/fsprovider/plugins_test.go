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

package fsprovider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
)

func TestDownloadPluginUsesUnpackedBundle(t *testing.T) {
	root := t.TempDir()
	bundleTM := filepath.Join(root, "dvp", "terraform-manager")
	require.NoError(t, os.MkdirAll(bundleTM, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleTM, "terraform-provider-kubernetes"), []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleTM, versionFile), []byte("terraform: 1\n"), 0o644))

	set := &settings.Simple{
		NamespaceVal:         ptr.To("hashicorp"),
		TypeVal:              ptr.To("kubernetes"),
		CloudNameVal:         ptr.To("DVP"),
		DestinationBinaryVal: ptr.To("terraform-provider-kubernetes"),
		UseOpenTofuVal:       ptr.To(true),
	}

	p := newPluginsProvider(filepath.Join(root, "no-plugins-dir"))
	dest := filepath.Join(t.TempDir(), "terraform-provider-kubernetes")

	// The plugin must come from the unpacked provider bundle: no baked plugins
	// dir, no flat terraform-manager dir and no registry access are available.
	err := p.DownloadPlugin(context.Background(), cloud.InfrastructurePluginProviderParams{
		Version:  cloud.Version{Version: "2.38.0", Arch: "linux_amd64"},
		Settings: set,
	}, dest, &config.MetaConfig{DownloadRootDir: root})
	require.NoError(t, err)
	require.FileExists(t, dest)
	// Bundle settings are read where the bundle keeps them; nothing is copied
	// into the shared candi dir any more.
	require.NoFileExists(t, filepath.Join(root, "deckhouse", "candi", versionFile))
}
