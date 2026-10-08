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
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
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

// writeUnpackedBundle lays out a bundle the way an earlier run leaves it: a digest dir with the schema.
func writeUnpackedBundle(t *testing.T, downloadDir, provider, digest, kind string) string {
	t.Helper()
	dir := providerdir.DigestDir(downloadDir, provider, digest)
	writeTestProviderSchema(t, dir, kind)

	return dir
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
		CandiDir:    t.TempDir(),
		ModulesDir:  t.TempDir(),
		DownloadDir: downloadDir,
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
	downloadProviderBundle = func(_ context.Context, _, dest string, _ image.RegistryConfig, _ bool) error {
		calls.Add(1)
		time.Sleep(delay)
		writeTestProviderSchema(t, dest, kind)
		return nil
	}

	t.Cleanup(func() { downloadProviderBundle = orig })
}

func TestCNIBootstrapPathReadsTheBundleOfItsOwnOperation(t *testing.T) {
	globalOptions := ensureTestGlobalOptions(t)

	for _, bundleDir := range []string{"/dl/dvp@sha256:aaa", "/dl/dvp@sha256:bbb"} {
		m := &MetaConfig{ProviderName: "dvp", DownloadRootDir: "/dl", ProviderBundleDir: bundleDir}
		require.Equal(t, filepath.Join(bundleDir, cniBootstrapFileName), cniBootstrapPath(m, globalOptions))
	}
}

// Without a prepared bundle the file comes from candi. An alias left by an older dhctl is not read.
func TestCNIBootstrapPathWithoutABundleIgnoresTheAliasOfAnOlderDhctl(t *testing.T) {
	globalOptions := ensureTestGlobalOptions(t)
	alias := filepath.Join(globalOptions.DownloadDir, "dvp")
	require.NoError(t, os.MkdirAll(alias, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alias, cniBootstrapFileName), []byte("schemaVersion: 1\n"), 0o644))

	m := &MetaConfig{ProviderName: "dvp", DownloadRootDir: globalOptions.DownloadDir}
	require.Equal(t, filepath.Join(globalOptions.CandiDir, "cloud-providers", "dvp", cniBootstrapFileName), cniBootstrapPath(m, globalOptions))
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
	writeUnpackedBundle(t, globalOptions.DownloadDir, "ensfallback", "sha256:ensfallback", "EnsFallbackConfiguration")

	_, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc("ensfallback")}, globalOptions)
	require.ErrorContains(t, err, "ensfallback registry unreachable")
}

func TestEnsureProviderBundleReturnsCauseWithoutDeliveredBundle(t *testing.T) {
	stubProviderDigestError(t, errors.New("ensnofallback registry unreachable"))

	_, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc("ensnofallback")}, ensureTestGlobalOptions(t))
	require.ErrorContains(t, err, "ensnofallback registry unreachable")
}

// An earlier run left the resolved digest and an older one on disk. The resolved one is used as it
// lies, with no download, and the older one is not.
func TestEnsureProviderBundleReturnsAnUnpackedDigestWithoutDownload(t *testing.T) {
	const (
		provider = "ensstale"
		wanted   = "sha256:ensstalenew"
	)

	globalOptions := ensureTestGlobalOptions(t)
	wantedDir := writeUnpackedBundle(t, globalOptions.DownloadDir, provider, wanted, "EnsStaleConfiguration")
	writeUnpackedBundle(t, globalOptions.DownloadDir, provider, "sha256:ensstaleold", "EnsStaleOldConfiguration")

	stubProviderDigest(t, wanted, nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsStaleConfiguration", 0, &downloads)

	dir, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc(provider)}, globalOptions)
	require.NoError(t, err)
	require.Equal(t, wantedDir, dir)
	require.True(t, NewSchemaStore(globalOptions).ProviderSchemasLoaded(provider, wanted))
	require.Zero(t, downloads.Load(), "the resolved digest was already unpacked")
}

