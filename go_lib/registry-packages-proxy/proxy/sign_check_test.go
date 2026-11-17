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

package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"

	pkgCache "github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

func pushUnsignedPackage(t *testing.T) (*registry.ClientConfig, string) {
	t.Helper()

	return pushUnsignedImage(t, "", map[string][]byte{"install": []byte("#!/bin/sh\n"), "rpp-get": []byte("binary")})
}

func pushUnsignedImage(t *testing.T, imagePath string, files map[string][]byte) (*registry.ClientConfig, string) {
	t.Helper()

	registryServer := httptest.NewServer(ggcrregistry.New())
	t.Cleanup(registryServer.Close)
	repository := strings.TrimPrefix(registryServer.URL, "http://") + "/deckhouse"

	layer, err := crane.Layer(files)
	require.NoError(t, err)
	image, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	if imagePath != "" {
		imagePath = "/" + imagePath
	}

	reference, err := name.ParseReference(repository + imagePath + ":v1.0.0")
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, image, remote.WithContext(t.Context())))
	digest, err := image.Digest()
	require.NoError(t, err)

	return &registry.ClientConfig{Repository: repository, Scheme: "http"}, digest.String()
}

func packageServer(t *testing.T, getter registry.ClientConfigGetter, signCheck bool, c *cliMemCache) *httptest.Server {
	t.Helper()

	p := newTestProxy(t, &registry.DefaultClient{}, getter, c)
	p.config = Config{SignCheck: signCheck}
	srv := httptest.NewServer(http.HandlerFunc(p.servePackage))
	t.Cleanup(srv.Close)

	return srv
}

func TestServePackageRefusesAnUnsignedDeckhousePackage(t *testing.T) {
	cfg, digest := pushUnsignedPackage(t)
	cache := newCLIMemCache()
	srv := packageServer(t, &fakeCLIGetter{cfg: cfg}, true, cache)

	resp, body := rppGet(t, srv, "/package?digest="+digest)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	require.Contains(t, string(body), registry.ErrSignatureVerification.Error())

	_, _, err := cache.Get(digest)
	require.ErrorIs(t, err, pkgCache.ErrEntryNotFound)

	resp, body = rppGet(t, srv, "/package?digest="+digest)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, string(body))
}

func TestServePackageSkipsTheSignatureCheck(t *testing.T) {
	cfg, digest := pushUnsignedPackage(t)

	tests := []struct {
		name      string
		signCheck bool
		query     string
	}{
		{name: "a ModuleSource package", signCheck: true, query: "&repository=module-source"},
		{name: "an edition without the check", signCheck: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultConfig := *cfg
			defaultConfig.Repository += "/default"
			moduleConfig := *cfg
			moduleConfig.SkipSignatureCheck = true

			getter := &fakeCLIGetter{configs: map[string]*registry.ClientConfig{
				registry.DefaultRepository: &defaultConfig,
				"module-source":            &moduleConfig,
			}}
			if !tt.signCheck {
				getter.configs[registry.DefaultRepository] = cfg
			}

			srv := packageServer(t, getter, tt.signCheck, newCLIMemCache())

			resp, body := rppGet(t, srv, "/package?digest="+digest+tt.query)
			require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
		})
	}
}

func TestServePackageChecksRepositoryAliases(t *testing.T) {
	cfg, digest := pushUnsignedPackage(t)
	host, _, _ := strings.Cut(cfg.Repository, "/")

	for _, repository := range []string{registry.DefaultRepository, cfg.Repository, host, "anything"} {
		t.Run(repository, func(t *testing.T) {
			srv := packageServer(t, &fakeCLIGetter{cfg: cfg}, true, newCLIMemCache())

			resp, _ := rppGet(t, srv, "/package?digest="+digest+"&repository="+url.QueryEscape(repository))
			require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
		})
	}
}

