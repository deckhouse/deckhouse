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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgCache "github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

const (
	testPackageRepositoryName = "packages-repo"
	testPackageName           = "my-package"
	testImagePath             = "packages/my-package"
	testIconPath    = "docs/icon.svg"
	testIconContent = "<svg>icon</svg>"
)

var testPackageRepositoryDockerCfg = base64.StdEncoding.EncodeToString(
	[]byte(`{"auths":{"registry.test":{"auth":"dXNlcjpwYXNz"}}}`),
)

func packageBodyWithIcon(t *testing.T) []byte {
	t.Helper()

	layer := tarLayerWithFile(t, testIconPath, testIconContent)
	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	reader := flattenedPackageReader(t, img)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	return data
}

func packageBodyWithoutIcon(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "README",
		Mode: 0o644,
		Size: 5,
	}))
	_, err := tw.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func newPackagesTestServer(
	t *testing.T,
	fake *fakeCLIRegistryClient,
	getter registry.ClientConfigGetter,
	c pkgCache.Cache,
) *httptest.Server {
	t.Helper()

	p := newPackagesTestProxy(t, fake, getter, c, true)
	mux := http.NewServeMux()
	mux.HandleFunc(packagesPathPrefix, p.PackagesHandler())
	return httptest.NewServer(mux)
}

func newPackagesTestServerWithoutRepository(
	t *testing.T,
	fake *fakeCLIRegistryClient,
	getter registry.ClientConfigGetter,
	c pkgCache.Cache,
) *httptest.Server {
	t.Helper()

	p := newPackagesTestProxy(t, fake, getter, c, false)
	mux := http.NewServeMux()
	mux.HandleFunc(packagesPathPrefix, p.PackagesHandler())
	return httptest.NewServer(mux)
}

func newPackagesTestProxy(
	t *testing.T,
	registryClient registry.Client,
	getter registry.ClientConfigGetter,
	c pkgCache.Cache,
	withDefaultRepository bool,
) *Proxy {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme)
	if withDefaultRepository {
		fakeClientBuilder = fakeClientBuilder.WithObjects(&v1alpha1.PackageRepository{
			ObjectMeta: metav1.ObjectMeta{Name: testPackageRepositoryName},
			Spec: v1alpha1.PackageRepositorySpec{
				Registry: v1alpha1.PackageRepositorySpecRegistry{
					Repo:      "registry.test/deckhouse",
					Scheme:    "https",
					DockerCFG: testPackageRepositoryDockerCfg,
				},
			},
		})
	}
	k8sClient := fakeClientBuilder.Build()

	var opts []ProxyOption
	if c != nil {
		opts = append(opts, WithCache(c))
	}

	p := NewProxy(nil, nil, getter, nopCLILogger{}, k8sClient, registryClient, opts...)
	p.config = Config{}
	return p
}

func TestPackagesHandler_MethodNotAllowed(t *testing.T) {
	fake := &fakeCLIRegistryClient{}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/packages/packages-repo/my-package/metadata/icon/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	assert.Equal(t, int32(0), atomic.LoadInt32(&fake.listTagsCalls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&fake.resolveTagCalls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&fake.getPackageCalls))
}

func TestPackagesHandler_InvalidPath(t *testing.T) {
	fake := &fakeCLIRegistryClient{}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/unknown/action")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int32(0), atomic.LoadInt32(&fake.listTagsCalls))
}

func TestPackagesHandler_GetIcon_SpecificVersion(t *testing.T) {
	const manifestDigest = "sha256:deadbeef"
	fake := &fakeCLIRegistryClient{
		tagToManifestDigest: map[string]string{
			testImagePath + ":v1.0.1": manifestDigest,
		},
		packageBody: packageBodyWithIcon(t),
		layerDigest: "layer-digest",
	}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/v1.0.1")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, testIconContent, string(body))
	assert.Equal(t, "image/svg+xml", resp.Header.Get("Content-Type"))
	assert.Equal(t, `attachment; filename="my-package.svg"`, resp.Header.Get("Content-Disposition"))
	assert.Equal(t, strconv.Itoa(len(testIconContent)), resp.Header.Get("Content-Length"))

	assert.Equal(t, int32(0), atomic.LoadInt32(&fake.listTagsCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.resolveTagCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.getPackageCalls))
}

func TestPackagesHandler_GetIcon_UsesPackageRepositoryConfigForCacheMiss(t *testing.T) {
	const manifestDigest = "sha256:package-repository"
	fake := &fakeCLIRegistryClient{
		tagToManifestDigest: map[string]string{
			testImagePath + ":v1.0.1": manifestDigest,
		},
		packageBody: packageBodyWithIcon(t),
		layerDigest: "layer-digest",
	}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{err: errors.New("getter should not be called")}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/v1.0.1")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, testIconContent, string(body))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.resolveTagCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.getPackageCalls))
}

func TestPackagesHandler_GetIcon_LatestVersion(t *testing.T) {
	const manifestDigest = "sha256:cafebabe"
	fake := &fakeCLIRegistryClient{
		tags: map[string][]string{
			testImagePath: {"v1.0.0", "v1.0.1", "not-a-version"},
		},
		tagToManifestDigest: map[string]string{
			testImagePath + ":v1.0.1": manifestDigest,
		},
		packageBody: packageBodyWithIcon(t),
	}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, testIconContent, string(body))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.listTagsCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.resolveTagCalls))
}