func TestEnsureProviderBundleStaticNoop(t *testing.T) {
	var digestCalls atomic.Int32
	stubProviderDigest(t, "sha256:unused", &digestCalls)

	docs := []string{`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
`}
	_, err := ensureProviderBundleFromConfig(context.Background(), docs, ensureTestGlobalOptions(t))
	require.NoError(t, err)
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

			_, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc(provider)}, globalOptions)
			require.NoError(t, err)
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
	downloadProviderBundle = func(_ context.Context, imgName, dest string, _ image.RegistryConfig, _ bool) error {
		gotImgName = imgName

		writeTestProviderSchema(t, dest, "EnsNoRegConfiguration")
		return nil
	}

	t.Cleanup(func() { downloadProviderBundle = orig })

	_, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc("EnsNoReg")}, ensureTestGlobalOptions(t))
	require.NoError(t, err)
	require.Equal(t, "registry.deckhouse.io/deckhouse/ce@sha256:noreg", gotImgName)
}

func TestEnsureProviderBundleDownloadsLoadsAndCaches(t *testing.T) {
	stubProviderDigest(t, "sha256:enstest1", nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsTestConfiguration", 0, &downloads)

	globalOptions := ensureTestGlobalOptions(t)
	docs := []string{ensureClusterConfigDoc("EnsTest"), ensureRegistryMCDoc}

	dir, err := ensureProviderBundleFromConfig(context.Background(), docs, globalOptions)
	require.NoError(t, err)
	require.Equal(t, int32(1), downloads.Load())
	require.FileExists(t, providerdir.SchemaPath(dir))

	schemaStore := NewSchemaStore(globalOptions)
	require.True(t, schemaStore.ProviderSchemasLoaded("enstest", "sha256:enstest1"))
	require.NotNil(t, schemaStore.Get(&SchemaIndex{Kind: "EnsTestConfiguration", Version: "deckhouse.io/v1"}))

	// Warm path: no second download.
	_, err = ensureProviderBundleFromConfig(context.Background(), docs, globalOptions)
	require.NoError(t, err)
	require.Equal(t, int32(1), downloads.Load())
}

func TestEnsureProviderBundleConcurrentCallsShareOneDownload(t *testing.T) {
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
			_, errs[n] = ensureProviderBundleFromConfig(context.Background(), docs, globalOptions)
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
	digestDir := providerdir.DigestDir(globalOptions.DownloadDir, "enspartial", digest)

	fail := true
	orig := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, _, dest string, _ image.RegistryConfig, _ bool) error {
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

	_, err = unpackProviderBundle(context.Background(), "enspartial", providerBundleRef{Digest: digest}, registry, globalOptions)
	require.ErrorContains(t, err, "Make that reference reachable", "a failed pull points the operator at the registry")
	_, statErr := os.Stat(digestDir)
	require.True(t, os.IsNotExist(statErr), "failed download must not leave a digest dir that poisons the cache")
	matches, err := filepath.Glob(digestDir + ".partial-*")
	require.NoError(t, err)
	require.Empty(t, matches, "failed download must not leave a partial dir")

	fail = false

	_, err = unpackProviderBundle(context.Background(), "enspartial", providerBundleRef{Digest: digest}, registry, globalOptions)
	require.NoError(t, err)
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
	downloadProviderBundle = func(_ context.Context, imgName, dest string, conf image.RegistryConfig, _ bool) error {
		gotImgName = imgName
		gotRegistry = conf.GetRegistry()

		writeTestProviderSchema(t, dest, "EnsModConfiguration")
		return nil
	}

	t.Cleanup(func() { downloadProviderBundle = origDownload })

	globalOptions := ensureTestGlobalOptions(t)
	docs := []string{ensureClusterConfigDoc("EnsMod"), ensureRegistryMCDoc}
	_, err := ensureProviderBundleFromConfig(context.Background(), docs, globalOptions)
	require.NoError(t, err)

	require.Equal(t, "modules.example.com/modules/cloud-provider-ensmod@"+digest, gotImgName)
	require.Equal(t, "modules.example.com/modules", gotRegistry, "the ModuleSource registry must win over the cluster one")
	// The on-disk cache still keys on the digest alone.
	_, err = os.Stat(providerdir.DigestDir(globalOptions.DownloadDir, "ensmod", digest))
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

			_, err := ensureProviderBundleFromConfig(context.Background(), docs, globalOptions)
			require.NoError(t, err)

			if !tc.resolved {
				require.Zero(t, resolveCalls.Load(), "candi already carries the schemas and nothing pins another build")
				require.Zero(t, downloadCalls.Load())
				return
			}

			require.Equal(t, int32(1), resolveCalls.Load())
			require.Equal(t, int32(1), downloadCalls.Load())
			_, err = os.Stat(providerdir.DigestDir(globalOptions.DownloadDir, "vcd", tc.digest))
			require.NoError(t, err)
		})
	}
}

