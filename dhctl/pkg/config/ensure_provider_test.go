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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
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
	orig := resolveProviderBundleDigest
	resolveProviderBundleDigest = func(_ string) (string, error) {
		if calls != nil {
			calls.Add(1)
		}
		return digest, nil
	}
	t.Cleanup(func() { resolveProviderBundleDigest = orig })
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
