// Copyright 2021 Flant JSC
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

package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

var once sync.Once

var (
	_ state.Cache = &cache.StateCache{}
	_ state.Cache = &cache.DummyCache{}
	_ state.Cache = &client.StateCache{}
)

var globalCache state.Cache = &cache.DummyCache{}

func choiceCache(ctx context.Context, identity string, opts CacheOptions) (state.Cache, error) {
	cacheOpts := opts.Cache
	tmpDir := filepath.Join(cacheOpts.Dir, stringsutil.Sha256Encode(identity))
	log.InfoF("State cache directory: %s\n", tmpDir)

	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("Can't create cache directory: %w", err)
	}

	if cacheOpts.KubeNamespace == "" {
		if opts.ResetInitialState {
			return cache.NewStateCacheWithInitialState(tmpDir, opts.InitialState)
		}
		return cache.NewStateCache(tmpDir)
	}

	log.DebugLn("Use kubernetes state cache")

	kubeCl := client.NewKubernetesClient()
	err := kubeCl.Init(&client.KubernetesInitParams{
		KubeConfig:          cacheOpts.KubeConfig,
		KubeConfigContext:   cacheOpts.KubeConfigContext,
		KubeConfigInCluster: cacheOpts.KubeConfigInCluster,
	})
	if err != nil {
		return nil, err
	}

	secretName := identity
	if cacheOpts.KubeName != "" {
		secretName = cacheOpts.KubeName
	}

	k8sCache := client.NewK8sStateCache(kubeCl, cacheOpts.KubeNamespace, secretName, tmpDir).
		WithLabels(cacheOpts.KubeLabels)

	err = k8sCache.Init(ctx)
	if err != nil {
		return nil, err
	}

	hasTombstone, err := k8sCache.InCache(ctx, state.TombstoneKey)
	if err != nil {
		return nil, err
	}

	if hasTombstone {
		log.InfoF("Tombstone found in Kubernetes cache - cluster may have already been bootstrapped\n")
		return nil, fmt.Errorf("Cache exhausted")
	}

	return k8sCache, nil
}

func initCache(ctx context.Context, identity string, opts CacheOptions) error {
	var err error

	if opts.ResetInitialState {
		globalCache, err = choiceCache(ctx, identity, opts)
	} else {
		once.Do(func() {
			globalCache, err = choiceCache(ctx, identity, opts)
		})
	}

	return err
}

// CacheOptions bundles per-call init state with the resolved
// options.CacheOptions used to pick the on-disk vs Kubernetes cache backend.
type CacheOptions struct {
	InitialState      map[string][]byte
	ResetInitialState bool

	// Cache holds the resolved cache configuration (directory, kube secret
	// settings). Required.
	Cache options.CacheOptions
}

func Init(ctx context.Context, identity string, cacheOpts options.CacheOptions) error {
	return initCache(ctx, identity, CacheOptions{Cache: cacheOpts})
}

func InitWithOptions(ctx context.Context, identity string, opts CacheOptions) error {
	return initCache(ctx, identity, opts)
}

func Global() state.Cache {
	return globalCache
}

func Dummy() state.Cache {
	return &cache.DummyCache{}
}

func GetCacheIdentityFromKubeconfig(
	kubeconfigPath string,
	kubeconfigContext string,
) string {
	if kubeconfigPath == "" {
		return ""
	}

	builder := strings.Builder{}
	builder.WriteString("kubeconfig")

	h := sha256.New()
	h.Write([]byte(kubeconfigPath))

	builder.WriteString("-")

	if kubeconfigContext == "" {
		builder.WriteString(hex.EncodeToString(h.Sum(nil)))
		return builder.String()
	}

	h.Write([]byte(kubeconfigContext))
	builder.WriteString(hex.EncodeToString(h.Sum(nil)))

	return builder.String()
}
