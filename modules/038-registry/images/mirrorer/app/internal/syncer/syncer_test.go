/*
Copyright 2025 Flant JSC

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

package syncer

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// TestSync_PushFailure_FailedCounterIncremented verifies that when a push to the
// destination registry fails, Sync returns nil but Failed() reports a non-zero count.
// This is the core correctness property: swallowing per-tag errors is intentional for
// Run (continuous mode), but callers such as RunOnce must be able to detect partial failures
// by inspecting Failed().
func TestSync_PushFailure_FailedCounterIncremented(t *testing.T) {
	// --- Source registry: real in-memory registry serving one image ---
	srcHandler := registry.New()
	srcServer := httptest.NewServer(srcHandler)
	defer srcServer.Close()

	srcReg, err := name.NewRegistry(srcServer.Listener.Addr().String(), name.Insecure)
	if err != nil {
		t.Fatalf("parse src registry: %v", err)
	}

	// Push one tiny image to the source registry.
	img, err := random.Image(512, 1)
	if err != nil {
		t.Fatalf("create random image: %v", err)
	}

	srcRef := srcReg.Repo("testimage").Tag("latest")
	if err = remote.Write(srcRef, img, remote.WithTransport(http.DefaultTransport)); err != nil {
		t.Fatalf("write image to src: %v", err)
	}

	// --- Destination registry: always rejects writes (PUT/POST → 500) ---
	dstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut || r.Method == http.MethodPost {
			http.Error(w, "simulated failure", http.StatusInternalServerError)
			return
		}
		// Allow GET/HEAD so the puller can check the manifest (returns 404 → copy needed).
		http.NotFound(w, r)
	}))
	defer dstServer.Close()

	dstReg, err := name.NewRegistry(dstServer.Listener.Addr().String(), name.Insecure)
	if err != nil {
		t.Fatalf("parse dst registry: %v", err)
	}

	s := &syncer{
		Src:        srcReg,
		Dst:        dstReg,
		SrcOptions: []remote.Option{remote.WithTransport(http.DefaultTransport)},
		DstOptions: []remote.Option{remote.WithTransport(http.DefaultTransport)},
		Log:        slog.Default(),
	}

	syncErr := s.Sync(context.Background())
	if syncErr != nil {
		t.Fatalf("Sync returned unexpected error: %v (expected nil — failures are recorded via Failed())", syncErr)
	}

	failed := s.Failed()
	if failed == 0 {
		t.Fatal("expected Failed() > 0 after a push error, got 0 — the bug is not fixed")
	}
	t.Logf("Sync returned nil (expected), Failed() = %d (expected > 0)", failed)
}
