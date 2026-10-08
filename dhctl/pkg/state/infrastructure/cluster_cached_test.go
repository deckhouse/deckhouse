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

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

const cachedBundleSchema = `kind: CachedBundleConfiguration
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
`

type unreachableKube struct{}

func (unreachableKube) KubeClientCtx(context.Context) (*client.KubernetesClient, error) {
	return nil, errors.New("kubernetes API is unreachable")
}

// A MetaConfig from the state cache is used only together with the bundle it was built with.
func TestPopulateMetaConfigRestoresTheBundleOfTheCachedMetaConfig(t *testing.T) {
	const (
		provider = "cachedbundle"
		digest   = "sha256:cachedbundle"
	)

	globalOptions := &options.GlobalOptions{CandiDir: t.TempDir(), DownloadDir: t.TempDir()}
	bundleDir := providerdir.DigestDir(globalOptions.DownloadDir, provider, digest)
	require.NoError(t, os.MkdirAll(filepath.Dir(providerdir.SchemaPath(bundleDir)), 0o755))
	require.NoError(t, os.WriteFile(providerdir.SchemaPath(bundleDir), []byte(cachedBundleSchema), 0o644))

	populate := func(ctx context.Context, t *testing.T, cached *config.MetaConfig) (*config.MetaConfig, error) {
		stateCache := cache.NewTestCache()
		require.NoError(t, stateCache.SaveStruct(ctx, "cluster-config", cached))
		loader := NewCachedTerraStateLoader(unreachableKube{}, stateCache, "destroy").WithForceFromCache(true)
		return loader.PopulateMetaConfig(ctx, globalOptions)
	}

	t.Run("the recorded bundle gets its schemas loaded", func(t *testing.T) {
		metaConfig, err := populate(t.Context(), t, &config.MetaConfig{ProviderName: provider, ProviderBundleDir: bundleDir})
		require.NoError(t, err)
		require.Equal(t, bundleDir, metaConfig.ProviderBundleDir)
		require.True(t, config.NewSchemaStore(globalOptions).ProviderSchemasLoaded(provider, digest))
	})

	// With --skip-resources the next thing the operator sees is "State not found in cache", so say why.
	t.Run("one saved without a bundle dir is read from the cluster again", func(t *testing.T) {
		var logged bytes.Buffer
		ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

		_, err := populate(ctx, t, &config.MetaConfig{ProviderName: provider})
		require.ErrorContains(t, err, "kubernetes API is unreachable")
		require.Contains(t, logged.String(), "The cached cluster configuration predates provider bundle tracking, reading it from the cluster instead")
	})
}
