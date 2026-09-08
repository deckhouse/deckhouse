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

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

const ensureRegistryMCDoc = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: r.example.com/test
        username: test-user
        password: test-password
        scheme: HTTPS
  version: 1
`

func ensureClusterConfigDoc(provider string) string {
	return fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: %s
`, provider)
}

func ensureTestGlobalOptions(t *testing.T) *options.GlobalOptions {
	t.Helper()
	downloadDir := t.TempDir()
	return &options.GlobalOptions{
		CandiDir:         t.TempDir(),
		ModulesDir:       t.TempDir(),
		DownloadDir:      downloadDir,
		DownloadCacheDir: filepath.Join(downloadDir, "cache"),
	}
}

func stubProviderDigest(t *testing.T, digest string, calls *atomic.Int32) {
	t.Helper()
	orig := resolveProviderBundleRef
	resolveProviderBundleRef = func(_ context.Context, _ string, _ providerModuleLookup, _ *options.GlobalOptions) (providerBundleRef, error) {
		if calls != nil {
			calls.Add(1)
		}
		return providerBundleRef{Digest: digest}, nil
	}

	t.Cleanup(func() { resolveProviderBundleRef = orig })
}

func stubProviderDownload(t *testing.T, kind string, delay time.Duration, calls *atomic.Int32) {
	t.Helper()
	orig := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, _, dest, _ string, _ image.RegistryConfig, _ bool) error {
		calls.Add(1)
		time.Sleep(delay)
		writeTestProviderSchema(t, dest, kind)
		return nil
	}

	t.Cleanup(func() { downloadProviderBundle = orig })
}

func TestEnsureProviderBundleStaticNoop(t *testing.T) {
	var digestCalls atomic.Int32
	stubProviderDigest(t, "sha256:unused", &digestCalls)

	docs := []string{`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
`}
	require.NoError(t, EnsureProviderBundle(context.Background(), "", docs, ensureTestGlobalOptions(t)))
	require.Zero(t, digestCalls.Load(), "static cluster must not resolve provider digest")
}

func TestEnsureProviderBundleInTreeNoop(t *testing.T) {
	var digestCalls atomic.Int32
	stubProviderDigest(t, "sha256:unused", &digestCalls)

	globalOptions := ensureTestGlobalOptions(t)
	schemaPath := filepath.Join(globalOptions.CandiDir, "cloud-providers", "yandex", "openapi")
	require.NoError(t, os.MkdirAll(schemaPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(schemaPath, "cluster_configuration.yaml"), []byte("kind: X\napiVersions: []\n"), 0o644))

	require.NoError(t, EnsureProviderBundle(context.Background(), "", []string{ensureClusterConfigDoc("Yandex")}, globalOptions))
	require.NoError(t, EnsureProviderBundle(context.Background(), "Yandex", nil, globalOptions), "explicit provider must hit the same no-op")
	require.Zero(t, digestCalls.Load(), "in-tree provider with bundled candi must not resolve digest")
}

func TestEnsureProviderBundleDefaultRegistryFallback(t *testing.T) {
	// Docs without registry data fall back to the default public registry —
	// same semantics as the rest of dhctl.
	stubProviderDigest(t, "sha256:noreg", nil)

	var gotImgName string
	orig := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, imgName, dest, _ string, _ image.RegistryConfig, _ bool) error {
		gotImgName = imgName

		writeTestProviderSchema(t, dest, "EnsNoRegConfiguration")
		return nil
	}

	t.Cleanup(func() { downloadProviderBundle = orig })

	err := EnsureProviderBundle(context.Background(), "", []string{ensureClusterConfigDoc("EnsNoReg")}, ensureTestGlobalOptions(t))
	require.NoError(t, err)
	require.Equal(t, "registry.deckhouse.io/deckhouse/ce@sha256:noreg", gotImgName)
}