// The download dir is shared with every other cluster driven from this machine, so a bundle found
// there may belong to another cluster and must not stand in for the one this cluster runs.
func TestEnsureProviderBundleFromClusterReturnsTheLookupError(t *testing.T) {
	globalOptions := ensureTestGlobalOptions(t)
	writeUnpackedBundle(t, globalOptions.DownloadDir, "ensnofall", "sha256:ensnofall", "EnsNoFallConfiguration")

	getter := func(context.Context) (*client.KubernetesClient, error) {
		return nil, fmt.Errorf("dialed")
	}

	_, _, err := EnsureProviderBundleFromCluster(
		context.Background(), getter, ensureClusterConfigDoc("EnsNoFall"), globalOptions)
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
	digestDir := writeUnpackedBundle(t, globalOptions.DownloadDir, provider, digest, "EnsDialClusterConfiguration")
	require.NoError(t, NewSchemaStore(globalOptions).LoadProviderDir(provider, digest, digestDir))

	var dialed atomic.Bool
	getter := func(context.Context) (*client.KubernetesClient, error) {
		dialed.Store(true)
		return nil, fmt.Errorf("dialed")
	}

	// The dial itself is what this test is about.
	_, _, err := EnsureProviderBundleFromCluster(context.Background(), getter, ensureClusterConfigDoc(provider), globalOptions)
	require.ErrorContains(t, err, "dialed")
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
	_, _, err := EnsureProviderBundleFromCluster(t.Context(), nil, staticCluster, &options.GlobalOptions{DownloadDir: t.TempDir()})
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
	_, _, err = EnsureProviderBundleFromCluster(t.Context(), nil, vcdCluster, &options.GlobalOptions{DownloadDir: t.TempDir(), CandiDir: candiDir})
	require.NoError(t, err)
}

func TestEnsureProviderBundleFromClusterFailedDownloadIsAnError(t *testing.T) {
	const (
		provider = "ensdlfail"
		old      = "sha256:ensdlfailold"
		fresh    = "sha256:ensdlfailnew"
	)

	conf, err := image.NewRegistryConfig("HTTPS", "r.example.com/test", "u", "p", "")
	require.NoError(t, err)

	origDigest := resolveProviderBundleRef
	resolveProviderBundleRef = func(context.Context, string, providerModuleLookup, *options.GlobalOptions) (providerBundleRef, error) {
		return providerBundleRef{Digest: fresh, Registry: conf}, nil
	}
	t.Cleanup(func() { resolveProviderBundleRef = origDigest })

	origDownload := downloadProviderBundle
	downloadProviderBundle = func(context.Context, string, string, image.RegistryConfig, bool) error {
		return fmt.Errorf("download refused")
	}
	t.Cleanup(func() { downloadProviderBundle = origDownload })

	globalOptions := ensureTestGlobalOptions(t)
	writeUnpackedBundle(t, globalOptions.DownloadDir, provider, old, "EnsDlFailConfiguration")

	_, _, err = EnsureProviderBundleFromCluster(
		context.Background(), nil, ensureClusterConfigDoc(provider), globalOptions)
	require.ErrorContains(t, err, "download refused")

	require.DirExists(t, providerdir.DigestDir(globalOptions.DownloadDir, provider, old), "a failed download must leave the old digest in place")
	require.NoDirExists(t, providerdir.DigestDir(globalOptions.DownloadDir, provider, fresh), "a failed download must leave no dir of the new digest")
}

