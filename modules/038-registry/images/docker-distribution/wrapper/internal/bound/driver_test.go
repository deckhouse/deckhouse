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

package bound

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/distribution/distribution/v3/registry/storage/driver/factory"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	storagemiddleware "github.com/distribution/distribution/v3/registry/storage/driver/middleware"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bounded opens a filesystem store at root with the middleware in front, the way distribution does.
func bounded(t *testing.T, root string, guard *Guard, mode Mode) storagedriver.StorageDriver {
	t.Helper()
	ctx := context.Background()
	inner, err := factory.Create(ctx, "filesystem", map[string]any{"rootdirectory": root})
	require.NoError(t, err)
	outer, err := storagemiddleware.Get(ctx, MiddlewareName, Options(guard, mode), inner)
	require.NoError(t, err)
	return outer
}

func randomBytes(t *testing.T, size int) []byte {
	t.Helper()
	content := make([]byte, size)
	_, err := rand.Read(content)
	require.NoError(t, err)
	return content
}

func onDisk(t *testing.T, root, path string) (int64, bool) {
	t.Helper()
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
	if os.IsNotExist(err) {
		return 0, false
	}
	require.NoError(t, err)
	return info.Size(), true
}

func TestAPushStopsAtTheBudget(t *testing.T) {
	root := t.TempDir()
	guard := New(root, 256<<10, 0)
	guard.statfs = fixed(1 << 40)
	store := bounded(t, root, guard, Fail)

	writer, err := store.Writer(context.Background(), uploadPath, false)
	require.NoError(t, err)

	_, err = io.Copy(writer, bytes.NewReader(randomBytes(t, 1<<20)))
	assert.Equal(t, BudgetExhausted, refusal(t, err).Reason)
	require.NoError(t, writer.Close())

	size, _ := onDisk(t, root, uploadPath)
	assert.LessOrEqual(t, size, int64(256<<10), "the upload went past the budget")
}

// TestADiscardedMissReachesTheClientWholeAndIsNotKept is the serving half's side of a full store:
// the client of a cache miss shares one stream with the local copy, and must not lose it.
func TestADiscardedMissReachesTheClientWholeAndIsNotKept(t *testing.T) {
	root := t.TempDir()
	guard := New(root, 256<<10, 0)
	guard.statfs = fixed(1 << 40)
	store := bounded(t, root, guard, Discard)
	ctx := context.Background()

	writer, err := store.Writer(ctx, uploadPath, false)
	require.NoError(t, err)

	blob := randomBytes(t, 1<<20)
	var client bytes.Buffer
	// The shape of distribution's proxy: one copy, two destinations, stopping at the first error.
	_, err = io.Copy(io.MultiWriter(&client, writer), bytes.NewReader(blob))
	require.NoError(t, err)
	assert.Equal(t, blob, client.Bytes(), "the client did not get the whole blob")

	assert.Equal(t, BudgetExhausted, refusal(t, writer.Commit(ctx)).Reason)
	require.NoError(t, writer.Cancel(ctx))

	_, exists := onDisk(t, root, uploadPath)
	assert.False(t, exists, "a cancelled upload was left behind")
}

func TestARefusalBeforeTheFirstByteFailsAPush(t *testing.T) {
	root := t.TempDir()
	guard := New(root, 0, 100)
	guard.statfs = fixed(50)
	store := bounded(t, root, guard, Fail)
	ctx := context.Background()

	_, err := store.Writer(ctx, uploadPath, false)
	assert.Equal(t, ReserveExhausted, refusal(t, err).Reason)

	err = store.PutContent(ctx, linkPath, []byte("sha256:abcdef"))
	assert.Equal(t, ReserveExhausted, refusal(t, err).Reason)
	_, exists := onDisk(t, root, linkPath)
	assert.False(t, exists)
}

// TestACopyThatWouldNotFitIsNeverWritten: at the edge, a cache miss's local copy is dropped from
// the start rather than written until refused and then deleted — while what is not an upload still
// fails, because a store must not report holding what it does not.
func TestACopyThatWouldNotFitIsNeverWritten(t *testing.T) {
	root := t.TempDir()
	guard := New(root, 0, 100)
	guard.statfs = fixed(50)
	store := bounded(t, root, guard, Discard)
	ctx := context.Background()

	scratch := "/docker/registry/v2/repositories/system/deckhouse/_uploads/0f3a/startedat"
	require.NoError(t, store.PutContent(ctx, scratch, []byte("2026-09-29T00:00:00Z")))

	writer, err := store.Writer(ctx, uploadPath, false)
	require.NoError(t, err)
	n, err := writer.Write(randomBytes(t, 1<<20))
	require.NoError(t, err)
	assert.Equal(t, 1<<20, n)
	assert.Equal(t, int64(1<<20), writer.Size())
	assert.Equal(t, ReserveExhausted, refusal(t, writer.Commit(ctx)).Reason)
	require.NoError(t, writer.Cancel(ctx))

	for _, path := range []string{scratch, uploadPath} {
		_, exists := onDisk(t, root, path)
		assert.False(t, exists, path)
	}

	err = store.PutContent(ctx, linkPath, []byte("sha256:abcdef"))
	assert.Equal(t, ReserveExhausted, refusal(t, err).Reason, "a link is never dropped")
}

