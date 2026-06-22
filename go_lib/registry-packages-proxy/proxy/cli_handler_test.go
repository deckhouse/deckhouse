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

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgCache "github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
	pkgLog "github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

const (
	cliEventuallyTimeout  = 2 * time.Second
	cliEventuallyInterval = 10 * time.Millisecond
)

func TestParseCLIPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url        string
		wantImg    string
		wantAction cliAction
		wantTag    string
		wantErr    bool
	}{
		{url: "/v1/images/deckhouse-cli/tags", wantImg: "deckhouse-cli", wantAction: cliActionListTags},
		{url: "/v1/images/deckhouse-cli/tags/v1.0.1", wantImg: "deckhouse-cli", wantAction: cliActionPullTag, wantTag: "v1.0.1"},
		{url: "/v1/images/deckhouse-cli/plugins/tags", wantImg: "deckhouse-cli/plugins", wantAction: cliActionListTags},
		{url: "/v1/images/deckhouse-cli/plugins/foo/tags", wantImg: "deckhouse-cli/plugins/foo", wantAction: cliActionListTags},
		{url: "/v1/images/deckhouse-cli/plugins/foo/tags/v2", wantImg: "deckhouse-cli/plugins/foo", wantAction: cliActionPullTag, wantTag: "v2"},
		{url: "/v1/images/", wantErr: true},
		{url: "/v1/images/just-image", wantErr: true},
		{url: "/v1/images/img/tags/with/slashes", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.url, func(t *testing.T) {
			img, action, tag, err := parseCLIPath(tc.url)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantImg, img)
			assert.Equal(t, tc.wantAction, action)
			assert.Equal(t, tc.wantTag, tag)
		})
	}
}

func TestIsAllowedCLIImagePath(t *testing.T) {
	t.Parallel()

	allowed := []string{
		"deckhouse-cli",
		"deckhouse-cli/plugins/foo",
		"deckhouse-cli/plugins/some-plugin",
	}
	for _, p := range allowed {
		assert.Truef(t, isAllowedCLIImagePath(p), "expected %q to be allowed", p)
	}

	denied := []string{
		"",
		"other",
		"deckhouse-cli/extras",
		// Bare "plugins" namespace with no plugin name is rejected here
		// rather than relying on go-containerregistry's name validation
		// downstream.
		"deckhouse-cli/plugins",
		"deckhouse-cli/plugins/",
		"deckhouse-cli/plugins/a/b",
		"/deckhouse-cli",
	}
	for _, p := range denied {
		assert.Falsef(t, isAllowedCLIImagePath(p), "expected %q to be denied", p)
	}
}

// fakeCLIRegistryClient implements registry.Client for handler tests.
type fakeCLIRegistryClient struct {
	mu                  sync.Mutex
	tags                map[string][]string
	tagToManifestDigest map[string]string
	platformDigests map[string]string
	lastPlatform    string
	packageBody     []byte
	layerDigest     string

	getPackageCalls int32
	listTagsCalls   int32
	resolveTagCalls int32

	listTagsErr   error
	resolveTagErr error
	getPackageErr error
}