func TestEnsureExternalProviderBundleKeepsTheOldDigestWhenTheNewOneHasNoSchema(t *testing.T) {
	const (
		provider = "ensmissingschema"
		old      = "sha256:ensmissingschemaold"
		fresh    = "sha256:ensmissingschemanew"
	)

	conf, err := image.NewRegistryConfig("HTTPS", "r.example.com/test", "u", "p", "")
	require.NoError(t, err)

	origDigest := resolveProviderBundleRef
	resolveProviderBundleRef = func(context.Context, string, providerModuleLookup, *options.GlobalOptions) (providerBundleRef, error) {
		return providerBundleRef{Digest: fresh, Registry: conf}, nil
	}
	t.Cleanup(func() { resolveProviderBundleRef = origDigest })

	origDownload := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, _ string, dest string, _ image.RegistryConfig, _ bool) error {
		return os.MkdirAll(dest, 0o755)
	}
	t.Cleanup(func() { downloadProviderBundle = origDownload })

	globalOptions := ensureTestGlobalOptions(t)
	writeUnpackedBundle(t, globalOptions.DownloadDir, provider, old, "EnsMissingSchemaOldConfiguration")

	_, _, err = EnsureProviderBundleFromCluster(
		context.Background(), nil, ensureClusterConfigDoc(provider), globalOptions)
	require.ErrorIs(t, err, errBundleWithoutSchema)

	require.DirExists(t, providerdir.DigestDir(globalOptions.DownloadDir, provider, old), "an invalid new bundle must leave the old digest in place")
	require.NoDirExists(t, providerdir.DigestDir(globalOptions.DownloadDir, provider, fresh),
		"a bundle without the schema must not become visible under its digest")
}

func TestEnsureProviderBundleDownloadsTheNewDigest(t *testing.T) {
	const (
		provider = "ensnewdigest"
		wanted   = "sha256:ensnewdigestnew"
	)

	globalOptions := ensureTestGlobalOptions(t)
	writeUnpackedBundle(t, globalOptions.DownloadDir, provider, "sha256:ensnewdigestold", "EnsNewDigestOldConfiguration")

	stubProviderDigest(t, wanted, nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsNewDigestConfiguration", 0, &downloads)

	dir, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc(provider)}, globalOptions)
	require.NoError(t, err)
	require.Equal(t, int32(1), downloads.Load(), "a digest with no directory of its own must be downloaded")
	require.Equal(t, providerdir.DigestDir(globalOptions.DownloadDir, provider, wanted), dir)

	require.True(t, NewSchemaStore(globalOptions).ProviderSchemasLoaded(provider, wanted), "the new digest's schemas must be the loaded ones")
	require.NotNil(t, NewSchemaStore(globalOptions).Get(&SchemaIndex{Kind: "EnsNewDigestConfiguration", Version: "deckhouse.io/v1"}))
}

func TestEnsureProviderBundleReturnsTheDirOfTheResolvedDigest(t *testing.T) {
	const digest = "sha256:ensdir"

	stubProviderDigest(t, digest, nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsDirConfiguration", 0, &downloads)

	globalOptions := ensureTestGlobalOptions(t)
	dir, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc("ensdir")}, globalOptions)
	require.NoError(t, err)
	require.Equal(t, providerdir.DigestDir(globalOptions.DownloadDir, "ensdir", digest), dir)
	require.FileExists(t, providerdir.SchemaPath(dir))
}

// A digest directory that exists without the schema was not unpacked by dhctl. Parsing the
// provider configuration against nothing would accept anything, so it is an error.
func TestEnsureProviderBundleRejectsAnUnpackedDirWithoutSchema(t *testing.T) {
	const digest = "sha256:ensnoschema"

	stubProviderDigest(t, digest, nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsNoSchemaConfiguration", 0, &downloads)

	globalOptions := ensureTestGlobalOptions(t)
	dir := providerdir.DigestDir(globalOptions.DownloadDir, "ensnoschema", digest)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	_, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc("ensnoschema")}, globalOptions)
	require.ErrorIs(t, err, errBundleWithoutSchema)
	require.ErrorContains(t, err, "check cluster configuration schema in "+dir)
	require.Zero(t, downloads.Load())
}