func TestEnsureProviderBundleDownloadsLoadsAndCaches(t *testing.T) {
	stubProviderDigest(t, "sha256:enstest1", nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsTestConfiguration", 0, &downloads)

	globalOptions := ensureTestGlobalOptions(t)
	docs := []string{ensureClusterConfigDoc("EnsTest"), ensureRegistryMCDoc}

	require.NoError(t, EnsureProviderBundle(context.Background(), "", docs, globalOptions))
	require.Equal(t, int32(1), downloads.Load())

	providerDir := filepath.Join(globalOptions.DownloadDir, "enstest")
	link, err := os.Lstat(providerDir)
	require.NoError(t, err)
	require.NotZero(t, link.Mode()&os.ModeSymlink, "provider dir must be a symlink to the digest dir")
	_, err = os.Stat(filepath.Join(providerDir, "openapi", "cluster_configuration.yaml"))
	require.NoError(t, err)

	schemaStore := NewSchemaStore(globalOptions)
	require.NotNil(t, schemaStore.Get(&SchemaIndex{Kind: "EnsTestConfiguration", Version: "deckhouse.io/v1"}))

	// Warm path: no second download.
	require.NoError(t, EnsureProviderBundle(context.Background(), "", docs, globalOptions))
	require.Equal(t, int32(1), downloads.Load())
}

func TestEnsureProviderBundleLoadsDeliveredBundleFromDisk(t *testing.T) {
	// The bundle is delivered on disk (validator + schemas) but never loaded
	// into this process's store — as happens on a shared/persisted download dir
	// where another process unpacked it. EnsureProviderBundle must load the
	// schemas from disk without downloading.
	var downloads atomic.Int32
	orig := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, _, _, _ string, _ image.RegistryConfig, _ bool) error {
		downloads.Add(1)
		return nil
	}

	t.Cleanup(func() { downloadProviderBundle = orig })

	globalOptions := ensureTestGlobalOptions(t)
	bundleDir := filepath.Join(globalOptions.DownloadDir, "ensdelivered")
	writeTestProviderSchema(t, bundleDir, "EnsDeliveredConfiguration")
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "validator"), []byte("#!/bin/sh\n"), 0o755))

	require.NoError(t, EnsureProviderBundle(context.Background(), "ensdelivered", nil, globalOptions))
	require.Equal(t, int32(0), downloads.Load(), "delivered bundle must load from disk, not download")

	schemaStore := NewSchemaStore(globalOptions)
	require.NotNil(t,
		schemaStore.Get(&SchemaIndex{Kind: "EnsDeliveredConfiguration", Version: "deckhouse.io/v1"}),
		"schemas must be loaded from the delivered on-disk bundle",
	)
}

func TestEnsureProviderBundleSingleflight(t *testing.T) {
	stubProviderDigest(t, "sha256:ensflight", nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsFlightConfiguration", 50*time.Millisecond, &downloads)

	globalOptions := ensureTestGlobalOptions(t)
	docs := []string{ensureClusterConfigDoc("EnsFlight"), ensureRegistryMCDoc}

	var wg sync.WaitGroup
	errs := make([]error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			errs[n] = EnsureProviderBundle(context.Background(), "", docs, globalOptions)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), downloads.Load(), "concurrent calls must share one download")
}

func TestApplyRegistryToDeckhouseConfigPopulatesDecodableDockerCfg(t *testing.T) {
	// Commander cold-pod ops carry registry creds in a separate registryConfig
	// doc, not in the cluster config; the lazy provider-plugin download then
	// decodes DeckhouseConfig.RegistryDockerCfg. An empty value used to crash
	// with "unmarshaling dockerconfig JSON: unexpected end of JSON input".
	metaConfig := &MetaConfig{}
	require.NoError(t, applyRegistryToDeckhouseConfig(metaConfig, []string{ensureRegistryMCDoc}))

	require.NotEmpty(t, metaConfig.DeckhouseConfig.RegistryDockerCfg, "dockercfg must be populated from registry MC")
	require.Equal(t, "r.example.com/test", metaConfig.DeckhouseConfig.ImagesRepo)
	require.True(t, strings.EqualFold("HTTPS", metaConfig.DeckhouseConfig.RegistryScheme))

	// The exact round-trip the lazy image download performs must succeed.
	dc, err := image.DecodeDockerConfig(metaConfig.DeckhouseConfig.RegistryDockerCfg)
	require.NoError(t, err)
	rc, err := image.RegistryConfigFromDockerConfig(dc, "HTTPS", metaConfig.DeckhouseConfig.ImagesRepo)
	require.NoError(t, err)
	require.NotNil(t, rc)
}

func TestApplyRegistryToDeckhouseConfigKeepsExisting(t *testing.T) {
	metaConfig := &MetaConfig{}
	metaConfig.DeckhouseConfig.RegistryDockerCfg = "preset"
	require.NoError(t, applyRegistryToDeckhouseConfig(metaConfig, []string{ensureRegistryMCDoc}))
	require.Equal(t, "preset", metaConfig.DeckhouseConfig.RegistryDockerCfg, "must not clobber dockercfg already supplied by configData")
}

