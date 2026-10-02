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
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	"github.com/google/go-containerregistry/pkg/name"
	craneregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/registry-distribution/internal/bound"
	"github.com/deckhouse/registry-distribution/internal/config"
)

// blobBytes is what the store holds under blobs/, which is where a kept cache miss would land.
func blobBytes(t *testing.T, root string) int64 {
	t.Helper()
	var total int64
	blobs := filepath.Join(root, "docker", "registry", "v2", "blobs")
	err := filepath.WalkDir(blobs, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	require.NoError(t, err)
	return total
}

// TestACacheMissOverTheBudgetReachesTheClientWhole is the serving half of a full store, and the
// reason it drops rather than fails: distribution streams a miss to the client and into the store
// through one writer, so a refused write there would cut the pull off mid-layer.
func TestACacheMissOverTheBudgetReachesTheClientWhole(t *testing.T) {
	upstream := httptest.NewServer(craneregistry.New(craneregistry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(upstream.Close)

	image, err := random.Image(1<<20, 2)
	require.NoError(t, err)
	tag, err := name.NewTag(upstream.Listener.Addr().String()+"/system/deckhouse/big:v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))

	root := t.TempDir()
	guard := bound.New(root, 256<<10, 1)
	require.NoError(t, guard.Measure())

	never := time.Duration(0)
	serving := storeConfiguration(root)
	serving.Proxy = configuration.Proxy{RemoteURL: upstream.URL, TTL: &never}
	serving.Middleware = bounded(nil, guard, bound.Discard)

	cache := httptest.NewServer(handlers.NewApp(context.Background(), serving))
	t.Cleanup(cache.Close)

	pull, err := name.NewTag(cache.Listener.Addr().String()+"/system/deckhouse/big:v1", name.Insecure)
	require.NoError(t, err)
	pulled, err := remote.Image(pull)
	require.NoError(t, err)
	layers, err := pulled.Layers()
	require.NoError(t, err)
	for _, layer := range layers {
		// The digest is verified as the layer is read, so a truncated stream fails here. The
		// "superfluous WriteHeader" distribution logs meanwhile is its own attempt to report the
		// failed commit after the body has already gone to the client.
		content, err := layer.Compressed()
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, content)
		require.NoError(t, err, "the client lost a layer the store could not keep")
		require.NoError(t, content.Close())
	}

	assert.LessOrEqual(t, blobBytes(t, root), int64(256<<10), "the store kept more than its budget")
}

// TestAFullStoreRefusesAPushWith507 is what `d8 mirror push` and the fill see: a status that says
// what happened, and a message that says what to do.
func TestAFullStoreRefusesAPushWith507(t *testing.T) {
	// A store already holding its whole budget, as it is after a fill that ran into it.
	root := t.TempDir()
	blob := filepath.Join(root, "docker", "registry", "v2", "blobs", "sha256", "ab", "abcdef", "data")
	require.NoError(t, os.MkdirAll(filepath.Dir(blob), 0o755))
	require.NoError(t, os.WriteFile(blob, make([]byte, 4096), 0o644))

	guard := bound.New(root, 4096, 1)
	require.NoError(t, guard.Measure())

	wrapper := &config.Wrapper{Scope: "system/deckhouse", WriteEndpoint: config.WriteEndpoint{Address: "127.0.0.1:0"}}
	writing := writeConfiguration(storeConfiguration(root), wrapper, guard)
	write := httptest.NewServer(bound.Handler(guard, handlers.NewApp(context.Background(), writing)))
	t.Cleanup(write.Close)

	upload := func(authorization string) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, write.URL+"/v2/system/deckhouse/x/blobs/uploads/", nil)
		require.NoError(t, err)
		if authorization != "" {
			request.Header.Set("Authorization", authorization)
		}
		response, err := write.Client().Do(request)
		require.NoError(t, err)
		t.Cleanup(func() { _ = response.Body.Close() })
		return response
	}

	refused := upload("Bearer token")
	assert.Equal(t, http.StatusInsufficientStorage, refused.StatusCode)

	var body struct {
		Errors []struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Detail  map[string]any `json:"detail"`
		} `json:"errors"`
	}
	require.NoError(t, json.NewDecoder(refused.Body).Decode(&body))
	require.Len(t, body.Errors, 1)
	assert.Contains(t, body.Errors[0].Message, "storage.size")
	assert.Equal(t, string(bound.BudgetExhausted), body.Errors[0].Detail["reason"])

	// A client that has not presented credentials is not told about the disk; it gets whatever
	// distribution answers, which in a real cluster is the token challenge.
	assert.NotEqual(t, http.StatusInsufficientStorage, upload("").StatusCode)

	// And the store still serves reads.
	response, err := write.Client().Get(write.URL + "/v2/")
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

// TestTheTwoHalvesShareOneGuardInTheirOwnModes: one budget for the process, spent by whichever half
// writes, and each half refusing in the way its client can live with.
func TestTheTwoHalvesShareOneGuardInTheirOwnModes(t *testing.T) {
	guard := unbounded(t, t.TempDir())
	serving := storeConfiguration(t.TempDir())
	serving.Middleware = map[string][]configuration.Middleware{
		"registry": {{Name: "something-else"}},
	}
	serving.Middleware = bounded(serving.Middleware, guard, bound.Discard)

	writing := writeConfiguration(serving, &config.Wrapper{
		Scope:         "system/deckhouse",
		WriteEndpoint: config.WriteEndpoint{Address: "0.0.0.0:5003"},
	}, guard)

	mode := func(settings *configuration.Configuration) any {
		entries := settings.Middleware["storage"]
		require.Len(t, entries, 1, "exactly one guard in front of the storage")
		assert.Same(t, guard, entries[0].Options["guard"])
		return entries[0].Options["onrefusal"]
	}
	assert.Equal(t, string(bound.Discard), mode(serving))
	assert.Equal(t, string(bound.Fail), mode(writing))
	assert.Equal(t, serving.Middleware["registry"], writing.Middleware["registry"],
		"the rest of the middleware is carried over")
}

func TestRunRefusesAStoreItCannotBound(t *testing.T) {
	settings := &configuration.Configuration{Storage: configuration.Storage{"inmemory": configuration.Parameters{}}}
	err := Run(context.Background(), settings, &config.Wrapper{Scope: "system/deckhouse"}, quiet())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bounded on a filesystem")
}

// TestACacheMissAtTheEdgeIsServedWithoutWritingTheLayers: the store is within a measurement of its
// budget, so the layers of a miss are not written at all — the client gets them from the upstream,
// and only the small manifests, which still fit, are kept.
func TestACacheMissAtTheEdgeIsServedWithoutWritingTheLayers(t *testing.T) {
	upstream := httptest.NewServer(craneregistry.New(craneregistry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(upstream.Close)

	image, err := random.Image(1<<20, 3)
	require.NoError(t, err)
	tag, err := name.NewTag(upstream.Listener.Addr().String()+"/system/deckhouse/edge:v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))

	root := t.TempDir()
	held := filepath.Join(root, "docker", "registry", "v2", "blobs", "sha256", "ab", "abcdef", "data")
	require.NoError(t, os.MkdirAll(filepath.Dir(held), 0o755))
	require.NoError(t, os.WriteFile(held, make([]byte, 100<<20), 0o644))

	guard := bound.New(root, 110<<20, 1)
	require.NoError(t, guard.Measure())

	never := time.Duration(0)
	serving := storeConfiguration(root)
	serving.Proxy = configuration.Proxy{RemoteURL: upstream.URL, TTL: &never}
	serving.Middleware = bounded(nil, guard, bound.Discard)
	cache := httptest.NewServer(handlers.NewApp(context.Background(), serving))
	t.Cleanup(cache.Close)

	pull, err := name.NewTag(cache.Listener.Addr().String()+"/system/deckhouse/edge:v1", name.Insecure)
	require.NoError(t, err)
	pulled, err := remote.Image(pull)
	require.NoError(t, err)
	layers, err := pulled.Layers()
	require.NoError(t, err)
	for _, layer := range layers {
		content, err := layer.Compressed()
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, content)
		require.NoError(t, err)
		require.NoError(t, content.Close())
	}

	kept := blobBytes(t, root) - 100<<20
	assert.Less(t, kept, int64(64<<10), "only the manifest and the config should have been kept")

	// Waited for, not read once. The proxy serves and stores in one pass and learns that the copy
	// was refused only at the commit, after the client has its last byte; the scratch is removed
	// by the cancellation after that. A loaded runner read the directory in between.
	uploads := filepath.Join(root, "docker", "registry", "v2", "repositories", "system", "deckhouse", "edge", "_uploads")
	assert.Eventually(t, func() bool {
		entries, _ := os.ReadDir(uploads)
		return len(entries) == 0
	}, 10*time.Second, 20*time.Millisecond, "a dropped copy left scratch behind")
}
