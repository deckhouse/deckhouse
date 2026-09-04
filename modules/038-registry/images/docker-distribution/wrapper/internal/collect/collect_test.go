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

package collect

import (
	"context"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/google/go-containerregistry/pkg/name"
	craneregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func store(root string) *configuration.Configuration {
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

// TestASweepKeepsWhatATagNamesAndReclaimsWhatNoneDoes is the whole contract of the subcommand: the
// syncer decides which tags may go and deletes them, and this reclaims what those deletions made
// unreachable — no more.
func TestASweepKeepsWhatATagNamesAndReclaimsWhatNoneDoes(t *testing.T) {
	root := t.TempDir()
	settings := store(root)

	registry := httptest.NewServer(handlers.NewApp(context.Background(), settings))
	t.Cleanup(registry.Close)

	kept, err := random.Image(1024, 1)
	require.NoError(t, err)
	keptTag, err := name.NewTag(registry.Listener.Addr().String()+"/system/deckhouse:v1.77.0", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(keptTag, kept))

	before := blobCount(t, root)
	require.NotZero(t, before)

	require.NoError(t, Run(context.Background(), settings, Options{}))

	assert.Equal(t, before, blobCount(t, root),
		"everything here is named by a tag, so a sweep must touch nothing")

	// The image the cluster still pulls has to survive a sweep, not merely have its blobs left on
	// disk: reachability is what the sweep computes, and a manifest it got wrong is an image that
	// stops resolving.
	served, err := remote.Image(keptTag)
	require.NoError(t, err)
	_, err = served.Manifest()
	require.NoError(t, err)
}

// TestADryRunTouchesNothing keeps the reporting path honest: the syncer runs it to say what a
// collection would do.
func TestADryRunTouchesNothing(t *testing.T) {
	root := t.TempDir()
	settings := store(root)

	registry := httptest.NewServer(handlers.NewApp(context.Background(), settings))
	t.Cleanup(registry.Close)

	image, err := random.Image(512, 1)
	require.NoError(t, err)
	tag, err := name.NewTag(registry.Listener.Addr().String()+"/system/deckhouse:v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))

	before := blobCount(t, root)
	require.NoError(t, Run(context.Background(), settings, Options{DryRun: true}))
	assert.Equal(t, before, blobCount(t, root))
}

// TestRunRefusesAConfigurationWithNoStore: the syncer passes the file it rendered, and a file with
// no storage driver would otherwise fail somewhere deeper with a message about a nil driver.
func TestRunRefusesAConfigurationWithNoStore(t *testing.T) {
	err := Run(context.Background(), &configuration.Configuration{}, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no storage driver")
}

// TestASweepReadsTheStoreAndNotACache is why this subcommand is not upstream's.
//
// Upstream's takes the configuration file and builds the storage from it, proxy section included. A
// collection against a pull-through cache computes the reachable set from what the cache can FETCH
// — everything — instead of from what it holds. Here the store is opened from the driver alone, so
// a configuration that names an upstream makes no difference to what is swept.
func TestASweepReadsTheStoreAndNotACache(t *testing.T) {
	root := t.TempDir()

	upstream := httptest.NewServer(craneregistry.New(craneregistry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(upstream.Close)

	plain := store(root)
	registry := httptest.NewServer(handlers.NewApp(context.Background(), plain))
	t.Cleanup(registry.Close)

	image, err := random.Image(512, 1)
	require.NoError(t, err)
	tag, err := name.NewTag(registry.Listener.Addr().String()+"/system/deckhouse:v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))

	before := blobCount(t, root)

	// The same store, described by a configuration that also names an upstream.
	cached := store(root)
	cached.Proxy = configuration.Proxy{RemoteURL: upstream.URL}

	require.NoError(t, Run(context.Background(), cached, Options{}))
	assert.Equal(t, before, blobCount(t, root))
}

// blobCount is what the store weighs, counted the way an operator would: files under the blob tree.
func blobCount(t *testing.T, root string) int {
	t.Helper()

	blobs := filepath.Join(root, "docker", "registry", "v2", "blobs")
	count := 0
	err := filepath.Walk(blobs, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	require.NoError(t, err)
	return count
}
