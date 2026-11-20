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
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-agent/internal/metrics"
)

// layerContent is what the image set holds under layerDigest.
const layerContent = "the layer"

var layerDigest = func() string {
	sum := sha256.Sum256([]byte(layerContent))
	return "sha256:" + hex.EncodeToString(sum[:])
}()

const mirrorPath = "/d8-proxy/deckhouse/ee"

func blobPath(digest string) string {
	return "/v2" + constant.Path + "/modules/ml/blobs/" + digest
}

func mirrorLayout(t *testing.T, mirror, cache *registryStub) *registryv1alpha1.RegistryNodeSpec {
	t.Helper()

	return &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendStorage, cache, t, constant.Path, nil),
		},
		PrimaryMirror: &registryv1alpha1.PrimaryMirror{
			Name: "gpu-eu",
			Upstream: registryv1alpha1.Upstream{Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP,
				Host:   mirror.host(t),
				Path:   mirrorPath,
			}},
		},
	}
}

func withMetrics(server *Server) *Server {
	server.Metrics = metrics.New(prometheus.NewRegistry())
	return server
}

func mirrorCount(server *Server, result string) float64 {
	var metric dto.Metric
	if err := server.Metrics.MirrorRequests.WithLabelValues("gpu-eu", result).Write(&metric); err != nil {
		return -1
	}
	return metric.GetCounter().GetValue()
}

func TestResolveAsksTheMirrorFirstForADigestOnly(t *testing.T) {
	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name:     registryv1alpha1.BackendStorage,
			Endpoint: registryv1alpha1.Endpoint{Host: "10.0.0.1:4281", Path: constant.Path},
		}},
		PrimaryMirror: &registryv1alpha1.PrimaryMirror{
			Name: "gpu-eu",
			Upstream: registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{Host: "harbor.eu.example.com", Path: mirrorPath},
				Mirrors:  []registryv1alpha1.Endpoint{{Host: "harbor-2.eu.example.com", Path: "/other"}},
			},
		},
	}

	for _, path := range []string{
		blobPath(layerDigest),
		"/v2" + constant.Path + "/modules/ml/manifests/" + layerDigest,
		// The image of an embedded module names the image set's prefix as its whole repository.
		"/v2" + constant.Path + "/manifests/" + layerDigest,
	} {
		decision, err := Resolve(constant.Host, path, spec, self)
		require.NoError(t, err, path)
		require.Len(t, decision.Targets, 3, path)

		assert.Equal(t, TargetMirror, decision.Targets[0].Name, path)
		assert.Equal(t, "harbor.eu.example.com", decision.Targets[0].Host, path)
		assert.Equal(t, "gpu-eu", decision.Targets[0].Mirror, path)
		assert.Equal(t, layerDigest, decision.Targets[0].Digest, path)
		assert.Equal(t, "harbor-2.eu.example.com", decision.Targets[1].Host, path)
		assert.Equal(t, string(registryv1alpha1.BackendStorage), decision.Targets[2].Name, path)
		assert.Empty(t, decision.Targets[2].Mirror, path)
	}

	decision, err := Resolve(constant.Host, blobPath(layerDigest), spec, self)
	require.NoError(t, err)
	assert.Equal(t, "/v2/d8-proxy/deckhouse/ee/modules/ml/blobs/"+layerDigest, decision.Targets[0].Path)
	assert.Equal(t, "/v2/other/modules/ml/blobs/"+layerDigest, decision.Targets[1].Path)

	// A tag is resolved where it is resolved without a mirror.
	for _, path := range []string{
		"/v2" + constant.Path + "/modules/ml/manifests/v1.2.3",
		"/v2" + constant.Path + "/modules/ml/tags/list",
		"/v2" + constant.Path + "/modules/ml/blobs/uploads/",
	} {
		decision, err := Resolve(constant.Host, path, spec, self)
		require.NoError(t, err, path)
		require.Len(t, decision.Targets, 1, path)
		assert.Empty(t, decision.Targets[0].Mirror, path)
	}

	// Nor is anything outside the primary image set sent to it.
	routed := spec.DeepCopy()
	routed.AdditionalRoutes = []registryv1alpha1.Route{{
		Match: "ghcr.io", Endpoint: registryv1alpha1.Endpoint{Host: "proxy.example.com"},
	}}
	decision, err = Resolve("ghcr.io", "/v2/org/image/blobs/"+layerDigest, routed, self)
	require.NoError(t, err)
	for _, target := range decision.Targets {
		assert.Empty(t, target.Mirror)
	}
}

