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

package mirrorer

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

	"mirrorer/internal/syncer"
)

// TestRunOnce_NoSyncers guards the one-shot contract: RunOnce with no syncers
// configured returns nil (nothing to copy) and does not block (unlike Run,
// which loops). A zero-value mirrorer has a nil syncers slice — valid empty path.
func TestRunOnce_NoSyncers(t *testing.T) {
	m := &mirrorer{log: slog.Default()} // zero syncers
	if err := m.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce with no syncers: %v", err)
	}
}

// TestRunOnce_FailedPush verifies the core correctness guarantee: RunOnce must
// return a non-nil error when one or more tags fail to copy, even though Sync
// itself returns nil (per-tag errors are intentionally swallowed by the syncer
// for continuous Run mode). Without this check, the air-gap store-sync Job would
// emit a false "synced" signal on partial copies.
func TestRunOnce_FailedPush(t *testing.T) {
	// Source: real in-memory registry with one image.
	srcHandler := registry.New()
	srcServer := httptest.NewServer(srcHandler)
	defer srcServer.Close()

	srcReg, err := name.NewRegistry(srcServer.Listener.Addr().String(), name.Insecure)
	if err != nil {
		t.Fatalf("parse src registry: %v", err)
	}

	img, err := random.Image(512, 1)
	if err != nil {
		t.Fatalf("create random image: %v", err)
	}
	srcRef := srcReg.Repo("testimage").Tag("latest")
	if err = remote.Write(srcRef, img, remote.WithTransport(http.DefaultTransport)); err != nil {
		t.Fatalf("write image to src: %v", err)
	}

	// Destination: rejects all writes (PUT/POST → 500), returns 404 for GET.
	dstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut || r.Method == http.MethodPost {
			http.Error(w, "simulated failure", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer dstServer.Close()

	dstReg, err := name.NewRegistry(dstServer.Listener.Addr().String(), name.Insecure)
	if err != nil {
		t.Fatalf("parse dst registry: %v", err)
	}

	s := &syncer.Syncer{
		Src:        srcReg,
		Dst:        dstReg,
		SrcOptions: []remote.Option{remote.WithTransport(http.DefaultTransport)},
		DstOptions: []remote.Option{remote.WithTransport(http.DefaultTransport)},
		Log:        slog.Default(),
	}

	m := &mirrorer{
		log:     slog.Default(),
		syncers: []*syncer.Syncer{s},
	}

	if err := m.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce returned nil despite push failures — premature 'synced' signal would corrupt air-gap store")
	} else {
		t.Logf("RunOnce correctly returned error: %v", err)
	}
}
