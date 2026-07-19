// Copyright 2025 Flant JSC
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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/gcp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
)

var terraformProviders = []string{
	"aws",
	gcp.ProviderName,
	"azure",
}

var tofuProviders = []string{
	yandex.ProviderName,
	"dvp",
	"dynamix",
	"zvirt",
	"vsphere",
	"huaweicloud",
	"openstack",
	vcd.ProviderName,
}

func TestAllProviderPresentInStore(t *testing.T) {
	s, err := loadProvidersForTest(t.Context(), options.DefaultInfrastructureVersions)
	require.NoError(t, err)

	all := append(make([]string, 0), tofuProviders...)
	all = append(all, terraformProviders...)

	require.Len(t, s, len(all))
}

func TestProvidersSettings(t *testing.T) {
	s, err := loadProvidersForTest(t.Context(), options.DefaultInfrastructureVersions)
	require.NoError(t, err)

	assertSettings := func(t *testing.T, s settingsStore, p string, assertProvider func(t *testing.T, settings settings.ProviderSettings)) {
		require.Contains(t, s, p)
		set := s[p]
		require.NotNil(t, set)

		assertProvider(t, set)

		require.NotEmpty(t, set.CloudName())
		require.NotEmpty(t, set.Namespace())
		require.NotEmpty(t, set.DestinationBinary())
		require.NotEmpty(t, set.Versions())
		require.NotEmpty(t, set.VMResourceType())
	}

	for _, p := range tofuProviders {
		assertSettings(t, s, p, func(t *testing.T, settings settings.ProviderSettings) {
			require.True(t, settings.UseOpenTofu())
			require.Equal(t, settings.InfrastructureVersion(), "1.12.0")
			require.Nil(t, settings.VMResource())
		})
	}

	for _, p := range terraformProviders {
		assertSettings(t, s, p, func(t *testing.T, settings settings.ProviderSettings) {
			require.False(t, settings.UseOpenTofu())
			require.Equal(t, settings.InfrastructureVersion(), "0.14.8")
			require.Nil(t, settings.VMResource())
		})
	}
}

func TestProviderSettingsLoadError(t *testing.T) {
	// settings store returns error on not exists file
	sFailed := newSettingsProvider(t.Context(), "/not/exists/file-aakjdiejfuefuefjej", "", func(_ context.Context, _, _ string) (settingsStore, error) {
		return nil, fmt.Errorf("file does not exist")
	})
	require.Error(t, sFailed.initError)
	require.Nil(t, sFailed.store)
	require.Len(t, fileToSettingsStore, 0)

	// failed store returns init error due getting
	_, err := sFailed.GetSettings(t.Context(), yandex.ProviderName, cloud.ProviderAdditionalParams{})
	require.Error(t, err)
}

func TestProviderSettingsLoadedAndStoreInCache(t *testing.T) {
	file := options.DefaultInfrastructureVersions

	assertOneStoreInCache := func(t *testing.T, store *SettingsProvider) {
		require.NoError(t, store.initError)
		require.NotNil(t, store)
		require.Len(t, fileToSettingsStore, 1)
		// The store is keyed by the versions file plus the bundle download dir,
		// since bundles contribute their own provider settings.
		require.Contains(t, fileToSettingsStore, file+"\x00")
	}

	allProviders := append(make([]string, 0), tofuProviders...)
	allProviders = append(allProviders, terraformProviders...)
	assertGettingDoesNotAffectStores := func(t *testing.T, store *SettingsProvider) {
		require.Len(t, fileToSettingsStore, 1)
		require.Len(t, store.store, len(allProviders))
	}

	sFirst := newSettingsProvider(t.Context(), file, "", loadOrGetStore)
	assertOneStoreInCache(t, sFirst)

	sSecond := newSettingsProvider(t.Context(), file, "", loadOrGetStore)
	assertOneStoreInCache(t, sSecond)

	require.Equal(t, sFirst.store, sSecond.store)

	// get settings for existing provider
	settingsYandex, err := sFirst.GetSettings(t.Context(), yandex.ProviderName, cloud.ProviderAdditionalParams{})
	require.NoError(t, err)
	require.False(t, govalue.IsNil(settingsYandex))
	require.Equal(t, settingsYandex.CloudName(), yandex.ProviderName)
	assertGettingDoesNotAffectStores(t, sFirst)

	// returns error for non exists store
	_, err = sFirst.GetSettings(t.Context(), "incorrect", cloud.ProviderAdditionalParams{})
	require.Error(t, err)
	assertGettingDoesNotAffectStores(t, sFirst)
}

// An external provider ships its settings inside its OCI bundle, not in the
// candi image. The fixture is the artifact werf actually packs (see
// modules/030-cloud-provider-dvp/images/terraform-manager/werf.inc.yaml), not a
// hand-written copy: the real file carries no `terraform:` key, and a fixture
// that invents one hides that the loader rejects it.
func TestBundleSettingsMergedFromDownloadDir(t *testing.T) {
	downloadDir := t.TempDir()
	installDVPBundle(t, downloadDir, "dvp")

	store, err := loadOrGetStore(t.Context(), writeCandiVersions(t), downloadDir)
	require.NoError(t, err)

	set, ok := store["dvp"]
	require.True(t, ok, "provider settings must come from the unpacked bundle")
	require.Equal(t, "kubernetes_manifest", set.VMResource().Type)
}