func TestDigestReference(t *testing.T) {
	for remainder, want := range map[string]string{
		"/blobs/" + layerDigest:      layerDigest,
		"/manifests/" + layerDigest:  layerDigest,
		"/manifests/sha512:abc":      "sha512:abc",
		"/manifests/latest":          "",
		"/manifests/sha256:":         "",
		"/blobs/uploads/":            "",
		"/blobs/uploads/sha256:abc":  "",
		"/tags/list":                 "",
		"/referrers/" + layerDigest:  "",
		"/manifests/:abc":            "",
		"/blobs/sha256:abc/anything": "",
	} {
		assert.Equal(t, want, digestReference(remainder), remainder)
	}
}

func TestServeTakesADigestFromTheMirror(t *testing.T) {
	mirror := (&registryStub{name: "mirror", body: layerContent}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	server := withMetrics(newServer(mirrorLayout(t, mirror, cache)))

	response := pull(t, server, constant.Host, blobPath(layerDigest))

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, layerContent, bodyOf(t, response))
	assert.Equal(t, []string{"/v2" + mirrorPath + "/modules/ml/blobs/" + layerDigest}, mirror.asked())
	assert.Empty(t, cache.asked(), "content the mirror holds does not cross the long link")
	assert.Equal(t, 1.0, mirrorCount(server, metrics.MirrorHit))
}

func TestServeGoesOnWhenTheMirrorDoesNotServe(t *testing.T) {
	for name, stub := range map[string]*registryStub{
		// The ordinary miss: the mirror is up, and does not hold this.
		"missing": {name: "mirror", status: http.StatusNotFound},
		// A mirror asking for credentials the agent was not given. Relayed, the challenge would
		// send the runtime off to the mirror's token service, past every target behind it.
		"anonymous": {name: "mirror", challenge: "basic", username: "u", password: "p"},
		"failing":   {name: "mirror", status: http.StatusServiceUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			mirror := stub.start(t)
			cache := (&registryStub{name: "cache", body: layerContent}).start(t)
			server := withMetrics(newServer(mirrorLayout(t, mirror, cache)))

			response := pull(t, server, constant.Host, blobPath(layerDigest))

			require.Equal(t, http.StatusOK, response.StatusCode)
			assert.Equal(t, layerContent, bodyOf(t, response))
			assert.Len(t, mirror.asked(), 1)
			assert.Len(t, cache.asked(), 1)
		})
	}
}

