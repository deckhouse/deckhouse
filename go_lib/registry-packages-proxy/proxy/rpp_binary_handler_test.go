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
	"archive/tar"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

var rppGetTestDigest = "sha256:" + strings.Repeat("1", 64)

func rppGetServer(t *testing.T, getter registry.ClientConfigGetter, client registry.Client) *httptest.Server {
	t.Helper()

	server := NewRPPClientBinaryServerFromRegistry(RPPClientBinaryServerOptions{
		Logger:             nopCLILogger{},
		ClientConfigGetter: getter,
		RegistryClient:     client,
		ClusterUUID:        "cluster-uuid",
	})
	srv := httptest.NewServer(server.server.Handler)
	t.Cleanup(srv.Close)

	return srv
}

func rppGet(t *testing.T, srv *httptest.Server, urlPath string) (*http.Response, []byte) {
	t.Helper()

	resp, err := http.Get(srv.URL + urlPath)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	return resp, body
}

func compressedLayer(t *testing.T, layer v1.Layer) []byte {
	t.Helper()

	reader, err := layer.Compressed()
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	return body
}

// A node verifies the binary through the manifest, so the bootstrap server hands out the manifest
// bytes and the layer bytes as they are, beside the extracted binary that earlier bundles ask for.
func TestRPPGetBootstrapServesTheManifestAndTheLayer(t *testing.T) {
	binary := "#!/bin/sh\necho rpp-get\n"
	layer := packageBodyWithFiles(t, map[string]string{"rpp-get": binary})
	manifest := []byte(`{"schemaVersion":2}`)
	srv := rppGetServer(t, &fakeCLIGetter{}, &fakeCLIRegistryClient{
		packageBody:  layer,
		rawManifests: map[string]string{":" + rppGetTestDigest: string(manifest)},
	})

	resp, body := rppGet(t, srv, "/cluster-uuid/rpp-get/manifest?digest="+rppGetTestDigest)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, manifest, body)
	require.Equal(t, "application/vnd.oci.image.manifest.v1+json", resp.Header.Get("Content-Type"))
	require.Empty(t, resp.Header.Get("Content-Disposition"))

	resp, body = rppGet(t, srv, "/cluster-uuid/rpp-get/layer?digest="+rppGetTestDigest)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, layer, body)
	require.Equal(t, "application/x-gzip", resp.Header.Get("Content-Type"))

	resp, body = rppGet(t, srv, "/cluster-uuid/rpp-get?digest="+rppGetTestDigest)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, binary, string(body))
	require.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	require.Equal(t, `attachment; filename="rpp-get"`, resp.Header.Get("Content-Disposition"))

	for _, path := range []string{"/cluster-uuid/rpp-get/manifest", "/cluster-uuid/rpp-get/layer", "/cluster-uuid/rpp-get"} {
		require.Equal(t, http.StatusBadRequest, getStatus(t, srv, path), path)
	}

	require.Equal(t, http.StatusNotFound, getStatus(t, srv, "/rpp-get/manifest?digest="+rppGetTestDigest), "the cluster UUID prefix is part of the path")
	require.Equal(t, http.StatusNotFound, getStatus(t, srv, "/cluster-uuid/rpp-get/config?digest="+rppGetTestDigest))
	require.Equal(t, http.StatusNotFound, getStatus(t, srv, "/cluster-uuid/rpp-get/manifest?digest=sha256:"+strings.Repeat("2", 64)))

	// Anyone on the node network reaches this port, so a layer without the binary is not served.
	readme := rppGetServer(t, &fakeCLIGetter{}, &fakeCLIRegistryClient{packageBody: packageBodyWithoutIcon(t)})
	require.Equal(t, http.StatusInternalServerError, getStatus(t, readme, "/cluster-uuid/rpp-get/layer?digest="+rppGetTestDigest))
	require.Equal(t, http.StatusInternalServerError, getStatus(t, readme, "/cluster-uuid/rpp-get?digest="+rppGetTestDigest))

	resp, err := http.Post(srv.URL+"/cluster-uuid/rpp-get/manifest?digest="+rppGetTestDigest, "text/plain", nil)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