func TestPackagesHandler_GetIcon_HEAD(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		tagToManifestDigest: map[string]string{
			testImagePath + ":v1.0.1": "sha256:head",
		},
		packageBody: packageBodyWithIcon(t),
	}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodHead, srv.URL+"/v1/packages/packages-repo/my-package/metadata/icon/v1.0.1", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "image/svg+xml", resp.Header.Get("Content-Type"))
	assert.Equal(t, `attachment; filename="my-package.svg"`, resp.Header.Get("Content-Disposition"))
	assert.Empty(t, resp.Header.Get("Content-Length"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Empty(t, body)

	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.getPackageCalls))
}

func TestPackagesHandler_GetIcon_PackageNotFound(t *testing.T) {
	fake := &fakeCLIRegistryClient{tags: map[string][]string{}}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.listTagsCalls))
}

func TestPackagesHandler_GetIcon_NoValidTags(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		tags: map[string][]string{
			testImagePath: {"latest", "release"},
		},
	}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.listTagsCalls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&fake.resolveTagCalls))
}

func TestPackagesHandler_GetIcon_TagNotFound(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		tagToManifestDigest: map[string]string{},
	}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/v9.9.9")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.resolveTagCalls))
}

func TestPackagesHandler_GetIcon_IconMissingInArchive(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		tagToManifestDigest: map[string]string{
			testImagePath + ":v1.0.0": "sha256:nope",
		},
		packageBody: packageBodyWithoutIcon(t),
	}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/v1.0.0")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestPackagesHandler_GetIcon_RegistryConfigUnavailable(t *testing.T) {
	fake := &fakeCLIRegistryClient{}
	srv := newPackagesTestServerWithoutRepository(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/v1.0.0")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, int32(0), atomic.LoadInt32(&fake.resolveTagCalls))
}

func TestPackagesHandler_GetIcon_BadGatewayOnRegistryError(t *testing.T) {
	fake := &fakeCLIRegistryClient{
		resolveTagErr: errors.New("boom"),
	}
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/v1.0.0")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestPackagesHandler_GetIcon_CacheHit(t *testing.T) {
	const manifestDigest = "sha256:cached"
	fake := &fakeCLIRegistryClient{
		tagToManifestDigest: map[string]string{
			testImagePath + ":v1.0.1": manifestDigest,
		},
		packageBody: packageBodyWithIcon(t),
	}
	cache := newCLIMemCache()
	srv := newPackagesTestServer(t, fake, &fakeCLIGetter{}, cache)
	defer srv.Close()

	url := srv.URL + "/v1/packages/packages-repo/my-package/metadata/icon/v1.0.1"

	resp, err := http.Get(url)
	require.NoError(t, err)
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Eventually(t, func() bool {
		_, rd, err := cache.Get(manifestDigest)
		if err == nil && rd != nil {
			_ = rd.Close()
			return true
		}
		return false
	}, cliEventuallyTimeout, cliEventuallyInterval)

	resp2, err := http.Get(url)
	require.NoError(t, err)
	body, err := io.ReadAll(resp2.Body)
	require.NoError(t, err)
	require.NoError(t, resp2.Body.Close())

	require.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Equal(t, testIconContent, string(body))
	assert.Equal(t, int32(2), atomic.LoadInt32(&fake.resolveTagCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fake.getPackageCalls))
	assert.GreaterOrEqual(t, atomic.LoadInt32(&cache.hits), int32(1))
}

func TestParsePackagesPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url         string
		wantRepo    string
		wantPackage string
		wantAction  packagesAction
		wantVersion string
		wantErr     bool
	}{
		{
			url:         "/v1/packages/packages-repo/my-package/metadata/icon/",
			wantRepo:    testPackageRepositoryName,
			wantPackage: testPackageName,
			wantAction:  packagesMetadataActionGetIcon,
		},
		{
			url:         "/v1/packages/packages-repo/my-package/metadata/icon",
			wantRepo:    testPackageRepositoryName,
			wantPackage: testPackageName,
			wantAction:  packagesMetadataActionGetIcon,
		},
		{
			url:         "/v1/packages/packages-repo/my-package/metadata/icon/v1.0.1",
			wantRepo:    testPackageRepositoryName,
			wantPackage: testPackageName,
			wantAction:  packagesMetadataActionGetIcon,
			wantVersion: "v1.0.1",
		},
		{
			url:         "/v1/packages/foo/bar/metadata/icon",
			wantRepo:    "foo",
			wantPackage: "bar",
			wantAction:  packagesMetadataActionGetIcon,
		},
		{url: "/v1/packages/", wantErr: true},
		{url: "/v1/packages/packages-repo", wantErr: true},
		{url: "/v1/packages/packages-repo/my-package", wantErr: true},
		{url: "/v1/packages/my-package/metadata/icon", wantErr: true},
		{url: "/v1/packages/packages-repo/my-package/unknown/action", wantErr: true},
		{url: "/v1/packages/packages-repo/my-package/metadata/icon/not-semver", wantErr: true},
		{url: "/v1/packages/packages-repo/my-package/metadata/icon/v1/2/3", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()

			action, repo, pkg, version, err := parsePackagesPath(tc.url)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantAction, action)
			assert.Equal(t, tc.wantRepo, repo)
			assert.Equal(t, tc.wantPackage, pkg)
			assert.Equal(t, tc.wantVersion, version)
		})
	}
}