func TestServeCountsWhatTheMirrorAnswered(t *testing.T) {
	mirror := (&registryStub{name: "mirror", status: http.StatusNotFound}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	server := withMetrics(newServer(mirrorLayout(t, mirror, cache)))

	pull(t, server, constant.Host, blobPath(layerDigest))

	assert.Equal(t, 1.0, mirrorCount(server, metrics.MirrorMiss))
	assert.Zero(t, mirrorCount(server, metrics.MirrorHit))
}

func TestServeNeverRelaysTheMirrorsMiss(t *testing.T) {
	// The cache's own 404 is the answer, as it would be without the mirror.
	mirror := (&registryStub{name: "mirror", status: http.StatusNotFound}).start(t)
	cache := (&registryStub{name: "cache", status: http.StatusNotFound, body: "cache says no"}).start(t)

	response := pull(t, newServer(mirrorLayout(t, mirror, cache)), constant.Host, blobPath(layerDigest))

	assert.Equal(t, http.StatusNotFound, response.StatusCode)
	assert.Equal(t, "cache says no", bodyOf(t, response))

	// And when nothing behind the mirror answers at all, the pull fails as unreachable, not as
	// "not found": the mirror's miss says nothing about whether the image exists.
	unreachable := mirrorLayout(t, mirror, cache)
	unreachable.Backends[0].Host = "127.0.0.1:1"
	response = pull(t, newServer(unreachable), constant.Host, blobPath(layerDigest))
	assert.Equal(t, http.StatusBadGateway, response.StatusCode)
}

func TestServeLeavesTagsToTheBackends(t *testing.T) {
	mirror := (&registryStub{name: "mirror", body: "{}"}).start(t)
	cache := (&registryStub{name: "cache", body: "{}"}).start(t)

	response := pull(t, newServer(mirrorLayout(t, mirror, cache)), constant.Host,
		"/v2"+constant.Path+"/modules/ml/manifests/v1.2.3")

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Empty(t, mirror.asked())
	assert.Len(t, cache.asked(), 1)
}

func TestServeStopsAskingAMirrorForContentItServedWrong(t *testing.T) {
	mirror := (&registryStub{name: "mirror", body: "not the layer"}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	server := withMetrics(newServer(mirrorLayout(t, mirror, cache)))

	// An error with no response is a transfer cut before the headers went out: a small body is still
	// buffered when the transfer is aborted, and the runtime sees the connection close.
	get := func() (*http.Response, []byte, error) { return fetch(t, server, blobPath(layerDigest)) }

	// The first transfer breaks rather than completing with the wrong content.
	_, content, err := get()
	assert.True(t, err != nil || string(content) != "not the layer",
		"the wrong content was relayed whole: %q", content)
	assert.Equal(t, 1.0, mirrorCount(server, metrics.MirrorDigestMismatch))
	assert.Zero(t, mirrorCount(server, metrics.MirrorHit), "wrong content was counted as served")

	// The retry goes past the mirror, and gets the content the digest names.
	response, content, err := get()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, layerContent, string(content))
	assert.Len(t, mirror.asked(), 1, "the mirror was asked again for the digest it served wrong")

	// Until the window closes.
	server.mirrorRefused.now = func() time.Time { return time.Now().Add(MirrorRefusalWindow + time.Minute) }
	_, _, _ = get()
	assert.Len(t, mirror.asked(), 2)
}

func TestServeBreaksALargeWrongTransferMidway(t *testing.T) {
	// Past the server's buffer the headers are out before the end is reached, and what the runtime
	// must see is a body that breaks off, never one that ends cleanly.
	large := strings.Repeat("x", 256<<10)
	sum := sha256.Sum256([]byte(large))
	digest := "sha256:" + hex.EncodeToString(sum[:])

	mirror := (&registryStub{name: "mirror", body: large + "tampered"}).start(t)
	cache := (&registryStub{name: "cache", body: large}).start(t)
	server := newServer(mirrorLayout(t, mirror, cache))

	response, _, err := fetch(t, server, blobPath(digest))
	require.NotNil(t, response, "the transfer broke before the headers went out: %v", err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Error(t, err, "the wrong content ended as if it were complete")
}

func TestServeRelaysARangeFromTheMirrorUnchecked(t *testing.T) {
	// A runtime resuming a transfer asks for the rest of the blob, which cannot be checked against
	// the digest of the whole; the runtime checks the whole when it has it.
	mirror := (&registryStub{name: "mirror", body: "layer"}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	server := withMetrics(newServer(mirrorLayout(t, mirror, cache)))

	front := httptest.NewServer(server.Handler())
	t.Cleanup(front.Close)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		front.URL+blobPath(layerDigest)+"?ns="+url.QueryEscape(constant.Host), nil)
	require.NoError(t, err)
	request.Header.Set("Range", "bytes=4-")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()

	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "layer", string(content))
	assert.Zero(t, mirrorCount(server, metrics.MirrorDigestMismatch))
}

func TestRefusedDigestsStayBounded(t *testing.T) {
	var refused refusedDigests
	start := time.Now()
	tick := 0
	refused.now = func() time.Time { tick++; return start.Add(time.Duration(tick) * time.Millisecond) }

	for i := range maxRefusedDigests + 10 {
		refused.refuse(&Target{Mirror: "gpu-eu", Host: "harbor.eu.example.com",
			Digest: "sha256:" + string(rune('a'+i%26)) + hex.EncodeToString([]byte{byte(i), byte(i >> 8)})})
	}

	assert.Len(t, refused.entries, maxRefusedDigests)
	assert.False(t, refused.refused(&Target{Mirror: "gpu-eu", Host: "harbor.eu.example.com",
		Digest: "sha256:a" + hex.EncodeToString([]byte{0, 0})}),
		"the oldest refusal goes first")
}

func TestADemotedMirrorLeavesTheCachesMissFinal(t *testing.T) {
	// The cache's 404 speaks for the upstream behind it. A mirror moved to the back after failing
	// does not change that: its miss would never have been the answer, so nothing passed over it
	// needs asking on its account.
	cache := (&registryStub{name: "cache", status: http.StatusNotFound}).start(t)
	upstream := (&registryStub{name: "upstream", body: layerContent}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendStorage, cache, t, constant.Path, nil),
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
		PrimaryMirror: &registryv1alpha1.PrimaryMirror{
			Name: "gpu-eu",
			Upstream: registryv1alpha1.Upstream{Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: "127.0.0.1:1", Path: mirrorPath,
			}},
		},
	}
	server := newServer(spec)

	for range 2 {
		response := pull(t, server, constant.Host, blobPath(layerDigest))
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	}
	assert.Empty(t, upstream.asked())
}