// A bundle that was pulled but has no schema is no registry problem, so the registry hint stays off.
func TestEnsureProviderBundleRejectsADownloadedBundleWithoutSchema(t *testing.T) {
	stubProviderDigest(t, "sha256:ensnohint", nil)
	orig := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, _, dest string, _ image.RegistryConfig, _ bool) error {
		return os.MkdirAll(dest, 0o755)
	}
	t.Cleanup(func() { downloadProviderBundle = orig })

	globalOptions := ensureTestGlobalOptions(t)
	_, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc("ensnohint")}, globalOptions)
	require.ErrorIs(t, err, errBundleWithoutSchema)
	require.NotContains(t, err.Error(), "Make that reference reachable")
	// The staging dir it was checked in is gone by now, so the message names where the bundle goes.
	digestDir := providerdir.DigestDir(globalOptions.DownloadDir, "ensnohint", "sha256:ensnohint")
	require.ErrorContains(t, err, "check provider bundle "+digestDir+": ")
}

// The registry hint belongs to a failed pull. A download dir that cannot be written is no registry problem.
func TestUnpackProviderBundleGivesTheRegistryHintOnlyForAFailedPull(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes into a read-only directory")
	}

	stubProviderDigest(t, "sha256:ensnolock", nil)
	var downloads atomic.Int32
	stubProviderDownload(t, "EnsNoLockConfiguration", 0, &downloads)

	globalOptions := ensureTestGlobalOptions(t)
	require.NoError(t, os.Chmod(globalOptions.DownloadDir, 0o555))
	t.Cleanup(func() {
		if err := os.Chmod(globalOptions.DownloadDir, 0o755); err != nil {
			t.Errorf("make download dir writable again: %v", err)
		}
	})

	_, err := ensureProviderBundleFromConfig(context.Background(), []string{ensureClusterConfigDoc("ensnolock")}, globalOptions)
	require.ErrorContains(t, err, "create staging dir")
	require.NotContains(t, err.Error(), "Make that reference reachable")
	require.Zero(t, downloads.Load())
}