func (f *fakeCLIRegistryClient) ListTags(_ context.Context, _ pkgLog.Logger, _ *registry.ClientConfig, path string) ([]string, error) {
	atomic.AddInt32(&f.listTagsCalls, 1)
	if f.listTagsErr != nil {
		return nil, f.listTagsErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	tags, ok := f.tags[path]
	if !ok {
		return nil, registry.ErrPackageNotFound
	}
	return tags, nil
}

func (f *fakeCLIRegistryClient) ResolveTag(_ context.Context, _ pkgLog.Logger, _ *registry.ClientConfig, path, tag string, platform *v1.Platform) (string, error) {
	atomic.AddInt32(&f.resolveTagCalls, 1)
	if f.resolveTagErr != nil {
		return "", f.resolveTagErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if platform != nil {
		f.lastPlatform = platform.String()
		if d, ok := f.platformDigests[path+":"+tag+":"+platform.String()]; ok {
			return d, nil
		}
	}
	d, ok := f.tagToManifestDigest[path+":"+tag]
	if !ok {
		return "", registry.ErrPackageNotFound
	}
	return d, nil
}

func (f *fakeCLIRegistryClient) GetPackage(_ context.Context, _ pkgLog.Logger, _ *registry.ClientConfig, _ string, _ string) (int64, string, io.ReadCloser, error) {
	atomic.AddInt32(&f.getPackageCalls, 1)
	if f.getPackageErr != nil {
		return 0, "", nil, f.getPackageErr
	}
	return int64(len(f.packageBody)), f.layerDigest, io.NopCloser(bytes.NewReader(f.packageBody)), nil
}

type fakeCLIGetter struct {
	cfg *registry.ClientConfig
	err error

	// packagesCfgs maps PackageRepository name -> resolved packages config.
	// Tests that don't care about the /v1/packages/* path can leave this nil
	// and the default fixture (registry.test/deckhouse + https) is returned.
	packagesCfgs    map[string]*registry.PackagesConfig
	packagesCfgErr  error
	defaultPkgFound bool // when true, missing keys still resolve to default
}

func (g *fakeCLIGetter) Get(_ string) (*registry.ClientConfig, error) {
	if g.err != nil {
		return nil, g.err
	}
	if g.cfg != nil {
		return g.cfg, nil
	}
	return &registry.ClientConfig{Repository: "registry.test/deckhouse", Scheme: "https"}, nil
}

func (g *fakeCLIGetter) GetPackagesConfig(packageRepositoryName string) (*registry.PackagesConfig, error) {
	if g.packagesCfgErr != nil {
		return nil, g.packagesCfgErr
	}
	if g.packagesCfgs != nil {
		if cfg, ok := g.packagesCfgs[packageRepositoryName]; ok {
			return cfg, nil
		}
		if !g.defaultPkgFound {
			return nil, errors.New("package repository not found")
		}
	}
	return &registry.PackagesConfig{Repository: "registry.test/deckhouse", Scheme: "https"}, nil
}

// cliMemCache is a simple in-memory cache.Cache used to verify cache hits.
type cliMemCache struct {
	mu      sync.Mutex
	entries map[string][]byte
	hits    int32
	misses  int32
}

func newCLIMemCache() *cliMemCache {
	return &cliMemCache{entries: map[string][]byte{}}
}

func (c *cliMemCache) Get(digest string) (int64, io.ReadCloser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	body, ok := c.entries[digest]
	if !ok {
		atomic.AddInt32(&c.misses, 1)
		return 0, nil, pkgCache.ErrEntryNotFound
	}
	atomic.AddInt32(&c.hits, 1)
	return int64(len(body)), io.NopCloser(bytes.NewReader(body)), nil
}

func (c *cliMemCache) Set(digest string, _ string, reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[digest] = data
	return nil
}

// nopCLILogger satisfies log.Logger without producing output.
type nopCLILogger struct{}

func (nopCLILogger) Errorf(string, ...interface{}) {}
func (nopCLILogger) Infof(string, ...interface{})  {}
func (nopCLILogger) Warnf(string, ...interface{})  {}
func (nopCLILogger) Debugf(string, ...interface{}) {}
func (nopCLILogger) Error(string, ...interface{})  {}

// newTestProxy builds a Proxy wired up only with the bits CLIHandler needs.
// It deliberately avoids creating an http.Server / net.Listener so the tests can register the
// handler on their own mux via Proxy.CLIHandler().
func newTestProxy(t *testing.T, registryClient registry.Client, getter registry.ClientConfigGetter, c pkgCache.Cache) *Proxy {
	t.Helper()
	var opts []ProxyOption
	if c != nil {
		opts = append(opts, WithCache(c))
	}
	p := NewProxy(nil, nil, getter, nopCLILogger{}, registryClient, opts...)
	// Serve() normally initializes p.config; do the equivalent for CLIHandler tests.
	p.config = Config{}
	return p
}

func TestCLIHandler_ListTags_HappyPath(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		tags: map[string][]string{
			"deckhouse-cli": {"v1.0.0", "v1.0.1"},
		},
	}
	p := newTestProxy(t, fake, &fakeCLIGetter{}, nil)

	mux := http.NewServeMux()
	mux.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/images/deckhouse-cli/tags")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out cliTagsResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Equal(t, "deckhouse-cli", out.Name)
	assert.Equal(t, []string{"v1.0.0", "v1.0.1"}, out.Tags)
}

func TestCLIHandler_ListTags_NotFound(t *testing.T) {
	fake := &fakeCLIRegistryClient{tags: map[string][]string{}}
	p := newTestProxy(t, fake, &fakeCLIGetter{}, nil)

	mux := http.NewServeMux()
	mux.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/images/deckhouse-cli/tags")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCLIHandler_DisallowedImagePath(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		tags: map[string][]string{
			"other-image": {"v1"},
		},
	}
	p := newTestProxy(t, fake, &fakeCLIGetter{}, nil)

	mux := http.NewServeMux()
	mux.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, urlPath := range []string{
		"/v1/images/other-image/tags",
		"/v1/images/deckhouse-cli/extras/tags",
		"/v1/images/deckhouse-cli/plugins/a/b/tags",
	} {
		resp, err := http.Get(srv.URL + urlPath)
		require.NoError(t, err, urlPath)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, urlPath)
		_ = resp.Body.Close()
	}

	// Make sure the registry was never consulted for a denied path.
	assert.Equal(t, int32(0), atomic.LoadInt32(&fake.listTagsCalls))
}

