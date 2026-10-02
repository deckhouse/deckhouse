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
	"errors"
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

func stubProviderDigestError(t *testing.T, err error) {
	t.Helper()
	orig := resolveProviderBundleRef
	resolveProviderBundleRef = func(context.Context, string, providerModuleLookup, *options.GlobalOptions) (providerBundleRef, error) {
		return providerBundleRef{}, err
	}

	t.Cleanup(func() { resolveProviderBundleRef = orig })
}

// writeDeliveredBundle lays out a previously downloaded bundle the way a real run leaves it: a
// digest dir with the schema, and the default alias pointing at it.
func writeDeliveredBundle(t *testing.T, downloadDir, provider, digest, kind string) {
	t.Helper()
	digestDir := providerdir.ProviderDigestDir(downloadDir, provider, digest)
	writeTestProviderSchema(t, digestDir, kind)
	require.NoError(t, os.Symlink(digestDir, providerdir.ProviderDir(downloadDir, provider)))
}

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

func TestProviderBundleEmptyProviderIsInternal(t *testing.T) {
	require.True(t, IsInternalCloudProviderBundle("", &options.GlobalOptions{CandiDir: t.TempDir()}))
}

// EnsureProviderBundle serves bootstrap and the commander data parse, neither of which may run on
// a bundle left behind by another installation, so a delivered one is no excuse for a failed
// resolve here.
func TestEnsureProviderBundleDoesNotFallBackToDeliveredBundle(t *testing.T) {
	stubProviderDigestError(t, errors.New("ensfallback registry unreachable"))

	globalOptions := ensureTestGlobalOptions(t)
	writeDeliveredBundle(t, globalOptions.DownloadDir, "ensfallback", "sha256:ensfallback", "EnsFallbackConfiguration")

	err := EnsureProviderBundleFromConfig(context.Background(), "ensfallback", nil, globalOptions)
	require.ErrorContains(t, err, "ensfallback registry unreachable")
}

func TestEnsureProviderBundleReturnsCauseWithoutDeliveredBundle(t *testing.T) {
	stubProviderDigestError(t, errors.New("ensnofallback registry unreachable"))

	err := EnsureProviderBundleFromConfig(context.Background(), "ensnofallback", nil, ensureTestGlobalOptions(t))
	require.ErrorContains(t, err, "ensnofallback registry unreachable")
}

