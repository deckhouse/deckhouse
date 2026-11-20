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
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-agent/internal/metrics"
)

func digestOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func failovers(server *Server, target, reason string) float64 {
	var metric dto.Metric
	if err := server.Metrics.Failovers.WithLabelValues(target, reason).Write(&metric); err != nil {
		return -1
	}

	return metric.GetCounter().GetValue()
}

// fetch asks the agent for a path the way a runtime does, and reads the whole body.
func fetch(t *testing.T, server *Server, path string) (*http.Response, []byte, error) {
	t.Helper()

	front := httptest.NewServer(server.Handler())
	t.Cleanup(front.Close)

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		front.URL+path+"?ns="+url.QueryEscape(constant.Host), nil)
	require.NoError(t, err)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = response.Body.Close() }()

	content, err := io.ReadAll(response.Body)

	return response, content, err
}

// h2Mirror is a mirror reached over TLS and HTTP/2, as a real Harbor is, that serves body for every
// request, with its length declared. The end of the stream is sent a moment after the last data,
// which is when a client that has all the declared bytes is done and hangs up.
func h2Mirror(t *testing.T, body string) (host, authority string) {
	t.Helper()

	mirror := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = writer.Write([]byte(body))
		writer.(http.Flusher).Flush()
		time.Sleep(100 * time.Millisecond)
	}))
	mirror.EnableHTTP2 = true
	mirror.StartTLS()
	t.Cleanup(mirror.Close)

	parsed, err := url.Parse(mirror.URL)
	require.NoError(t, err)

	return parsed.Host, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: mirror.Certificate().Raw}))
}

func h2MirrorLayout(t *testing.T, body string, cache *registryStub) *registryv1alpha1.RegistryNodeSpec {
	t.Helper()

	spec := mirrorLayout(t, cache, cache)
	host, authority := h2Mirror(t, body)
	spec.PrimaryMirror.Scheme = registryv1alpha1.SchemeHTTPS
	spec.PrimaryMirror.Host = host
	spec.PrimaryMirror.CA = authority

	return spec
}

func TestServeBreaksAWrongTransferOverHTTP2(t *testing.T) {
	// Over HTTP/2 the last data comes with no error and the end of the stream only with the next
	// read. Every declared byte of the wrong content reached the runtime before the end was seen,
	// the runtime took the transfer as finished, and the agent never counted or refused it.
	for _, size := range []int{4 << 10, 64 << 10, 1 << 20} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			right := strings.Repeat("x", size)
			cache := (&registryStub{name: "cache", body: right}).start(t)
			server := withMetrics(newServer(h2MirrorLayout(t, strings.Repeat("y", size), cache)))

			response, content, err := fetch(t, server, blobPath(digestOf(right)))
			if err == nil {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				assert.Less(t, len(content), size, "the wrong content was relayed whole")
			}

			assert.Equal(t, 1.0, mirrorCount(server, metrics.MirrorDigestMismatch))
			assert.Zero(t, mirrorCount(server, metrics.MirrorHit))

			// The retry goes past the mirror.
			_, content, err = fetch(t, server, blobPath(digestOf(right)))
			require.NoError(t, err)
			assert.Equal(t, right, string(content))
			assert.Len(t, cache.asked(), 1)
		})
	}
}

func TestServeRelaysRightContentWholeOverHTTP2(t *testing.T) {
	right := strings.Repeat("x", 256<<10)
	cache := (&registryStub{name: "cache", body: right}).start(t)
	server := withMetrics(newServer(h2MirrorLayout(t, right, cache)))

	response, content, err := fetch(t, server, blobPath(digestOf(right)))

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, right, string(content))
	assert.Empty(t, cache.asked())
	assert.Equal(t, 1.0, mirrorCount(server, metrics.MirrorHit))
}

func TestVerifyingBodyHoldsTheLastByteUntilTheEnd(t *testing.T) {
	const content = "the layer, read a byte at a time"

	read := func(body string, wrap func(io.Reader) io.Reader) (string, error, int, int) {
		var verified, mismatched int
		verifying := &verifyingBody{
			// A bytes.Reader ends the way an HTTP/2 body does: the last data with no error, then
			// io.EOF on its own.
			body:     io.NopCloser(bytes.NewReader([]byte(body))),
			hash:     sha256.New(),
			expected: strings.TrimPrefix(digestOf(content), "sha256:"),
			verified: func() { verified++ },
			mismatch: func() { mismatched++ },
		}
		out, err := io.ReadAll(wrap(verifying))

		return string(out), err, verified, mismatched
	}

	for name, wrap := range map[string]func(io.Reader) io.Reader{
		"whole":       func(r io.Reader) io.Reader { return r },
		"byte a time": iotest.OneByteReader,
	} {
		t.Run(name, func(t *testing.T) {
			out, err, verified, mismatched := read(content, wrap)
			require.NoError(t, err)
			assert.Equal(t, content, out)
			assert.Equal(t, 1, verified)
			assert.Zero(t, mismatched)

			out, err, verified, mismatched = read(strings.ToUpper(content), wrap)
			require.ErrorIs(t, err, errDigestMismatch)
			assert.Less(t, len(out), len(content), "every byte of the wrong content went out")
			assert.Zero(t, verified)
			assert.Equal(t, 1, mismatched)
		})
	}
}