// A tag would reach the manifest of any image in the repository.
func TestRPPGetBootstrapTakesOnlyADigest(t *testing.T) {
	client := &fakeCLIRegistryClient{
		packageBody:  packageBodyWithFiles(t, map[string]string{"rpp-get": "binary"}),
		rawManifests: map[string]string{":v1": `{"schemaVersion":2}`},
	}
	srv := rppGetServer(t, &fakeCLIGetter{}, client)

	for _, path := range []string{"/cluster-uuid/rpp-get/manifest", "/cluster-uuid/rpp-get/layer", "/cluster-uuid/rpp-get"} {
		require.Equal(t, http.StatusBadRequest, getStatus(t, srv, path+"?digest=v1"), path)
	}

	require.Zero(t, client.manifestCalls)
	require.Zero(t, client.getPackageCalls)
}

// The digest may name any image of the repository, so a layer over the limit is refused as a
// whole instead of being read into memory or cut at the limit.
func TestRPPGetBootstrapRefusesALayerOverTheLimit(t *testing.T) {
	random := make([]byte, maxRPPGetLayerBytes)
	_, err := rand.Read(random)
	require.NoError(t, err)
	srv := rppGetServer(t, &fakeCLIGetter{}, &fakeCLIRegistryClient{
		packageBody: compressedLayer(t, tarLayerWithFile(t, "rpp-get", string(random))),
	})

	require.Equal(t, http.StatusInternalServerError, getStatus(t, srv, "/cluster-uuid/rpp-get/layer?digest="+rppGetTestDigest))
	require.Equal(t, http.StatusInternalServerError, getStatus(t, srv, "/cluster-uuid/rpp-get?digest="+rppGetTestDigest))
}

// GNU tar on the node takes only the member named exactly rpp-get, so a layer with ./rpp-get is
// of no use to it. The bare path keeps matching rpp-get anywhere, as earlier releases did.
func TestRPPGetBootstrapServesALayerTheNodeCanUse(t *testing.T) {
	srv := rppGetServer(t, &fakeCLIGetter{}, &fakeCLIRegistryClient{
		packageBody: compressedLayer(t, tarLayerWithFile(t, "./rpp-get", "binary")),
	})

	require.Equal(t, http.StatusInternalServerError, getStatus(t, srv, "/cluster-uuid/rpp-get/layer?digest="+rppGetTestDigest))
	require.Equal(t, http.StatusOK, getStatus(t, srv, "/cluster-uuid/rpp-get?digest="+rppGetTestDigest))
}

// The node checks the manifest against the digest from its bundle, looks for the sha256 of the
// layer in the manifest and takes rpp-get out of the layer (candi/bashible/lib.sh.tpl,
// bb-rpp-get-download). Against a registry, what the routes serve passes those checks.
func TestRPPGetBootstrapServesWhatTheNodeVerifies(t *testing.T) {
	registryServer := httptest.NewServer(ggcrregistry.New())
	t.Cleanup(registryServer.Close)
	repository := strings.TrimPrefix(registryServer.URL, "http://") + "/deckhouse"

	// As the rpp-get image is built: an empty layer, then the binary with its install and uninstall scripts.
	emptyLayer, err := crane.Layer(map[string][]byte{})
	require.NoError(t, err)
	rppGetLayer, err := crane.Layer(map[string][]byte{"install": []byte("#!/bin/sh\n"), "uninstall": []byte("#!/bin/sh\n"), "rpp-get": []byte("binary")})
	require.NoError(t, err)
	image, err := mutate.AppendLayers(empty.Image, emptyLayer, rppGetLayer)
	require.NoError(t, err)
	reference, err := name.ParseReference(repository + ":rpp-get")
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, image))
	digest, err := image.Digest()
	require.NoError(t, err)

	srv := rppGetServer(t, &fakeCLIGetter{cfg: &registry.ClientConfig{Repository: repository, Scheme: "http"}}, &registry.DefaultClient{})

	resp, manifest := rppGet(t, srv, "/cluster-uuid/rpp-get/manifest?digest="+digest.String())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(manifest))
	manifestSum := sha256.Sum256(manifest)
	require.Equal(t, digest.Hex, hex.EncodeToString(manifestSum[:]))

	resp, layer := rppGet(t, srv, "/cluster-uuid/rpp-get/layer?digest="+digest.String())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(layer))
	layerSum := sha256.Sum256(layer)
	require.Contains(t, string(manifest), `"sha256:`+hex.EncodeToString(layerSum[:])+`"`)

	binary, err := extractTarGzFile(bytes.NewReader(layer), func(header *tar.Header) bool { return header.Name == "rpp-get" }, 1<<20)
	require.NoError(t, err)
	require.Equal(t, "binary", string(binary))
}