func TestEnsureProviderBundleMovesTheAliasOntoTheResolvedDigest(t *testing.T) {
	const (
		provider = "ensstale"
		wanted   = "sha256:ensstalenew"
	)

	globalOptions := ensureTestGlobalOptions(t)

	// This process already validates the provider against the digest the resolver returns, but the
	// alias — the path the validator binary and the opentofu plugin are taken from — still leads to
	// the previous bundle.
	wantedDir := providerdir.ProviderDigestDir(globalOptions.DownloadDir, provider, wanted)
	writeTestProviderSchema(t, wantedDir, "EnsStaleConfiguration")
	require.NoError(t, NewSchemaStore(globalOptions).LoadProviderDir(provider, wanted, wantedDir))
	writeDeliveredBundle(t, globalOptions.DownloadDir, provider, "sha256:ensstaleold", "EnsStaleOldConfiguration")

	stubProviderDigest(t, wanted, nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsStaleConfiguration", 0, &downloads)

	require.NoError(t, EnsureProviderBundleFromConfig(context.Background(), provider, nil, globalOptions))

	dir, digest, ok := providerdir.Delivered(globalOptions.DownloadDir, provider)
	require.True(t, ok)
	require.Equal(t, wanted, digest)
	require.Equal(t, filepath.Base(wantedDir), filepath.Base(dir))
	require.Zero(t, downloads.Load(), "the resolved digest was already unpacked, only the alias was stale")
}

// An alias leading to a directory without the schema is not a bundle to fall back to, so even
// destroy surfaces the original failure instead of carrying on with nothing.
func TestEnsureProviderBundleIgnoresAliasWithoutSchema(t *testing.T) {
	stubProviderDigestError(t, errors.New("ensemptyalias registry unreachable"))

	globalOptions := ensureTestGlobalOptions(t)
	digestDir := providerdir.ProviderDigestDir(globalOptions.DownloadDir, "ensemptyalias", "sha256:ensemptyalias")
	require.NoError(t, os.MkdirAll(digestDir, 0o755))
	require.NoError(t, os.Symlink(digestDir, providerdir.ProviderDir(globalOptions.DownloadDir, "ensemptyalias")))

	getter := func(context.Context) (*client.KubernetesClient, error) {
		return nil, fmt.Errorf("ensemptyalias registry unreachable")
	}

	err := EnsureProviderBundleFromCluster(
		context.Background(), getter, ensureClusterConfigDoc("ensemptyalias"), globalOptions, "destroy")
	require.ErrorContains(t, err, "ensemptyalias registry unreachable")
}

func TestEnsureProviderBundleStaticNoop(t *testing.T) {
	var digestCalls atomic.Int32
	stubProviderDigest(t, "sha256:unused", &digestCalls)

	docs := []string{`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
`}
	require.NoError(t, EnsureProviderBundleFromConfig(context.Background(), "", docs, ensureTestGlobalOptions(t)))
	require.Zero(t, digestCalls.Load(), "static cluster must not resolve provider digest")
}

func TestEnsureProviderBundleInTreeNoop(t *testing.T) {
	for _, provider := range []string{"VCD", "AWS"} {
		t.Run(provider, func(t *testing.T) {
			var digestCalls atomic.Int32
			stubProviderDigest(t, "sha256:unused", &digestCalls)

			globalOptions := ensureTestGlobalOptions(t)
			schemaPath := filepath.Join(globalOptions.CandiDir, "cloud-providers", strings.ToLower(provider), "openapi")
			require.NoError(t, os.MkdirAll(schemaPath, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(schemaPath, "cluster_configuration.yaml"), []byte("kind: X\napiVersions: []\n"), 0o644))

			require.NoError(t, EnsureProviderBundleFromConfig(context.Background(), "", []string{ensureClusterConfigDoc(provider)}, globalOptions))
			require.NoError(t, EnsureProviderBundleFromConfig(context.Background(), provider, nil, globalOptions), "explicit provider must hit the same no-op")
			require.Zero(t, digestCalls.Load(), "in-tree provider with bundled candi must not resolve digest")
		})
	}
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

	err := EnsureProviderBundleFromConfig(context.Background(), "", []string{ensureClusterConfigDoc("EnsNoReg")}, ensureTestGlobalOptions(t))
	require.NoError(t, err)
	require.Equal(t, "registry.deckhouse.io/deckhouse/ce@sha256:noreg", gotImgName)
}

func TestEnsureProviderBundleDownloadsLoadsAndCaches(t *testing.T) {
	stubProviderDigest(t, "sha256:enstest1", nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsTestConfiguration", 0, &downloads)

	globalOptions := ensureTestGlobalOptions(t)
	docs := []string{ensureClusterConfigDoc("EnsTest"), ensureRegistryMCDoc}

	require.NoError(t, EnsureProviderBundleFromConfig(context.Background(), "", docs, globalOptions))
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
	require.NoError(t, EnsureProviderBundleFromConfig(context.Background(), "", docs, globalOptions))
	require.Equal(t, int32(1), downloads.Load())
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
			errs[n] = EnsureProviderBundleFromConfig(context.Background(), "", docs, globalOptions)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), downloads.Load(), "concurrent calls must share one download")
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
	registry := func(context.Context) (*image.RegistryConfig, error) { return conf, nil }

	require.Error(t, unpackProviderBundle(context.Background(), "enspartial", providerBundleRef{Digest: digest}, registry, globalOptions))
	_, statErr := os.Stat(digestDir)
	require.True(t, os.IsNotExist(statErr), "failed download must not leave a digest dir that poisons the cache")
	_, statErr = os.Stat(digestDir + ".partial")
	require.True(t, os.IsNotExist(statErr), "failed download must not leave a partial dir")

	fail = false

	require.NoError(t, unpackProviderBundle(context.Background(), "enspartial", providerBundleRef{Digest: digest}, registry, globalOptions))
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
	require.NoError(t, EnsureProviderBundleFromConfig(context.Background(), "", docs, globalOptions))

	require.Equal(t, "modules.example.com/modules/cloud-provider-ensmod@"+digest, gotImgName)
	require.Equal(t, "modules.example.com/modules", gotRegistry, "the ModuleSource registry must win over the cluster one")
	// The on-disk cache still keys on the digest alone.
	_, err := os.Stat(providerdir.ProviderDigestDir(globalOptions.DownloadDir, "ensmod", digest))
	require.NoError(t, err)
}

