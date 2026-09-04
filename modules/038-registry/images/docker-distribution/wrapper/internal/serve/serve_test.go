/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package serve

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/google/go-containerregistry/pkg/name"
	craneregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/registry-distribution/internal/config"
	"github.com/deckhouse/registry-distribution/internal/upstream"
)

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func storeConfiguration(root string) *configuration.Configuration {
	settings := &configuration.Configuration{
		Storage: configuration.Storage{
			"filesystem": configuration.Parameters{"rootdirectory": root},
			"delete":     configuration.Parameters{"enabled": true},
		},
	}
	settings.HTTP.Addr = "127.0.0.1:0"
	settings.HTTP.Secret = "test"
	settings.Log.Level = "error"
	settings.Log.AccessLog.Disabled = true
	return settings
}

// TestTheStoreKeepsTheClusterNamesAndOutlivesTheUpstream is what the whole change rests on.
//
// The cluster pulls by a fixed prefix, the upstream serves the image set under its own, and the
// store must follow the cluster's — or changing the upstream would re-lay every blob on disk and
// invalidate every image reference in the cluster. The previous implementation held that apart with
// two options patched into the registry's proxy. Here the mapping happens in front of the cache,
// which is upstream's own and unmodified, and this test is the evidence that the outcome is the
// same one.
func TestTheStoreKeepsTheClusterNamesAndOutlivesTheUpstream(t *testing.T) {
	// The real upstream, serving Deckhouse under its own prefix.
	real := httptest.NewServer(craneregistry.New(craneregistry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(real.Close)

	pushed, err := random.Image(2048, 2)
	require.NoError(t, err)
	upstreamTag, err := name.NewTag(real.Listener.Addr().String()+"/deckhouse/ee/some-module:v1.2.3", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(upstreamTag, pushed))

	// The rewriter, exactly as the serving process builds it.
	target, err := url.Parse(real.URL)
	require.NoError(t, err)
	wrapper := &config.Wrapper{
		Scope: "system/deckhouse",
		Upstream: &config.Upstream{
			Address: target.Host,
			Scheme:  "http",
			Path:    "/deckhouse/ee",
		},
	}
	rewriter, err := upstream.New(wrapper, "127.0.0.1:0", quiet())
	require.NoError(t, err)
	require.NotNil(t, rewriter)

	loopback := httptest.NewServer(rewriter.Handler())
	t.Cleanup(loopback.Close)

	// The cache, pointed at the rewriter and told never to expire anything.
	root := t.TempDir()
	never := time.Duration(0)
	serving := storeConfiguration(root)
	serving.Proxy = configuration.Proxy{RemoteURL: loopback.URL, TTL: &never}

	cache := httptest.NewServer(handlers.NewApp(context.Background(), serving))
	t.Cleanup(cache.Close)

	// A pull by the name the cluster uses.
	clusterTag, err := name.NewTag(cache.Listener.Addr().String()+"/system/deckhouse/some-module:v1.2.3", name.Insecure)
	require.NoError(t, err)

	pulled, err := remote.Image(clusterTag)
	require.NoError(t, err, "a pull by the cluster's own name must reach the upstream's path")
	layers, err := pulled.Layers()
	require.NoError(t, err)
	for _, layer := range layers {
		content, err := layer.Compressed()
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, content)
		require.NoError(t, err, "every layer has to come through, not just the manifest")
		require.NoError(t, content.Close())
	}

	// Where it landed. This is the assertion the two patched options existed for.
	_, err = os.Stat(filepath.Join(root, "docker", "registry", "v2", "repositories",
		"system", "deckhouse", "some-module"))
	require.NoError(t, err, "the store must hold the repository under the name it was asked for")

	_, err = os.Stat(filepath.Join(root, "docker", "registry", "v2", "repositories", "deckhouse"))
	assert.Error(t, err, "and must not hold the upstream's own path, or the layout follows the upstream")

	// And the point of holding it at all: the upstream goes away, as it does when a cluster becomes
	// air-gapped, and the store keeps answering.
	real.Close()
	loopback.Close()

	offline, err := remote.Image(clusterTag)
	require.NoError(t, err, "with the upstream gone the store is the only source, which is the whole design")
	_, err = offline.Manifest()
	require.NoError(t, err)
}

// TestAPushLandsInTheStoreTheCacheServesFrom is the other half of the same store: the cache refuses
// every write — upstream's proxy answers UNSUPPORTED, deliberately — while the store is filled BY
// writes for as long as an upstream is still configured. So the write endpoint is not a convenience,
// and what it writes has to be what the cache reads.
func TestAPushLandsInTheStoreTheCacheServesFrom(t *testing.T) {
	root := t.TempDir()

	// An upstream that is up and holds nothing, so "served from the store" cannot be confused with
	// "fetched from the upstream".
	empty := httptest.NewServer(craneregistry.New(craneregistry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(empty.Close)

	never := time.Duration(0)
	serving := storeConfiguration(root)
	serving.Proxy = configuration.Proxy{RemoteURL: empty.URL, TTL: &never}

	wrapper := &config.Wrapper{
		Scope:         "system/deckhouse",
		WriteEndpoint: config.WriteEndpoint{Address: "127.0.0.1:0"},
	}

	// The write half's configuration, derived by the code that runs in production.
	writing := writeConfiguration(serving, wrapper)
	assert.Empty(t, writing.Proxy.RemoteURL, "a push must be answered by the store, not by a cache")
	assert.Equal(t, root, writing.Storage.Parameters()["rootdirectory"],
		"both halves are one store, or a fill would not be what the cluster pulls")
	assert.Empty(t, writing.HTTP.Debug.Addr,
		"the metrics collectors are process-global; a second registration panics on startup")

	// The two halves, over one directory.
	write := httptest.NewServer(handlers.NewApp(context.Background(), writing))
	t.Cleanup(write.Close)
	cache := httptest.NewServer(handlers.NewApp(context.Background(), serving))
	t.Cleanup(cache.Close)

	image, err := random.Image(1024, 1)
	require.NoError(t, err)

	pushTag, err := name.NewTag(write.Listener.Addr().String()+"/system/deckhouse/filled:v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(pushTag, image), "the write endpoint accepts a push")

	pullTag, err := name.NewTag(cache.Listener.Addr().String()+"/system/deckhouse/filled:v1", name.Insecure)
	require.NoError(t, err)
	served, err := remote.Image(pullTag)
	require.NoError(t, err, "and the cache serves it, with an upstream that has never seen it")
	layers, err := served.Layers()
	require.NoError(t, err)
	for _, layer := range layers {
		content, err := layer.Compressed()
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, content)
		require.NoError(t, err)
		require.NoError(t, content.Close())
	}

	// The reason there are two listeners at all.
	refusedTag, err := name.NewTag(cache.Listener.Addr().String()+"/system/deckhouse/refused:v1", name.Insecure)
	require.NoError(t, err)
	err = remote.Write(refusedTag, image)
	require.Error(t, err, "a cache that accepted writes would make the write endpoint pointless")
	assert.Contains(t, err.Error(), "UNSUPPORTED")
}

// TestWriteConfigurationCarriesTheStoreAndNotTheCache pins the derivation on its own, because every
// field it copies is a field that must not be allowed to drift between the two halves.
func TestWriteConfigurationCarriesTheStoreAndNotTheCache(t *testing.T) {
	serving := storeConfiguration("/opt/deckhouse/registry/local_data")
	serving.Proxy = configuration.Proxy{RemoteURL: "http://127.0.0.1:5004"}
	serving.Auth = configuration.Auth{"token": configuration.Parameters{"realm": "https://registry/auth/token"}}
	serving.HTTP.TLS.Certificate = "/pki/distribution.crt"
	serving.HTTP.Debug.Addr = "127.0.0.1:5002"
	serving.HTTP.Debug.Prometheus.Enabled = true

	writing := writeConfiguration(serving, &config.Wrapper{
		Scope:         "system/deckhouse",
		WriteEndpoint: config.WriteEndpoint{Address: "0.0.0.0:5003"},
	})

	assert.Equal(t, "0.0.0.0:5003", writing.HTTP.Addr)
	assert.Equal(t, serving.Auth, writing.Auth, "one token service for both halves")
	assert.Equal(t, "/pki/distribution.crt", writing.HTTP.TLS.Certificate, "one certificate")
	assert.Empty(t, writing.Proxy.RemoteURL)
	assert.False(t, writing.HTTP.Debug.Prometheus.Enabled)

	// The storage map is a map: a shallow copy would let a read-only flag set for a garbage
	// collection on one half appear on the other.
	writing.Storage["filesystem"]["rootdirectory"] = "/somewhere/else"
	assert.Equal(t, "/opt/deckhouse/registry/local_data", serving.Storage.Parameters()["rootdirectory"])
}

// TestRunRefusesAConfigurationThatDecidesTheUpstreamTwice: which upstream a miss goes to is decided
// by this module's own file, and a proxy section in the rendered registry configuration is a second
// answer nobody reconciles.
func TestRunRefusesAConfigurationThatDecidesTheUpstreamTwice(t *testing.T) {
	settings := storeConfiguration(t.TempDir())
	settings.Proxy = configuration.Proxy{RemoteURL: "https://registry.deckhouse.io"}

	err := Run(context.Background(), settings, &config.Wrapper{Scope: "system/deckhouse"}, quiet())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proxy section")
}

// TestTheWriteEndpointRefusesToShareThePort is one process with two listeners, and the failure is
// worth naming rather than discovering as a bind error at three in the morning.
func TestTheWriteEndpointRefusesToShareThePort(t *testing.T) {
	settings := storeConfiguration(t.TempDir())
	settings.HTTP.Addr = "0.0.0.0:5001"

	_, err := writeEndpoint(context.Background(), settings, &config.Wrapper{
		Scope:         "system/deckhouse",
		WriteEndpoint: config.WriteEndpoint{Address: "0.0.0.0:5001"},
	}, quiet())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "two listeners in one process")
}