func TestCLIHandler_PullTag_HappyPathAndCache(t *testing.T) {
	const payload = "hello, world"
	fake := &fakeCLIRegistryClient{
		tagToManifestDigest: map[string]string{
			"deckhouse-cli:v1.0.1": "sha256:deadbeef",
		},
		packageBody: []byte(payload),
		layerDigest: "abc123",
	}
	cache := newCLIMemCache()
	p := newTestProxy(t, fake, &fakeCLIGetter{}, cache)

	mux := http.NewServeMux()
	mux.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// First request: cache miss, registry consulted.
	resp, err := http.Get(srv.URL + "/v1/images/deckhouse-cli/tags/v1.0.1")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, payload, string(body))
	assert.Equal(t, "application/x-gzip", resp.Header.Get("Content-Type"))
	assert.Equal(t, `"sha256:deadbeef"`, resp.Header.Get("ETag"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), `filename="deckhouse-cli-v1.0.1.tar.gz"`)
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.resolveTagCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.getPackageCalls))

	// Give the async cache writer a moment to finish populating the cache.
	require.Eventually(t, func() bool {
		_, rd, err := cache.Get("sha256:deadbeef")
		if err == nil && rd != nil {
			_ = rd.Close()
			return true
		}
		return false
	}, cliEventuallyTimeout, cliEventuallyInterval)

	// Second request: should be served from cache, registry GetPackage NOT called again.
	resp2, err := http.Get(srv.URL + "/v1/images/deckhouse-cli/tags/v1.0.1")
	require.NoError(t, err)
	body2, err := io.ReadAll(resp2.Body)
	require.NoError(t, err)
	require.NoError(t, resp2.Body.Close())
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Equal(t, payload, string(body2))
	assert.Equal(t, int32(2), atomic.LoadInt32(&fake.resolveTagCalls), "resolve is always called")
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.getPackageCalls), "GetPackage must not be called again on cache hit")
	assert.GreaterOrEqual(t, atomic.LoadInt32(&cache.hits), int32(1))
}

func TestCLIHandler_PullTag_NotFound(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		tagToManifestDigest: map[string]string{},
	}
	p := newTestProxy(t, fake, &fakeCLIGetter{}, nil)

	mux := http.NewServeMux()
	mux.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/images/deckhouse-cli/tags/v99.99.99")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCLIHandler_PullTag_BadGatewayOnRegistryError(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		resolveTagErr: errors.New("boom"),
	}
	p := newTestProxy(t, fake, &fakeCLIGetter{}, nil)

	mux := http.NewServeMux()
	mux.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/images/deckhouse-cli/tags/v1.0.0")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestCLIHandler_PullTag_PlatformSelectsChildDigest(t *testing.T) {
	const payload = "platform-specific binary"
	fake := &fakeCLIRegistryClient{
		platformDigests: map[string]string{
			"deckhouse-cli:v1.0.1:linux/amd64": "sha256:aaaaamd64",
			"deckhouse-cli:v1.0.1:linux/arm64": "sha256:bbbbarm64",
		},
		packageBody: []byte(payload),
		layerDigest: "layer123",
	}
	cache := newCLIMemCache()
	p := newTestProxy(t, fake, &fakeCLIGetter{}, cache)

	mux := http.NewServeMux()
	mux.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// arm64: the proxy resolves the arm64 child digest and stamps it as the ETag.
	resp, err := http.Get(srv.URL + "/v1/images/deckhouse-cli/tags/v1.0.1?platform=linux/arm64")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `"sha256:bbbbarm64"`, resp.Header.Get("ETag"))
	assert.Equal(t, "linux/arm64", fake.lastPlatform)

	// amd64: a different child digest.
	resp, err = http.Get(srv.URL + "/v1/images/deckhouse-cli/tags/v1.0.1?platform=linux/amd64")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `"sha256:aaaaamd64"`, resp.Header.Get("ETag"))

	// Different platforms resolved to different digests, so the cache holds two
	// distinct entries - one platform can never serve another's bytes.
	require.Eventually(t, func() bool {
		_, r1, e1 := cache.Get("sha256:aaaaamd64")
		_, r2, e2 := cache.Get("sha256:bbbbarm64")
		if e1 == nil && e2 == nil {
			_ = r1.Close()
			_ = r2.Close()
			return true
		}
		return false
	}, cliEventuallyTimeout, cliEventuallyInterval)
}

func TestCLIHandler_PullTag_InvalidPlatform(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		tagToManifestDigest: map[string]string{"deckhouse-cli:v1.0.1": "sha256:deadbeef"},
	}
	p := newTestProxy(t, fake, &fakeCLIGetter{}, nil)

	mux := http.NewServeMux()
	mux.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Too many slashes: v1.ParsePlatform rejects it, the handler answers 400 and
	// never consults the registry.
	resp, err := http.Get(srv.URL + "/v1/images/deckhouse-cli/tags/v1.0.1?platform=linux/amd64/v8/oops")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int32(0), atomic.LoadInt32(&fake.resolveTagCalls))
}