// An explicit opt-in has to reach the resolver even for a provider whose schemas ship in candi:
// returning on those alone would leave dhctl validating the configuration against them while
// cluster-bootstrapper installs the module build the very same config.yml pinned.
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

			require.NoError(t, EnsureProviderBundleFromConfig(context.Background(), "", docs, globalOptions))

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

// Destroy is served entirely from the local state cache when the cluster is already gone, so an
// unreachable kube API must leave it running on the bundle under the provider's alias.
func TestEnsureExternalProviderBundleDestroyFallsBackToDeliveredBundle(t *testing.T) {
	globalOptions := ensureTestGlobalOptions(t)
	writeDeliveredBundle(t, globalOptions.DownloadDir, "ensnodial", "sha256:ensnodial", "EnsNoDialConfiguration")

	getter := func(context.Context) (*client.KubernetesClient, error) {
		return nil, fmt.Errorf("dialed")
	}

	require.NoError(t, EnsureProviderBundleFromCluster(
		context.Background(), getter, ensureClusterConfigDoc("EnsNoDial"), globalOptions, "destroy"))

	require.NotNil(t,
		NewSchemaStore(globalOptions).Get(&SchemaIndex{Kind: "EnsNoDialConfiguration", Version: "deckhouse.io/v1"}),
		"schemas must be loaded from the bundle under the default alias",
	)
}

// Bootstrap shares its download dir with every other installation made from this machine, so a
// bundle found there belongs to another cluster and must not stand in for the one being installed.
func TestEnsureExternalProviderBundleBootstrapDoesNotFallBack(t *testing.T) {
	globalOptions := ensureTestGlobalOptions(t)
	writeDeliveredBundle(t, globalOptions.DownloadDir, "ensnofall", "sha256:ensnofall", "EnsNoFallConfiguration")

	getter := func(context.Context) (*client.KubernetesClient, error) {
		return nil, fmt.Errorf("dialed")
	}

	err := EnsureProviderBundleFromCluster(
		context.Background(), getter, ensureClusterConfigDoc("EnsNoFall"), globalOptions, "bootstrap")
	require.ErrorContains(t, err, "dialed")
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

	// A warm dhctl-server: the bundle is unpacked and loaded into this process.
	digestDir := providerdir.ProviderDigestDir(globalOptions.DownloadDir, provider, digest)
	writeTestProviderSchema(t, digestDir, "EnsDialClusterConfiguration")
	require.NoError(t, switchProviderSymlink(providerdir.ProviderDir(globalOptions.DownloadDir, provider), digestDir))
	require.NoError(t, NewSchemaStore(globalOptions).LoadProviderDir(provider, digest, digestDir))

	var dialed atomic.Bool
	getter := func(context.Context) (*client.KubernetesClient, error) {
		dialed.Store(true)
		return nil, fmt.Errorf("dialed")
	}

	// The dial itself is what this test is about. Its failure is no longer fatal: the bundle
	// already under the provider's alias carries the operation on.
	require.NoError(t, EnsureProviderBundleFromCluster(context.Background(), getter, ensureClusterConfigDoc(provider), globalOptions, "destroy"))
	require.True(t, dialed.Load(), "the module shipping in this image must not stand in for the cluster's answer")
}

// EnsureExternalProviderBundle must not touch the cluster for providers that
// need no downloaded bundle: a static cluster (no provider) and an in-tree
// provider whose schemas ship in candi. A nil kube client would panic in
// GetRegistryData, so a nil error proves the early exit before any cluster read.
func TestEnsureExternalProviderBundleSkipsClusterRead(t *testing.T) {
	staticCluster := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
`
	err := EnsureProviderBundleFromCluster(t.Context(), nil, staticCluster, &options.GlobalOptions{DownloadDir: t.TempDir()}, "destroy")
	require.NoError(t, err)

	candiDir := t.TempDir()
	schemaPath := filepath.Join(candiDir, "cloud-providers", "vcd", "openapi", "cluster_configuration.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(schemaPath), 0o755))
	require.NoError(t, os.WriteFile(schemaPath, []byte("type: object\n"), 0o644))

	vcdCluster := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
cloud:
  provider: VCD
  prefix: test
`
	err = EnsureProviderBundleFromCluster(t.Context(), nil, vcdCluster, &options.GlobalOptions{DownloadDir: t.TempDir(), CandiDir: candiDir}, "destroy")
	require.NoError(t, err)
}