func TestServeKeepsTheClientsCredentialsFromTheMirror(t *testing.T) {
	// A mirror the cluster has no credentials for is asked anonymously, whatever the client offers:
	// the client's credentials were meant for whoever challenged it, never for the mirror.
	mirror := (&registryStub{name: "mirror", body: layerContent}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)

	sent := pullWithHeaders(t, newServer(mirrorLayout(t, mirror, cache)), constant.Host, blobPath(layerDigest),
		map[string]string{"Authorization": "Basic dXNlcm5hbWU6cGFzc3dvcmQ=", "Accept-Encoding": "br"}, mirror)

	assert.Empty(t, sent.Get("Authorization"))
	// Not the client's: the agent's own transport asks for what it can decode, and decodes it.
	assert.NotContains(t, sent.Get("Accept-Encoding"), "br")
}

// encodedMirror answers every request with the given encoding and body.
func encodedMirror(t *testing.T, encoding string, body []byte) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Encoding", encoding)
		_, _ = writer.Write(body)
	}))
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	return parsed.Host
}

func TestServeChecksEncodedMirrorContentDecoded(t *testing.T) {
	// A proxy in front of the mirror may compress on its own. Gzip is decoded on the way in, so what
	// is checked — and relayed — is the content the digest is of.
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write([]byte(layerContent))
	require.NoError(t, writer.Close())

	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	spec := mirrorLayout(t, cache, cache)
	spec.PrimaryMirror.Host = encodedMirror(t, "gzip", compressed.Bytes())
	server := withMetrics(newServer(spec))

	response := pull(t, server, constant.Host, blobPath(layerDigest))

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, layerContent, bodyOf(t, response))
	assert.Empty(t, cache.asked())
	assert.Equal(t, 1.0, mirrorCount(server, metrics.MirrorHit))
	assert.Zero(t, mirrorCount(server, metrics.MirrorDigestMismatch))
}

func TestServeRelaysContentInAnEncodingItCannotDecodeUnchecked(t *testing.T) {
	// An encoding the agent does not decode cannot be checked against the digest of the content, and
	// correct content must not be refused for an hour over that. The runtime checks it decoded.
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	spec := mirrorLayout(t, cache, cache)
	spec.PrimaryMirror.Host = encodedMirror(t, "br", []byte("brotli bytes"))
	server := withMetrics(newServer(spec))

	response := pull(t, server, constant.Host, blobPath(layerDigest))

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "brotli bytes", bodyOf(t, response))
	assert.Equal(t, 1.0, mirrorCount(server, metrics.MirrorHit))
	assert.Zero(t, mirrorCount(server, metrics.MirrorDigestMismatch))
	assert.Empty(t, server.mirrorRefused.entries)
}

func TestServeNeverAsksTheAgentItselfAsAMirror(t *testing.T) {
	mirror := (&registryStub{name: "mirror", body: layerContent}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	server := newServer(mirrorLayout(t, mirror, cache))
	server.Self = mirror.host(t)

	response := pull(t, server, constant.Host, blobPath(layerDigest))

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Empty(t, mirror.asked())
	assert.Len(t, cache.asked(), 1)
}

func TestServeAsksARefusingMirrorLast(t *testing.T) {
	// A mirror that refuses — wrong credentials, here — is passed over for a while rather than paying
	// a token exchange on every request by digest.
	mirror := (&registryStub{name: "mirror", challenge: "basic", username: "u", password: "p"}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	spec := mirrorLayout(t, mirror, cache)
	spec.PrimaryMirror.Auth = &registryv1alpha1.Auth{Username: "u", Password: "wrong"}
	server := newServer(spec)

	for range 3 {
		response := pull(t, server, constant.Host, blobPath(layerDigest))
		require.Equal(t, http.StatusOK, response.StatusCode)
	}
	assert.Len(t, mirror.asked(), 1, "a refusing mirror was asked first again inside its demotion")
	assert.Len(t, cache.asked(), 3)
}