// TestACopyIsDroppedWithinAMeasurementOfALimit: not only once a limit is reached — there, every copy
// would still start, write until refused, and be deleted.
func TestACopyIsDroppedWithinAMeasurementOfALimit(t *testing.T) {
	root := t.TempDir()
	guard := New(root, 0, 10<<20)
	guard.statfs = fixed(20 << 20)
	store := bounded(t, root, guard, Discard)
	ctx := context.Background()

	writer, err := store.Writer(ctx, uploadPath, false)
	require.NoError(t, err)
	_, isDropped := writer.(*dropped)
	assert.True(t, isDropped, "10 MiB above the reserve is less than a measurement interval")
	require.NoError(t, writer.Cancel(ctx))

	guard.statfs = fixed(100 << 20)
	writer, err = store.Writer(ctx, uploadPath, false)
	require.NoError(t, err)
	_, isDropped = writer.(*dropped)
	assert.False(t, isDropped)
	require.NoError(t, writer.Cancel(ctx))
}

func TestReadsAndDeletionsPassAFullStore(t *testing.T) {
	root := t.TempDir()
	guard := New(root, 0, 0)
	guard.statfs = fixed(1 << 40)
	store := bounded(t, root, guard, Fail)
	ctx := context.Background()

	require.NoError(t, store.PutContent(ctx, blobPath, []byte("content")))

	guard.statfs = fixed(0)
	guard.reserve = 1

	content, err := store.GetContent(ctx, blobPath)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
	assert.NoError(t, store.Delete(ctx, "/docker/registry/v2/blobs"))
}

func TestTheMiddlewareNeedsAGuardAndAKnownMode(t *testing.T) {
	ctx := context.Background()
	inner, err := factory.Create(ctx, "filesystem", map[string]any{"rootdirectory": t.TempDir()})
	require.NoError(t, err)

	_, err = storagemiddleware.Get(ctx, MiddlewareName, map[string]any{"onrefusal": "fail"}, inner)
	assert.ErrorContains(t, err, "needs a guard")

	_, err = storagemiddleware.Get(ctx, MiddlewareName, Options(New("/", 0, 0), "ignore"), inner)
	assert.ErrorContains(t, err, `"ignore"`)
}

// TestARegistryWithTheMiddlewareRefusesAPushOverBudget runs the middleware where it will run: in a
// registry distribution builds from its configuration.
func TestARegistryWithTheMiddlewareRefusesAPushOverBudget(t *testing.T) {
	root := t.TempDir()
	guard := New(root, 512<<10, 0)
	guard.statfs = fixed(1 << 40)
	require.NoError(t, guard.Measure())

	settings := &configuration.Configuration{
		Storage: configuration.Storage{
			"filesystem": configuration.Parameters{"rootdirectory": root},
		},
		Middleware: map[string][]configuration.Middleware{
			"storage": {{Name: MiddlewareName, Options: Options(guard, Fail)}},
		},
	}
	settings.HTTP.Secret = "test"
	settings.Log.Level = "error"
	settings.Log.AccessLog.Disabled = true

	registry := httptest.NewServer(handlers.NewApp(context.Background(), settings))
	t.Cleanup(registry.Close)
	host := registry.Listener.Addr().String()

	small, err := random.Image(64<<10, 2)
	require.NoError(t, err)
	reference, err := name.ParseReference(host+"/system/small:v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, small))

	large, err := random.Image(1<<20, 2)
	require.NoError(t, err)
	reference, err = name.ParseReference(host+"/system/large:v1", name.Insecure)
	require.NoError(t, err)
	// No retries: a refusal is not transient, and the client's backoff only makes the test slow.
	err = remote.Write(reference, large, remote.WithRetryBackoff(remote.Backoff{Steps: 1}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage.size")

	require.NoError(t, guard.Measure())
	assert.LessOrEqual(t, guard.base, int64(512<<10), "the store holds more than its budget")
}

// TestATagIsNotRecordedWhileTheStoreRefuses: a miss records its tag before its manifest, and the
// manifest is refused — so the tag would name an image the store does not hold.
func TestATagIsNotRecordedWhileTheStoreRefuses(t *testing.T) {
	tagLink := "/docker/registry/v2/repositories/system/deckhouse/install/_manifests/tags/pr203/current/link"
	ctx := context.Background()

	for _, full := range []bool{true, false} {
		root := t.TempDir()
		guard := New(root, 0, 100)
		guard.statfs = fixed(1 << 40)
		if full {
			guard.statfs = fixed(50)
		}

		require.NoError(t, bounded(t, root, guard, Discard).PutContent(ctx, tagLink, []byte("sha256:abcdef")))
		_, exists := onDisk(t, root, tagLink)
		assert.Equal(t, !full, exists, "full=%v", full)
	}

	// The write endpoint is not a cache: a push that tags while the store refuses is told so.
	root := t.TempDir()
	guard := New(root, 0, 100)
	guard.statfs = fixed(50)
	err := bounded(t, root, guard, Fail).PutContent(ctx, tagLink, []byte("sha256:abcdef"))
	assert.Equal(t, ReserveExhausted, refusal(t, err).Reason)
}
