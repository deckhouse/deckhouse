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

package syncer

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syncer/pkg/config"
)

func pushRandom(t *testing.T, ref string) {
	t.Helper()
	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	r, err := name.ParseReference(ref, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(r, img))
}

func tagsOf(t *testing.T, repo string) []string {
	t.Helper()
	r, err := name.NewRepository(repo, name.Insecure)
	require.NoError(t, err)
	tags, err := remote.List(r)
	require.NoError(t, err)
	return tags
}

func TestRun_PruneDeletesStaleTags(t *testing.T) {
	srcReg := httptest.NewServer(registry.New())
	defer srcReg.Close()
	dstReg := httptest.NewServer(registry.New())
	defer dstReg.Close()
	srcHost := srcReg.Listener.Addr().String()
	dstHost := dstReg.Listener.Addr().String()

	pushRandom(t, srcHost+"/app:a")
	pushRandom(t, srcHost+"/app:b")
	pushRandom(t, dstHost+"/app:a")
	pushRandom(t, dstHost+"/app:b")
	pushRandom(t, dstHost+"/app:stale")

	cfg := config.Config{
		Src:   config.Registry{Address: srcHost},
		Dest:  config.Registry{Address: dstHost},
		Prune: true,
	}
	s, err := New(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	require.NoError(t, err)
	require.NoError(t, s.Run(context.Background()))

	assert.ElementsMatch(t, []string{"a", "b"}, tagsOf(t, dstHost+"/app"))
}

func TestRun_PruneEmptySourceRetainsDest(t *testing.T) {
	// Source is empty (zero repos/tags). Dest has existing tags.
	// With prune enabled, a transient empty source must NOT wipe destination.
	srcReg := httptest.NewServer(registry.New())
	defer srcReg.Close()
	dstReg := httptest.NewServer(registry.New())
	defer dstReg.Close()
	srcHost := srcReg.Listener.Addr().String()
	dstHost := dstReg.Listener.Addr().String()

	// Only push to dst — src has zero tags.
	pushRandom(t, dstHost+"/app:v1")
	pushRandom(t, dstHost+"/app:v2")

	cfg := config.Config{
		Src:   config.Registry{Address: srcHost},
		Dest:  config.Registry{Address: dstHost},
		Prune: true,
	}
	s, err := New(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	require.NoError(t, err)
	require.NoError(t, s.Run(context.Background()))

	// Destination tags must be retained — empty source must not cause wipe.
	assert.ElementsMatch(t, []string{"v1", "v2"}, tagsOf(t, dstHost+"/app"))
}

func TestRun_NoPruneKeepsStale(t *testing.T) {
	srcReg := httptest.NewServer(registry.New())
	defer srcReg.Close()
	dstReg := httptest.NewServer(registry.New())
	defer dstReg.Close()
	srcHost := srcReg.Listener.Addr().String()
	dstHost := dstReg.Listener.Addr().String()

	pushRandom(t, srcHost+"/app:a")
	pushRandom(t, dstHost+"/app:a")
	pushRandom(t, dstHost+"/app:stale")

	cfg := config.Config{Src: config.Registry{Address: srcHost}, Dest: config.Registry{Address: dstHost}}
	s, err := New(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	require.NoError(t, err)
	require.NoError(t, s.Run(context.Background()))

	assert.Contains(t, tagsOf(t, dstHost+"/app"), "stale")
}