func TestUnpackProviderBundleFailedDownloadLeavesNoDigestDir(t *testing.T) {
	globalOptions := ensureTestGlobalOptions(t)
	const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestDir := providerdir.ProviderDigestDir(globalOptions.DownloadDir, "enspartial", digest)

	fail := true
	orig := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, _, dest, _ string, _ image.RegistryConfig, _ bool) error {
		if fail {
			// Mimic the real puller: the destination is created and partially
			// filled before the download aborts.
			require.NoError(t, os.MkdirAll(dest, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dest, "garbage"), []byte("x"), 0o644))
			return fmt.Errorf("network reset")
		}
		writeTestProviderSchema(t, dest, "EnsPartialConfiguration")
		return nil
	}

	t.Cleanup(func() { downloadProviderBundle = orig })

	conf, err := image.NewRegistryConfig("HTTPS", "r.example.com/test", "", "", "")
	require.NoError(t, err)

	require.Error(t, unpackProviderBundle(context.Background(), "enspartial", providerBundleRef{Digest: digest}, conf, globalOptions))
	_, statErr := os.Stat(digestDir)
	require.True(t, os.IsNotExist(statErr), "failed download must not leave a digest dir that poisons the cache")
	_, statErr = os.Stat(digestDir + ".partial")
	require.True(t, os.IsNotExist(statErr), "failed download must not leave a partial dir")

	fail = false

	require.NoError(t, unpackProviderBundle(context.Background(), "enspartial", providerBundleRef{Digest: digest}, conf, globalOptions))
	_, statErr = os.Stat(filepath.Join(digestDir, "openapi"))
	require.NoError(t, statErr, "retry after failure must deliver the bundle")
}

func TestEnsureProviderBundleUsesResolvedImageReference(t *testing.T) {
	// An external module publishes the bundle under <repo>/<module>, so the digest must be
	// pulled from there and not from the flat images repo.
	const digest = "sha256:ensmodule"
	orig := resolveProviderBundleRef
	resolveProviderBundleRef = func(_ context.Context, _ string, _ providerModuleLookup, _ *options.GlobalOptions) (providerBundleRef, error) {
		conf, err := image.NewRegistryConfig("HTTPS", "modules.example.com/modules", "", "", "")
		require.NoError(t, err)
		return providerBundleRef{
			Image:    "modules.example.com/modules/cloud-provider-ensmod@" + digest,
			Digest:   digest,
			Registry: conf,
		}, nil
	}

	t.Cleanup(func() { resolveProviderBundleRef = orig })

	var gotImgName, gotRegistry string
	origDownload := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, imgName, dest, _ string, conf image.RegistryConfig, _ bool) error {
		gotImgName = imgName
		gotRegistry = conf.GetRegistry()

		writeTestProviderSchema(t, dest, "EnsModConfiguration")
		return nil
	}

	t.Cleanup(func() { downloadProviderBundle = origDownload })

	globalOptions := ensureTestGlobalOptions(t)
	docs := []string{ensureClusterConfigDoc("EnsMod"), ensureRegistryMCDoc}
	require.NoError(t, EnsureProviderBundle(context.Background(), "", docs, globalOptions))

	require.Equal(t, "modules.example.com/modules/cloud-provider-ensmod@"+digest, gotImgName)
	require.Equal(t, "modules.example.com/modules", gotRegistry, "the ModuleSource registry must win over the cluster one")
	// The on-disk cache still keys on the digest alone.
	_, err := os.Stat(providerdir.ProviderDigestDir(globalOptions.DownloadDir, "ensmod", digest))
	require.NoError(t, err)
}