func TestEnsureExternalProviderBundleFallsBackWhenTheRegistryIsUnreachable(t *testing.T) {
	const provider = "ensnoreg2"

	stubProviderDigest(t, "sha256:ensnoreg2", nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsNoReg2Configuration", 0, &downloads)

	globalOptions := ensureTestGlobalOptions(t)
	writeDeliveredBundle(t, globalOptions.DownloadDir, provider, "sha256:ensnoreg2old", "EnsNoReg2OldConfiguration")

	getter := func(context.Context) (*client.KubernetesClient, error) {
		return nil, fmt.Errorf("registry unreachable")
	}

	require.NoError(t, EnsureProviderBundleFromCluster(
		context.Background(), getter, ensureClusterConfigDoc(provider), globalOptions, "destroy"))
	require.Zero(t, downloads.Load(), "a registry that cannot be read leaves nothing to download")

	require.NotNil(t,
		NewSchemaStore(globalOptions).Get(&SchemaIndex{Kind: "EnsNoReg2OldConfiguration", Version: "deckhouse.io/v1"}),
		"schemas must come from the bundle under the default alias",
	)
}

func TestEnsureExternalProviderBundleFallsBackWhenTheDownloadFails(t *testing.T) {
	const (
		provider = "ensdlfail"
		old      = "sha256:ensdlfailold"
	)

	conf, err := image.NewRegistryConfig("HTTPS", "r.example.com/test", "u", "p", "")
	require.NoError(t, err)

	origDigest := resolveProviderBundleRef
	resolveProviderBundleRef = func(context.Context, string, providerModuleLookup, *options.GlobalOptions) (providerBundleRef, error) {
		return providerBundleRef{Digest: "sha256:ensdlfailnew", Registry: conf}, nil
	}
	t.Cleanup(func() { resolveProviderBundleRef = origDigest })

	origDownload := downloadProviderBundle
	downloadProviderBundle = func(context.Context, string, string, string, image.RegistryConfig, bool) error {
		return fmt.Errorf("download refused")
	}
	t.Cleanup(func() { downloadProviderBundle = origDownload })

	globalOptions := ensureTestGlobalOptions(t)
	writeDeliveredBundle(t, globalOptions.DownloadDir, provider, old, "EnsDlFailConfiguration")

	require.NoError(t, EnsureProviderBundleFromCluster(
		context.Background(), nil, ensureClusterConfigDoc(provider), globalOptions, "destroy"))

	_, digest, ok := providerdir.Delivered(globalOptions.DownloadDir, provider)
	require.True(t, ok)
	require.Equal(t, old, digest, "a failed download must leave the alias where it was")
}

func TestEnsureExternalProviderBundleKeepsOldAliasWhenDownloadedBundleHasNoSchema(t *testing.T) {
	const (
		provider = "ensmissingschema"
		old      = "sha256:ensmissingschemaold"
	)

	conf, err := image.NewRegistryConfig("HTTPS", "r.example.com/test", "u", "p", "")
	require.NoError(t, err)

	origDigest := resolveProviderBundleRef
	resolveProviderBundleRef = func(context.Context, string, providerModuleLookup, *options.GlobalOptions) (providerBundleRef, error) {
		return providerBundleRef{Digest: "sha256:ensmissingschemanew", Registry: conf}, nil
	}
	t.Cleanup(func() { resolveProviderBundleRef = origDigest })

	origDownload := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, _ string, dest string, _ string, _ image.RegistryConfig, _ bool) error {
		return os.MkdirAll(dest, 0o755)
	}
	t.Cleanup(func() { downloadProviderBundle = origDownload })

	globalOptions := ensureTestGlobalOptions(t)
	writeDeliveredBundle(t, globalOptions.DownloadDir, provider, old, "EnsMissingSchemaOldConfiguration")

	require.NoError(t, EnsureProviderBundleFromCluster(
		context.Background(), nil, ensureClusterConfigDoc(provider), globalOptions, "destroy"))

	_, digest, ok := providerdir.Delivered(globalOptions.DownloadDir, provider)
	require.True(t, ok)
	require.Equal(t, old, digest, "an invalid new bundle must not replace the working alias")
}

