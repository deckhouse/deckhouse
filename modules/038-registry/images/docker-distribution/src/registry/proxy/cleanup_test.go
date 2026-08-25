package proxy

import (
	"context"
	"testing"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry/storage"
	"github.com/docker/distribution/registry/storage/driver/inmemory"
)

// TestSkipModeCleanupIsWhatKeepsASharedStore covers the case two processes over one directory create.
//
// The cleanup exists so that data written as a proxy cache is never served as if it were local, and it
// decides "the mode changed" from whether the scheduler state file is present. That is sound when one
// registry owns the directory. Deckhouse runs two over the same store — one proxying for reads, one
// accepting the pushes that fill it — so the non-proxying process finds the other's scheduler state on
// every start and deletes /docker. Measured on a three-master cluster: 3236 files gone two seconds
// after the push instance came up, and once at the very moment the upstream was dropped for an
// air-gap, which left the cluster with no images and nowhere to pull them from.
func TestSkipModeCleanupIsWhatKeepsASharedStore(t *testing.T) {
	ctx := context.Background()

	for _, testcase := range []struct {
		name  string
		state bool // whether the scheduler state file exists, which is what "mode" is read from
		kept  bool
	}{
		{name: "a non-cache start with cache state cleans up by default", state: true, kept: false},
		{name: "a non-cache start with no cache state keeps the store", state: false, kept: true},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			driver := inmemory.New()
			if err := driver.PutContent(ctx, "/docker/registry/v2/blobs/x", []byte("blob")); err != nil {
				t.Fatal(err)
			}
			if testcase.state {
				if err := driver.PutContent(ctx, schedulerStateFilePath, []byte("{}")); err != nil {
					t.Fatal(err)
				}
			}

			if err := CleanupCacheStorage(ctx, driver); err != nil {
				t.Fatalf("cleanup: %v", err)
			}

			_, err := driver.GetContent(ctx, "/docker/registry/v2/blobs/x")
			kept := err == nil
			if kept != testcase.kept {
				t.Fatalf("store kept = %v, want %v", kept, testcase.kept)
			}
		})
	}

	// And the flag itself, asserted where it is actually read: building a pull-through cache over a
	// store that carries no scheduler state would clean it up, and must not when the flag is set.
	t.Run("a proxy start with skipmodecleanup keeps a store it did not write", func(t *testing.T) {
		driver := inmemory.New()
		if err := driver.PutContent(ctx, "/docker/registry/v2/blobs/x", []byte("blob")); err != nil {
			t.Fatal(err)
		}

		local, err := storage.NewRegistry(ctx, driver)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := NewRegistryPullThroughCache(ctx, local, driver, configuration.Proxy{
			RemoteURL:       "https://example.com",
			SkipModeCleanup: true,
		}); err != nil {
			t.Fatalf("building the cache: %v", err)
		}

		if _, err := driver.GetContent(ctx, "/docker/registry/v2/blobs/x"); err != nil {
			t.Fatalf("the store had to survive, and did not: %v", err)
		}
	})

	// The same construction without the flag is what deletes it, which is the behaviour being opted
	// out of rather than removed.
	t.Run("without the flag the same start empties the store", func(t *testing.T) {
		driver := inmemory.New()
		if err := driver.PutContent(ctx, "/docker/registry/v2/blobs/x", []byte("blob")); err != nil {
			t.Fatal(err)
		}

		local, err := storage.NewRegistry(ctx, driver)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := NewRegistryPullThroughCache(ctx, local, driver, configuration.Proxy{
			RemoteURL: "https://example.com",
		}); err != nil {
			t.Fatalf("building the cache: %v", err)
		}

		if _, err := driver.GetContent(ctx, "/docker/registry/v2/blobs/x"); err == nil {
			t.Fatal("the default is still to clean up, and this store was kept")
		}
	})
}