func TestServeTakesAPublicTokenMirror(t *testing.T) {
	// A public Harbor project still wants a token, issued to anyone. The challenge cannot go to the
	// runtime, so the agent fetches the token itself, with no credentials, and keeps it.
	mirror := (&registryStub{name: "mirror", challenge: "bearer", public: true, body: layerContent}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	server := withMetrics(newServer(mirrorLayout(t, mirror, cache)))

	for range 2 {
		response := pull(t, server, constant.Host, blobPath(layerDigest))
		require.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, layerContent, bodyOf(t, response))
	}

	assert.Empty(t, cache.asked())
	assert.Equal(t, 2.0, mirrorCount(server, metrics.MirrorHit))
	assert.EqualValues(t, 1, mirror.tokens.Load(), "the anonymous token was not kept")
	assert.Equal(t, "Bearer issued-mirror", mirror.headers.Load().(http.Header).Get("Authorization"))
}

func TestServeGoesPastAPrivateTokenMirrorWithoutCredentials(t *testing.T) {
	mirror := (&registryStub{name: "mirror", challenge: "bearer", username: "u", password: "p"}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	server := withMetrics(newServer(mirrorLayout(t, mirror, cache)))

	response := pull(t, server, constant.Host, blobPath(layerDigest))

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, layerContent, bodyOf(t, response))
	assert.Equal(t, 1.0, mirrorCount(server, metrics.MirrorRefused))
	assert.Equal(t, 1.0, failovers(server, TargetMirror, metrics.ReasonRefused))
	assert.Zero(t, failovers(server, TargetMirror, metrics.ReasonUnreachable),
		"a token service turning the request down is not a mirror out of reach")
}

func TestAnonymousTokensAreKeptApart(t *testing.T) {
	anonymous := &Target{Host: "harbor.eu.example.com", Path: blobPath(layerDigest)}
	credentialed := &Target{Host: anonymous.Host, Path: anonymous.Path, Auth: &registryv1alpha1.Auth{Username: "u"}}

	assert.NotEqual(t, tokenKeyFor(anonymous, "repository:x:pull"), tokenKeyFor(credentialed, "repository:x:pull"))
}

func TestMirrorDemotionIsApartFromTheSameHost(t *testing.T) {
	// One registry serves the mirror's project and an upstream's: the mirror failing must not move
	// the upstream's traffic, and the upstream answering must not lift the mirror's demotion.
	mirror := &Target{Name: TargetMirror, Mirror: "gpu-eu", Host: "harbor.corp.example"}
	upstream := &Target{Name: "Upstream", Host: "HARBOR.corp.example"}

	var demoted demotions
	demoted.fail(mirror, time.Now())
	assert.True(t, demoted.demoted(mirror))
	assert.False(t, demoted.demoted(upstream))

	demoted.succeed(upstream)
	assert.True(t, demoted.demoted(mirror))
}

func TestARefusedDigestStaysWithTheAddressThatServedIt(t *testing.T) {
	// The spare addresses may be replicas of their own; one bad copy says nothing about the others.
	primary := &Target{Mirror: "gpu-eu", Host: "harbor.eu.example.com", Digest: layerDigest}
	spare := &Target{Mirror: "gpu-eu", Host: "harbor-2.eu.example.com", Digest: layerDigest}

	var refused refusedDigests
	refused.refuse(primary)

	assert.True(t, refused.refused(primary))
	assert.False(t, refused.refused(spare))
}

func TestServeStopsWhenTheClientGivesUp(t *testing.T) {
	// A pull the runtime abandoned says nothing about the mirror, and the targets behind it would
	// only fail the same way on the same context.
	mirror := (&registryStub{name: "mirror", delay: time.Second, body: layerContent}).start(t)
	cache := (&registryStub{name: "cache", body: layerContent}).start(t)
	server := withMetrics(newServer(mirrorLayout(t, mirror, cache)))

	front := httptest.NewServer(server.Handler())
	t.Cleanup(front.Close)

	client := &http.Client{Timeout: 200 * time.Millisecond}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		front.URL+blobPath(layerDigest)+"?ns="+url.QueryEscape(constant.Host), nil)
	require.NoError(t, err)

	response, err := client.Do(request)
	if err == nil {
		_ = response.Body.Close()
	}
	require.Error(t, err)

	assert.Eventually(t, func() bool {
		var metric dto.Metric
		_ = server.Metrics.Requests.WithLabelValues(string(KindPrimary), metrics.ResultAllFailed).Write(&metric)

		return metric.GetCounter().GetValue() == 1
	}, 3*time.Second, 20*time.Millisecond)
	assert.Zero(t, mirrorCount(server, metrics.MirrorUnavailable))
	assert.Zero(t, failovers(server, TargetMirror, metrics.ReasonUnreachable))
	assert.Empty(t, cache.asked())
}
