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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	"github.com/google/go-containerregistry/pkg/name"
	craneregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/registry-distribution/internal/bound"
)

// pullAtTheEdge pulls the blobs of an image through a cache at its limit, over TLS and HTTP/2 — how
// the node agent talks to the store — reading every body to its end, as the agent's transport does.
//
// Small blobs, and that is the point: HTTP/2 refuses a write beyond the declared length only once
// the headers are on the wire, and a body smaller than the stream's buffer has not sent them yet
// when distribution appends its error. The layers of a real image are large and pass; its config
// blob is two kilobytes and does not.
func pullAtTheEdge(t *testing.T, wrap func(http.Handler) http.Handler) error {
	t.Helper()

	upstream := httptest.NewServer(craneregistry.New(craneregistry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(upstream.Close)
	image, err := random.Image(1024, 2)
	require.NoError(t, err)
	tag, err := name.NewTag(upstream.Listener.Addr().String()+"/system/deckhouse/edge:v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))

	root := t.TempDir()
	// Budget of one byte: every copy is dropped from the start, and every manifest refused.
	guard := bound.New(root, 1, 1)
	require.NoError(t, guard.Measure())

	never := time.Duration(0)
	serving := storeConfiguration(root)
	serving.Proxy = configuration.Proxy{RemoteURL: upstream.URL, TTL: &never}
	serving.Middleware = bounded(nil, guard, bound.Discard)

	cache := httptest.NewUnstartedServer(wrap(handlers.NewApp(context.Background(), serving)))
	cache.EnableHTTP2 = true
	cache.StartTLS()
	t.Cleanup(cache.Close)

	// Layers only: the manifest is refused by a full store and the agent takes it from the
	// upstream, which is not what is under test. What is, is a blob served from a copy dropped.
	layers, err := image.Layers()
	require.NoError(t, err)
	client := cache.Client()
	for _, layer := range layers {
		digest, err := layer.Digest()
		require.NoError(t, err)
		response, err := client.Get(cache.URL + "/v2/system/deckhouse/edge/blobs/" + digest.String())
		if err != nil {
			return err
		}
		require.Equal(t, 2, response.ProtoMajor, "the test must speak HTTP/2 to mean anything")
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			return &statusError{code: response.StatusCode}
		}
		size, err := layer.Size()
		require.NoError(t, err)
		if int64(len(body)) != size {
			return &statusError{code: -len(body)}
		}
	}
	return nil
}

type statusError struct{ code int }

func (e *statusError) Error() string { return http.StatusText(e.code) }

// TestADroppedCopyReachesTheClientIntact is what a live cluster found and the tests before it did
// not: distribution answers a failed commit AFTER the blob has gone out, appending its error to a
// body whose length it has already declared. A client that reads to the end — the node agent does
// — sees a response longer than promised, drops it as interrupted, and the pull fails with EOF.
func TestADroppedCopyReachesTheClientIntact(t *testing.T) {
	require.NoError(t, pullAtTheEdge(t, settle))
}

// TestWithoutSettlingTheStreamBreaks keeps the reason for settle on record: the same pull, with
// distribution's handler as it is.
func TestWithoutSettlingTheStreamBreaks(t *testing.T) {
	require.Error(t, pullAtTheEdge(t, func(handler http.Handler) http.Handler { return handler }))
}
