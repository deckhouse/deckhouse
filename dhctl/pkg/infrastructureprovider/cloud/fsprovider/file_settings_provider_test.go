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

// External providers (dvp, yandex) are deliberately absent: they carry no candi
// entry, their settings arrive with the bundle (see the bundle tests below).
var tofuProviders = []string{
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
			require.Equal(t, settings.InfrastructureVersion(), "1.12.5")
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
	require.NotContains(t, candiStoreCache, "/not/exists/file-aakjdiejfuefuefjej")

	// failed store returns init error due getting
	_, err := sFailed.GetSettings(t.Context(), yandex.ProviderName, cloud.ProviderAdditionalParams{})
	require.Error(t, err)
}

func TestProviderSettingsLoadedAndStoreInCache(t *testing.T) {
	file := options.DefaultInfrastructureVersions

	assertCandiCached := func(t *testing.T, store *SettingsProvider) {
		require.NoError(t, store.initError)
		require.NotNil(t, store)
		require.Contains(t, candiStoreCache, file)
	}

	allProviders := append(make([]string, 0), tofuProviders...)
	allProviders = append(allProviders, terraformProviders...)
	assertGettingDoesNotAffectStores := func(t *testing.T, store *SettingsProvider) {
		require.Contains(t, candiStoreCache, file)
		require.Len(t, store.store, len(allProviders))
	}

	sFirst := newSettingsProvider(t.Context(), file, "", loadOrGetStore)
	assertCandiCached(t, sFirst)

	sSecond := newSettingsProvider(t.Context(), file, "", loadOrGetStore)
	assertCandiCached(t, sSecond)

	require.Equal(t, sFirst.store, sSecond.store)

	// get settings for existing provider
	settingsVCD, err := sFirst.GetSettings(t.Context(), vcd.ProviderName, cloud.ProviderAdditionalParams{})
	require.NoError(t, err)
	require.False(t, govalue.IsNil(settingsVCD))
	require.Equal(t, settingsVCD.CloudName(), vcd.ProviderName)
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
func TestBundleSettingsForDVPComeFromModuleFiles(t *testing.T) {
	downloadDir := t.TempDir()
	bundleDir := installProviderBundle(t, downloadDir, "dvp@sha256:current", "030-cloud-provider-dvp")

	store, err := loadOrGetStore(t.Context(), writeCandiVersions(t), bundleDir)
	require.NoError(t, err)

	set, ok := store["dvp"]
	require.True(t, ok, "provider settings must come from the unpacked bundle")

	// The precise rule from the bundle's plan_rules.yml — not the coarse
	// type-only fallback — so a converge only calls a VirtualMachine delete a VM
	// change, not every kubernetes_manifest (disks, IPs) delete.
	rule := set.VMResource()
	require.NotNil(t, rule)
	require.Equal(t, "kubernetes_manifest", rule.Type)
	require.NotNil(t, rule.FieldEquals)
	require.Equal(t, "manifest.kind", rule.FieldEquals.Path)
	require.Equal(t, "VirtualMachine", rule.FieldEquals.Value)
}

// A bundle delivered after the store was first built (long-lived dhctl-server,
// converge exporter) must be picked up: the store caches only the candi file,
// so the second call re-merges the now-present bundle instead of returning the
// pre-bundle map.
func TestBundleDeliveredAfterFirstBuildIsPickedUp(t *testing.T) {
	candiFile := writeCandiVersions(t)
	downloadDir := t.TempDir()

	before, err := loadOrGetStore(t.Context(), candiFile, "")
	require.NoError(t, err)
	require.NotContains(t, before, "dvp", "bundle not delivered yet")

	bundleDir := installProviderBundle(t, downloadDir, "dvp@sha256:current", "030-cloud-provider-dvp")

	after, err := loadOrGetStore(t.Context(), candiFile, bundleDir)
	require.NoError(t, err)
	require.Contains(t, after, "dvp", "store must reflect a bundle delivered after the first build")
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
vcd:
  namespace: vmware
  cloudName: VCD
  type: vcd
  version: "3.14.1"
  artifact: terraform-provider-vcd-artifact
  artifactBinary: terraform-provider-vcd
  destinationBinary: terraform-provider-vcd
  vmResourceType: vcd_vapp_vm
  useOpentofu: true
`), 0o644))

	return path
}

// installProviderBundle lays the settings of a provider module out the way its unpacked bundle
// carries them and returns the bundle directory.
func installProviderBundle(t *testing.T, downloadDir, dirName, moduleDir string) string {
	t.Helper()

	moduleCandi := filepath.Join("..", "..", "..", "..", "..", "modules", moduleDir, "candi")
	bundleDir := filepath.Join(downloadDir, dirName)
	tm := filepath.Join(bundleDir, "terraform-manager")
	require.NoError(t, os.MkdirAll(tm, 0o755))

	for _, name := range []string{versionFile, planRulesFilename} {
		data, err := os.ReadFile(filepath.Join(moduleCandi, name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(tm, name), data, 0o644))
	}

	return bundleDir
}

// Yandex settings come from its bundle now that candi carries no yandex entry.
// The fixture is what werf packs from the module, so this breaks if the module
// stops shipping either half of the pair (a bundle without plan_rules.yml fails
// in attachBundlePlanRules).
func TestBundleSettingsForYandexComeFromModuleFiles(t *testing.T) {
	downloadDir := t.TempDir()
	bundleDir := installProviderBundle(t, downloadDir, yandex.ProviderName+"@sha256:current", "030-cloud-provider-yandex")

	store, err := loadOrGetStore(t.Context(), writeCandiVersions(t), bundleDir)
	require.NoError(t, err)

	set, ok := store[yandex.ProviderName]
	require.True(t, ok, "yandex settings must come from the unpacked bundle")
	require.True(t, set.UseOpenTofu())
	require.Equal(t, "1.12.0", set.InfrastructureVersion())
	require.Equal(t, "yandex_compute_instance", set.VMResourceType())

	rule := set.VMResource()
	require.NotNil(t, rule, "plan_rules.yml must ship in the bundle")
	require.Equal(t, "yandex_compute_instance", rule.Type)
	require.Nil(t, rule.FieldEquals, "a yandex_compute_instance change is always a VM change")
}

// Only the bundle of the operation is read: digest dirs of other builds and, mid
// unpack, an incomplete *.partial tree lie beside it in the same download dir.
func TestBundleSettingsComeOnlyFromTheGivenBundle(t *testing.T) {
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

	store, err := loadOrGetStore(t.Context(), writeCandiVersions(t), filepath.Join(downloadDir, "dvp@sha256:current"))
	require.NoError(t, err)

	require.Contains(t, store, "dvp")
	require.NotContains(t, store, "staledvp")
	require.NotContains(t, store, "partialdvp")
}

// The bundle is the build the cluster runs, so a fragment dhctl cannot use is an error naming it. An
// in-tree provider's terraform-manager image carries only its own fragment, without plan_rules.yml:
// candi knows that provider, so the fragment is dropped and candi's settings stay.
func TestBundleSettingsFragments(t *testing.T) {
	writeFile := func(t *testing.T, bundleDir, name, content string) {
		t.Helper()
		tm := filepath.Join(bundleDir, terraformManagerDir)
		require.NoError(t, os.MkdirAll(tm, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tm, name), []byte(content), 0o644))
	}

	tests := []struct {
		name    string
		write   func(t *testing.T, bundleDir string)
		wantErr string
	}{
		{
			name:  "fragment of a provider candi knows",
			write: writeAWSFragment,
		},
		{
			name: "unparsable fragment",
			write: func(t *testing.T, bundleDir string) {
				writeFile(t, bundleDir, versionFile, "not yaml: [{")
			},
			wantErr: "parse tool versions",
		},
		{
			// Two providers are malformed even when candi knows one of them and would drop it.
			name: "two providers, one of them known to candi",
			write: func(t *testing.T, bundleDir string) {
				writeFile(t, bundleDir, versionFile, `aws:
  namespace: hashicorp
  cloudName: AWS
  type: aws
  version: "5.83.1"
  artifact: terraform-provider-aws
  artifactBinary: terraform-provider-aws
  destinationBinary: terraform-provider-aws
  vmResourceType: aws_instance
  useOpentofu: false
kubernetes:
  namespace: hashicorp
  cloudName: DVP
  type: kubernetes
  version: "2.38.0"
  artifact: terraform-provider-kubernetes
  artifactBinary: terraform-provider-kubernetes
  destinationBinary: terraform-provider-kubernetes
  vmResourceType: kubernetes_manifest
  useOpentofu: true
`)
				writeFile(t, bundleDir, planRulesFilename, "vmResource:\n  type: kubernetes_manifest\n")
			},
			wantErr: "must describe exactly one provider, got 2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bundleDir := filepath.Join(t.TempDir(), "bundle")
			tc.write(t, bundleDir)

			store, err := loadOrGetStore(t.Context(), writeCandiVersions(t), bundleDir)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, filepath.Join(bundleDir, terraformManagerDir, versionFile))
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, []string{"4.62.0"}, store["aws"].Versions(), "candi stays authoritative for in-tree providers")
		})
	}
}

// A complete bundle under a name candi does not know may still describe a provider candi ships.
// candi's settings for that provider stay.
func TestBundleSettingsNeverReplaceACandiProvider(t *testing.T) {
	bundleDir := filepath.Join(t.TempDir(), "ensother@sha256:x")
	writeAWSFragment(t, bundleDir)
	planRules := filepath.Join(bundleDir, "terraform-manager", planRulesFilename)
	require.NoError(t, os.WriteFile(planRules, []byte("vmResource:\n  type: aws_instance\n"), 0o644))

	store, err := loadOrGetStore(t.Context(), writeCandiVersions(t), bundleDir)
	require.NoError(t, err)
	require.Equal(t, []string{"4.62.0"}, store["aws"].Versions(), "the fragment ships 5.83.1, candi 4.62.0")
}

// writeAWSFragment puts the terraform_versions.yml the aws module ships into a bundle dir.
func writeAWSFragment(t *testing.T, bundleDir string) {
	t.Helper()

	tm := filepath.Join(bundleDir, "terraform-manager")
	require.NoError(t, os.MkdirAll(tm, 0o755))
	fragment, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "modules", "030-cloud-provider-aws", "candi", versionFile))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tm, versionFile), fragment, 0o644))
}

// loadProvidersForTest keeps the tests focused on the providers a versions file
// describes, without the tool versions loadVersionsFile also returns.
func loadProvidersForTest(ctx context.Context, filename string) (settingsStore, error) {
	file, err := loadVersionsFile(ctx, filename, toolVersions{})
	return file.providers, err
}