// An explicit opt-in has to reach the resolver even for a provider whose validator is compiled
// into dhctl: for those (inTreeValidatorProviders - yandex and vcd) providerCandiPresent answers
// true on the candi schemas alone, and returning there would leave dhctl validating the
// configuration against those schemas while cluster-bootstrapper installs the module build the
// very same config.yml pinned.
func TestEnsureProviderBundlePinnedModuleReachesResolver(t *testing.T) {
	for _, tc := range []struct {
		name     string
		doc      string
		digest   string
		resolved bool
	}{
		{name: "no opt-in", digest: "sha256:enspinnednone"},
		{name: "spec.source", digest: "sha256:enspinnedsource", resolved: true, doc: `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vcd
spec:
  enabled: true
  version: 1
  source: deckhouse
`},
		{name: "ModulePullOverride", digest: "sha256:enspinnedoverride", resolved: true, doc: `
apiVersion: deckhouse.io/v1alpha2
kind: ModulePullOverride
metadata:
  name: cloud-provider-vcd
spec:
  imageTag: mr1
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			globalOptions := ensureTestGlobalOptions(t)
			writeTestProviderSchema(t, filepath.Join(globalOptions.CandiDir, "cloud-providers", "vcd"), "VCDClusterConfiguration")

			var resolveCalls, downloadCalls atomic.Int32
			stubProviderDigest(t, tc.digest, &resolveCalls)
			stubProviderDownload(t, "VCDClusterConfiguration", 0, &downloadCalls)

			docs := []string{ensureRegistryMCDoc, ensureClusterConfigDoc("VCD")}
			if tc.doc != "" {
				docs = append(docs, tc.doc)
			}

			require.NoError(t, EnsureProviderBundle(context.Background(), "", docs, globalOptions))

			if !tc.resolved {
				require.Zero(t, resolveCalls.Load(), "candi already carries the schemas and nothing pins another build")
				require.Zero(t, downloadCalls.Load())
				return
			}

			require.Equal(t, int32(1), resolveCalls.Load())
			require.Equal(t, int32(1), downloadCalls.Load())
			_, err := os.Stat(providerdir.ProviderDigestDir(globalOptions.DownloadDir, "vcd", tc.digest))
			require.NoError(t, err)
		})
	}
}

// Resolving the bundle reference now asks the cluster which module the provider came from, so
// the providerCandiPresent early-return is the only thing left keeping EnsureExternalProviderBundle's
// promise that a bundle already on disk never dials the kube API - the promise destroy relies on
// when it is served entirely from the local state cache.
func TestEnsureExternalProviderBundleNeverDialsForDeliveredBundle(t *testing.T) {
	globalOptions := ensureTestGlobalOptions(t)

	bundleDir := filepath.Join(globalOptions.DownloadDir, "ensnodial")
	writeTestProviderSchema(t, bundleDir, "EnsNoDialConfiguration")
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "validator"), []byte("#!/bin/sh\n"), 0o755))

	getter := func(context.Context) (*client.KubernetesClient, error) {
		t.Error("the kube API must not be dialed for an already-delivered bundle")
		return nil, fmt.Errorf("dialed")
	}

	require.NoError(t, EnsureExternalProviderBundle(context.Background(), getter, ensureClusterConfigDoc("EnsNoDial"), globalOptions))
}

// Which module build a cluster runs is a property of that cluster, and this installer's own
// contents say nothing about it: an operator can drive a cluster past the migration with a dhctl
// that still ships the module, or the other way round. So the modules directory must not be used
// to decide the cluster has nothing to say - only a bundle already on disk skips the resolve.
func TestEnsureExternalProviderBundleResolvesFromTheClusterEvenForAShippedModule(t *testing.T) {
	const (
		digest   = "sha256:ensdial"
		provider = "ensshipped"
	)

	globalOptions := ensureTestGlobalOptions(t)
	stubEmbeddedDigests(t, `{"cloudProviderEnsshipped": {"terraformManager": "`+digest+`"}}`)
	require.NoError(t, os.MkdirAll(filepath.Join(globalOptions.ModulesDir, "030-"+CloudProviderModuleName(provider)), 0o755))

	// A warm dhctl-server: the bundle is unpacked and loaded into this process, and the
	// validator that would satisfy providerCandiPresent is not on disk.
	digestDir := providerdir.ProviderDigestDir(globalOptions.DownloadDir, provider, digest)
	writeTestProviderSchema(t, digestDir, "EnsDialClusterConfiguration")
	require.NoError(t, switchProviderSymlink(providerdir.ProviderDir(globalOptions.DownloadDir, provider), digestDir))
	require.NoError(t, NewSchemaStore(globalOptions).LoadProviderDir(provider, digest, digestDir))

	var dialed atomic.Bool
	getter := func(context.Context) (*client.KubernetesClient, error) {
		dialed.Store(true)
		return nil, fmt.Errorf("dialed")
	}

	err := EnsureExternalProviderBundle(context.Background(), getter, ensureClusterConfigDoc(provider), globalOptions)
	require.ErrorContains(t, err, "dialed")
	require.True(t, dialed.Load(), "the module shipping in this image must not stand in for the cluster's answer")
}