func TestEnsureProviderBundleDownloadsTheNewDigest(t *testing.T) {
	const (
		provider = "ensnewdigest"
		wanted   = "sha256:ensnewdigestnew"
	)

	globalOptions := ensureTestGlobalOptions(t)
	writeDeliveredBundle(t, globalOptions.DownloadDir, provider, "sha256:ensnewdigestold", "EnsNewDigestOldConfiguration")

	stubProviderDigest(t, wanted, nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsNewDigestConfiguration", 0, &downloads)

	require.NoError(t, EnsureProviderBundleFromConfig(context.Background(), provider, nil, globalOptions))
	require.Equal(t, int32(1), downloads.Load(), "a digest with no directory of its own must be downloaded")

	_, digest, ok := providerdir.Delivered(globalOptions.DownloadDir, provider)
	require.True(t, ok)
	require.Equal(t, wanted, digest)

	require.NotNil(t,
		NewSchemaStore(globalOptions).Get(&SchemaIndex{Kind: "EnsNewDigestConfiguration", Version: "deckhouse.io/v1"}),
		"the new digest's schemas must replace the ones loaded before",
	)
}

func TestEnsureProviderSchemasSkipsTheResolveForADeliveredBundle(t *testing.T) {
	const provider = "ensschemas"

	var digestCalls atomic.Int32
	stubProviderDigest(t, "sha256:unused", &digestCalls)

	globalOptions := ensureTestGlobalOptions(t)
	writeDeliveredBundle(t, globalOptions.DownloadDir, provider, "sha256:ensschemas", "EnsSchemasConfiguration")

	require.NoError(t, ensureProviderSchemas(context.Background(), provider, nil, globalOptions))
	require.Zero(t, digestCalls.Load(), "whoever delivered the bundle chose the build; this parse must not choose another")

	require.NotNil(t,
		NewSchemaStore(globalOptions).Get(&SchemaIndex{Kind: "EnsSchemasConfiguration", Version: "deckhouse.io/v1"}),
	)
}

func TestEnsureProviderSchemasFailsWhenNothingIsDelivered(t *testing.T) {
	stubProviderDigestError(t, errors.New("ensnoschemas registry unreachable"))

	err := ensureProviderSchemas(context.Background(), "ensnoschemas", nil, ensureTestGlobalOptions(t))
	require.ErrorContains(t, err, "ensnoschemas registry unreachable")
}

// ProviderRequiresClusterConfig asks whether a section is mandatory, not whether a bundle has to be
// downloaded, and an empty provider has no section to require.
func TestProviderRequiresClusterConfigIsFalseWithoutAProvider(t *testing.T) {
	require.False(t, ProviderRequiresClusterConfig(""))
}

// A digest already unpacked is not going to be downloaded again, so nothing may ask for a registry
// on its account — on the strict path that question is fatal, and reading it costs a cluster call.
func TestUpdateProviderBundleSkipsTheRegistryForAnUnpackedDigest(t *testing.T) {
	const (
		provider = "ensnoregread"
		digest   = "sha256:ensnoregread"
	)

	stubProviderDigest(t, digest, nil)

	globalOptions := ensureTestGlobalOptions(t)
	writeTestProviderSchema(t, providerdir.ProviderDigestDir(globalOptions.DownloadDir, provider, digest), "EnsNoRegReadConfiguration")

	var asked atomic.Int32
	registry := func(context.Context) (*image.RegistryConfig, error) {
		asked.Add(1)
		return nil, fmt.Errorf("registry unreachable")
	}

	require.NoError(t, updateProviderBundle(context.Background(), provider, nil, registry, globalOptions, true))
	require.Zero(t, asked.Load(), "the digest was already on disk; the registry must not be read")
}