// writeCandiVersions stands in for the candi versions file, so the bundle tests
// exercise the merge whatever providers the shipped candi happens to carry.
func writeCandiVersions(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), versionFile)
	require.NoError(t, os.WriteFile(path, []byte(`opentofu: 1.12.0
terraform: 0.14.8
aws:
  namespace: hashicorp
  cloudName: AWS
  type: aws
  version: "4.62.0"
  artifact: terraform-provider-aws
  artifactBinary: terraform-provider-aws
  destinationBinary: terraform-provider-aws
  vmResourceType: aws_instance
  useOpentofu: false
yandex:
  namespace: yandex-cloud
  cloudName: Yandex
  type: yandex
  version: "0.121.0"
  artifact: terraform-provider-yandex
  artifactBinary: terraform-provider-yandex
  destinationBinary: terraform-provider-yandex
  vmResourceType: yandex_compute_instance
  useOpentofu: true
`), 0o644))

	return path
}

// installDVPBundle lays out an unpacked bundle from the files the DVP module
// ships, so the test breaks whenever the shipped artifact stops loading.
func installDVPBundle(t *testing.T, downloadDir, dirName string) {
	t.Helper()

	moduleCandi := filepath.Join("..", "..", "..", "..", "..", "modules", "030-cloud-provider-dvp", "candi")
	tm := filepath.Join(downloadDir, dirName, "terraform-manager")
	require.NoError(t, os.MkdirAll(tm, 0o755))

	for _, name := range []string{versionFile, planRulesFilename} {
		data, err := os.ReadFile(filepath.Join(moduleCandi, name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(tm, name), data, 0o644))
	}
}

// Only the canonical <provider> symlink is read: the digest dir it points at is
// also on disk (as are digest dirs of previously delivered versions and, mid
// unpack, an incomplete *.partial tree), and picking one of those would make the
// effective settings depend on directory ordering.
func TestBundleSettingsSkipDigestAndPartialDirs(t *testing.T) {
	downloadDir := t.TempDir()
	writeBundle := func(dir, cloudName string) {
		tm := filepath.Join(downloadDir, dir, "terraform-manager")
		require.NoError(t, os.MkdirAll(tm, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tm, versionFile), fmt.Appendf(nil, `opentofu: 1.12.0
terraform: 0.14.8
kubernetes:
  namespace: hashicorp
  cloudName: %s
  type: kubernetes
  version: "2.38.0"
  artifact: terraform-provider-kubernetes
  artifactBinary: terraform-provider-kubernetes
  destinationBinary: terraform-provider-kubernetes
  vmResourceType: kubernetes_manifest
  useOpentofu: true
`, cloudName), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tm, planRulesFilename), []byte("vmResource:\n  type: kubernetes_manifest\n"), 0o644))
	}

	// Current bundle, a stale one and an interrupted unpack, as they coexist on disk.
	writeBundle("dvp@sha256:current", "DVP")
	writeBundle("dvp@sha256:stale", "STALEDVP")
	writeBundle("dvp@sha256:broken.partial", "PARTIALDVP")
	require.NoError(t, os.Symlink("dvp@sha256:current", filepath.Join(downloadDir, "dvp")))

	store, err := loadOrGetStore(t.Context(), writeCandiVersions(t), downloadDir)
	require.NoError(t, err)

	require.Contains(t, store, "dvp")
	require.NotContains(t, store, "staledvp")
	require.NotContains(t, store, "partialdvp")
}

// In-tree providers unpack their terraform-manager bundle into the same
// download dir, and their fragment describes only themselves — no plan_rules,
// and only the one tool version they use. Those bundles must be ignored: candi
// already carries the provider, and treating them as external once broke every
// provider at once.
func TestBundleSettingsIgnoreInTreeAndBrokenBundles(t *testing.T) {
	downloadDir := t.TempDir()

	inTree := filepath.Join(downloadDir, "aws", "terraform-manager")
	require.NoError(t, os.MkdirAll(inTree, 0o755))
	awsFragment, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "modules", "030-cloud-provider-aws", "candi", versionFile))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(inTree, versionFile), awsFragment, 0o644))

	broken := filepath.Join(downloadDir, "brokenprovider", "terraform-manager")
	require.NoError(t, os.MkdirAll(broken, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(broken, versionFile), []byte("not yaml: [{"), 0o644))

	installDVPBundle(t, downloadDir, "dvp")

	store, err := loadOrGetStore(t.Context(), writeCandiVersions(t), downloadDir)
	require.NoError(t, err, "one unusable bundle must not take down the whole store")
	require.Contains(t, store, "aws", "candi stays authoritative for in-tree providers")
	require.Contains(t, store, "dvp")
	require.NotContains(t, store, "brokenprovider")
}

// loadProvidersForTest keeps the tests focused on the providers a versions file
// describes, without the tool versions loadVersionsFile also returns.
func loadProvidersForTest(ctx context.Context, filename string) (settingsStore, error) {
	file, err := loadVersionsFile(ctx, filename, toolVersions{})
	return file.providers, err
}