func TestParseConfigPreparesTheProviderBundle(t *testing.T) {
	const digest = "sha256:ensparse"
	stubProviderDigest(t, digest, nil)

	schema, err := os.ReadFile(providerdir.SchemaPath(tests.RequireProviderCandiDir(t, "yandex")))
	require.NoError(t, err)

	var downloads atomic.Int32
	orig := downloadProviderBundle
	downloadProviderBundle = func(_ context.Context, _, dest string, _ image.RegistryConfig, _ bool) error {
		downloads.Add(1)
		if err := os.MkdirAll(filepath.Dir(providerdir.SchemaPath(dest)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(providerdir.SchemaPath(dest), schema, 0o644)
	}
	t.Cleanup(func() { downloadProviderBundle = orig })

	globalOptions := &options.GlobalOptions{DownloadDir: t.TempDir()}
	metaConfig, err := ParseConfig(t.Context(), []string{writeConfig(t, yandexCompleteConfig)}, DummyValidatorProvider(), globalOptions)
	require.NoError(t, err)
	require.Equal(t, int32(1), downloads.Load())
	require.Equal(t, providerdir.DigestDir(globalOptions.DownloadDir, "yandex", digest), metaConfig.ProviderBundleDir)
}

func TestParseConfigFromDataEnsureProviderUsesTheBundleDirHandedIn(t *testing.T) {
	downloadDir := t.TempDir()
	bundleDir := tests.StubDeliveredProviderBundle(t, downloadDir, "yandex")
	globalOptions := &options.GlobalOptions{DownloadDir: downloadDir}
	// Whoever hands the dir in has loaded its schemas, the way commander's delivery does.
	require.NoError(t, NewSchemaStore(globalOptions).LoadProviderDir("yandex", tests.StubBundleDigest(), bundleDir))

	var digestCalls atomic.Int32
	stubProviderDigest(t, "sha256:enshanded", &digestCalls)

	metaConfig, err := ParseConfigFromDataEnsureProvider(t.Context(), yandexCompleteConfig, DummyValidatorProvider(), globalOptions, ValidateOptionProviderBundleDir(bundleDir))
	require.NoError(t, err)
	require.Equal(t, bundleDir, metaConfig.ProviderBundleDir)
	require.Zero(t, digestCalls.Load(), "a bundle dir was handed in, nothing may be resolved")
}

func TestUseUnpackedProviderBundle(t *testing.T) {
	const digest = "sha256:ensstate"
	globalOptions := ensureTestGlobalOptions(t)
	dir := providerdir.DigestDir(globalOptions.DownloadDir, "ensstate", digest)

	_, err := UseUnpackedProviderBundle(t.Context(), ensureClusterConfigDoc("EnsState"), digest, globalOptions)
	requireMissingBundleError(t, err, dir)

	writeTestProviderSchema(t, dir, "EnsStateConfiguration")

	got, err := UseUnpackedProviderBundle(t.Context(), ensureClusterConfigDoc("EnsState"), digest, globalOptions)
	require.NoError(t, err)
	require.Equal(t, dir, got)
	require.NotNil(t, NewSchemaStore(globalOptions).Get(&SchemaIndex{Kind: "EnsStateConfiguration", Version: "deckhouse.io/v1"}))
	require.True(t, NewSchemaStore(globalOptions).ProviderSchemasLoaded("ensstate", digest), "the schemas of this digest are loaded into this process")
}

func TestRestoreProviderBundle(t *testing.T) {
	globalOptions := ensureTestGlobalOptions(t)

	t.Run("a static cluster needs no bundle", func(t *testing.T) {
		usable, err := RestoreProviderBundle(t.Context(), &MetaConfig{}, globalOptions)
		require.NoError(t, err)
		require.True(t, usable)
	})

	t.Run("a MetaConfig saved by a dhctl that knew no bundle dir is not usable", func(t *testing.T) {
		usable, err := RestoreProviderBundle(t.Context(), &MetaConfig{ProviderName: "ensrestoreold"}, globalOptions)
		require.NoError(t, err)
		require.False(t, usable)
	})

	t.Run("a bundle that left the disk is an error naming its path", func(t *testing.T) {
		gone := providerdir.DigestDir(globalOptions.DownloadDir, "ensrestoregone", "sha256:gone")
		_, err := RestoreProviderBundle(t.Context(), &MetaConfig{ProviderName: "ensrestoregone", ProviderBundleDir: gone}, globalOptions)
		requireMissingBundleError(t, err, gone)
	})

	t.Run("a bundle on disk gets its schemas loaded", func(t *testing.T) {
		dir := providerdir.DigestDir(globalOptions.DownloadDir, "ensrestore", "sha256:ensrestore")
		writeTestProviderSchema(t, dir, "EnsRestoreConfiguration")

		usable, err := RestoreProviderBundle(t.Context(), &MetaConfig{ProviderName: "ensrestore", ProviderBundleDir: dir}, globalOptions)
		require.NoError(t, err)
		require.True(t, usable)
		require.NotNil(t, NewSchemaStore(globalOptions).Get(&SchemaIndex{Kind: "EnsRestoreConfiguration", Version: "deckhouse.io/v1"}))
		require.True(t, NewSchemaStore(globalOptions).ProviderSchemasLoaded("ensrestore", "sha256:ensrestore"), "the schemas of this digest are loaded into this process")
	})
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
	writeTestProviderSchema(t, providerdir.DigestDir(globalOptions.DownloadDir, provider, digest), "EnsNoRegReadConfiguration")

	var asked atomic.Int32
	registry := func(context.Context) (*image.RegistryConfig, error) {
		asked.Add(1)
		return nil, fmt.Errorf("registry unreachable")
	}

	_, gotDigest, err := updateProviderBundle(context.Background(), provider, nil, registry, globalOptions)
	require.NoError(t, err)
	require.Equal(t, digest, gotDigest)
	require.Zero(t, asked.Load(), "the digest was already on disk; the registry must not be read")
}

// A bundle that is not on this machine is reported as missing, not as one without a schema. Nothing
// advises deleting the state cache: a destroy retried after the masters are gone keeps the only
// copy of the infrastructure state there.
func requireMissingBundleError(t *testing.T, err error, dir string) {
	t.Helper()
	require.ErrorContains(t, err, "provider bundle "+dir+" is missing")
	require.NotContains(t, err.Error(), "no cluster configuration schema")
	require.NotContains(t, err.Error(), "delete the state cache")
}