func TestGetPackageCachedRejectsUncheckedEntry(t *testing.T) {
	cfg, digest := pushUnsignedPackage(t)
	cache := newCLIMemCache()
	client := &registry.DefaultClient{}
	getter := &fakeCLIGetter{cfg: cfg}

	_, reader, err := GetPackageCached(t.Context(), nopCLILogger{}, getter, client, cache, digest, registry.DefaultRepository, "", false, nil)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	_, cached, err := cache.Get(digest)
	require.NoError(t, err)
	require.NoError(t, cached.Close())

	_, reader, err = GetPackageCached(t.Context(), nopCLILogger{}, getter, client, cache, digest, registry.DefaultRepository, "", true, nil)
	if reader != nil {
		require.NoError(t, reader.Close())
	}

	require.ErrorIs(t, err, registry.ErrSignatureVerification)
	require.Nil(t, reader)
}

func TestServePackageAlwaysChecksDefaultRepository(t *testing.T) {
	cfg, digest := pushUnsignedPackage(t)
	cfg.SkipSignatureCheck = true
	srv := packageServer(t, &fakeCLIGetter{cfg: cfg}, true, newCLIMemCache())

	resp, _ := rppGet(t, srv, "/package?digest="+digest)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestGetPackageCachedKeepsVerifiedEntries(t *testing.T) {
	client := &fakeCLIRegistryClient{packageBody: []byte("package"), layerDigest: "layer"}
	cache := newCLIMemCache()

	for range 2 {
		_, reader, err := GetPackageCached(t.Context(), nopCLILogger{}, &fakeCLIGetter{}, client, cache, "sha256:abcd", registry.DefaultRepository, "", true, nil)
		require.NoError(t, err)
		body, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		require.Equal(t, "package", string(body))
	}

	require.Equal(t, int32(1), atomic.LoadInt32(&client.getPackageCalls))
}

func TestMetadataRoutesSkipSignatureCheck(t *testing.T) {
	tests := []struct {
		name        string
		imagePath   string
		requestPath string
	}{
		{name: "CLI", imagePath: "deckhouse-cli", requestPath: "/v1/images/deckhouse-cli/images/v1.0.0"},
		{name: "icon", imagePath: "my-package/version", requestPath: "/v1/packages/repo/my-package/metadata/icon/v1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := pushUnsignedImage(t, tt.imagePath, map[string][]byte{"docs/icon.svg": []byte("<svg>icon</svg>")})
			getter := &fakeCLIGetter{cfg: cfg, packagesCfgs: map[string]*registry.PackagesConfig{
				"repo": {Repository: cfg.Repository, Scheme: cfg.Scheme},
			}}
			p := newTestProxy(t, &registry.DefaultClient{}, getter, newCLIMemCache())
			p.config = Config{SignCheck: true}
			mux := http.NewServeMux()
			mux.HandleFunc(cliImagesPathPrefix, p.CLIHandler())
			mux.HandleFunc(packagesPathPrefix, p.PackagesHandler())
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			resp, body := rppGet(t, srv, tt.requestPath)
			require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
			require.NotEmpty(t, body)
		})
	}
}

func TestRPPGetBootstrapRefusesAnUnsignedPackage(t *testing.T) {
	cfg, digest := pushUnsignedPackage(t)
	server := NewRPPClientBinaryServerFromRegistry(RPPClientBinaryServerOptions{
		Logger:             nopCLILogger{},
		ClientConfigGetter: &fakeCLIGetter{cfg: cfg},
		RegistryClient:     &registry.DefaultClient{},
		SignCheck:          true,
		ClusterUUID:        "cluster-uuid",
	})
	srv := httptest.NewServer(server.server.Handler)
	t.Cleanup(srv.Close)

	for _, path := range []string{"/cluster-uuid/rpp-get/layer", "/cluster-uuid/rpp-get"} {
		resp, body := rppGet(t, srv, path+"?digest="+digest)
		require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, path)
		require.Contains(t, string(body), registry.ErrSignatureVerification.Error(), path)
	}
}
