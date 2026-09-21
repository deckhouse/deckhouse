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

package gc

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startRegistry runs a real registry in memory, so that deletion is exercised against actual
// registry behaviour rather than a mock of it — including the parts that refuse.
//
// It comes with a data directory, because the collector no longer asks the registry what it holds:
// a pull-through cache answers that with its upstream's contents, so what may be deleted is read
// from the store's own files. A registry without a matching directory therefore holds, as far as the
// collector is concerned, nothing at all — which is why `push` writes to both.
// store is a registry together with the directory its files would live in — the test's own pairing,
// not the product's: the collector takes the two separately.
type store struct {
	Registry
	dataDir string
}

func startRegistry(t *testing.T) store {
	t.Helper()

	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)

	return store{
		Registry: Registry{Address: parsed.Host, Insecure: true, Scope: "system/deckhouse"},
		dataDir:  t.TempDir(),
	}
}

func push(t *testing.T, target store, reference string) {
	t.Helper()

	image, err := random.Image(64, 1)
	require.NoError(t, err)

	tag, err := name.NewTag(target.Address+"/"+reference, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))

	// And the tag link distribution would have written, which is what the collector reads.
	repository, tagged, found := strings.Cut(reference, ":")
	require.True(t, found, "a pushed reference needs a tag for the collector to see it")
	dir := filepath.Join(target.dataDir, "docker", "registry", "v2", "repositories",
		filepath.FromSlash(repository), "_manifests", "tags", tagged, "current")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "link"), []byte("sha256:x"), 0o644))
}

func exists(t *testing.T, target store, reference string) bool {
	t.Helper()

	tag, err := name.NewTag(target.Address+"/"+reference, name.Insecure)
	require.NoError(t, err)

	_, err = remote.Get(tag)
	return err == nil
}

// countingSweeper stands in for the registry binary, which cannot run against an in-memory
// registry: it has no filesystem store to walk.
type countingSweeper struct {
	runs int
	err  error
}

func (s *countingSweeper) Sweep(context.Context) error {
	s.runs++
	return s.err
}

func newCollector(t *testing.T, target store, releases Releases, sweeper Sweeper) *Collector {
	t.Helper()

	return &Collector{
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Registry: target.Registry,
		DataDir:  target.dataDir,
		Releases: releases,
		Sweep:    sweeper,
	}
}

func TestRunReclaimsSupersededReleases(t *testing.T) {
	target := startRegistry(t)
	push(t, target, "system/deckhouse:v1.76.6")
	push(t, target, "system/deckhouse:v1.75.0")
	push(t, target, "system/deckhouse:v1.74.3")
	push(t, target, "system/deckhouse:v1.70.0")
	push(t, target, "system/deckhouse/release-channel:stable")

	sweeper := &countingSweeper{}
	collector := newCollector(t, target, Releases{Deployed: "v1.76.6", Previous: "v1.75.0"}, sweeper)

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 5, report.Considered)
	assert.Len(t, report.Deleted, 2)
	assert.Empty(t, report.Failed)
	assert.True(t, report.Swept)
	assert.Equal(t, 1, sweeper.runs)

	// What the cluster runs, and what it would roll back to.
	assert.True(t, exists(t, target, "system/deckhouse:v1.76.6"))
	assert.True(t, exists(t, target, "system/deckhouse:v1.75.0"))
	// What it has moved past.
	assert.False(t, exists(t, target, "system/deckhouse:v1.74.3"))
	assert.False(t, exists(t, target, "system/deckhouse:v1.70.0"))
	// A release channel tag is not a version, so what it points at is unknown.
	assert.True(t, exists(t, target, "system/deckhouse/release-channel:stable"))
}

// TestRunRefusesWithoutADeployedVersion is the safety property: every deletion is justified
// by a comparison against the deployed release, so without one there is no justification for
// any of them — and the run must do nothing rather than its best.
func TestRunRefusesWithoutADeployedVersion(t *testing.T) {
	target := startRegistry(t)
	push(t, target, "system/deckhouse:v1.76.6")
	push(t, target, "system/deckhouse:v1.74.3")

	sweeper := &countingSweeper{}
	collector := newCollector(t, target, Releases{}, sweeper)

	_, err := collector.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot decide what to keep")

	assert.True(t, exists(t, target, "system/deckhouse:v1.76.6"))
	assert.True(t, exists(t, target, "system/deckhouse:v1.74.3"))
	assert.Zero(t, sweeper.runs, "blobs were reclaimed after a run that decided nothing")
}

// TestRunLeavesOtherRepositoriesAlone: a store may hold more than this module put there, and
// these rules are about Deckhouse releases. Applying them elsewhere would delete by a measure
// that has nothing to do with those images.
func TestRunLeavesOtherRepositoriesAlone(t *testing.T) {
	target := startRegistry(t)
	push(t, target, "system/deckhouse:v1.74.3")
	push(t, target, "tenant/app:v1.0.0")
	push(t, target, "system/deckhouse-extra/thing:v0.1.0")

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, report.Considered, "a repository outside the scope was judged")
	assert.False(t, exists(t, target, "system/deckhouse:v1.74.3"))
	assert.True(t, exists(t, target, "tenant/app:v1.0.0"))
	// A prefix that merely starts with the same letters is a different repository.
	assert.True(t, exists(t, target, "system/deckhouse-extra/thing:v0.1.0"))
}

func TestRunDryRunTouchesNothing(t *testing.T) {
	target := startRegistry(t)
	push(t, target, "system/deckhouse:v1.74.3")

	sweeper := &countingSweeper{}
	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, sweeper)
	collector.DryRun = true

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.Len(t, report.Deleted, 1, "a dry run must still say what it would do")
	assert.False(t, report.Swept)
	assert.Zero(t, sweeper.runs)
	assert.True(t, exists(t, target, "system/deckhouse:v1.74.3"))
}

// TestRunWithNothingToReclaim keeps a healthy cluster from paying for the sweep every night:
// distribution's collector walks the whole store, which on a full cache is not free.
func TestRunWithNothingToReclaim(t *testing.T) {
	target := startRegistry(t)
	push(t, target, "system/deckhouse:v1.76.6")

	sweeper := &countingSweeper{}
	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, sweeper)

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.Empty(t, report.Deleted)
	assert.Zero(t, sweeper.runs, "the store was walked for nothing")
	assert.Equal(t, 1, report.Kept[ReasonCurrentRelease])
}

// TestRunReportsAFailedSweep: the manifests are gone either way, so the run did something —
// but a disk that never shrinks has to be attributable to the sweep rather than to the
// judgement.
func TestRunReportsAFailedSweep(t *testing.T) {
	target := startRegistry(t)
	push(t, target, "system/deckhouse:v1.74.3")

	sweeper := &countingSweeper{err: errors.New("the store is not writable")}
	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, sweeper)

	report, err := collector.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reclaiming the blobs")
	assert.Len(t, report.Deleted, 1)
	assert.False(t, report.Swept)
}

// TestRunCountsWhyThingsWereKept is the number an operator needs when the disk is still full
// after a run: "nothing was reclaimable" and "the rules did not recognise anything" look the
// same from the outside otherwise.
func TestRunCountsWhyThingsWereKept(t *testing.T) {
	target := startRegistry(t)
	push(t, target, "system/deckhouse:v1.76.6")
	push(t, target, "system/deckhouse:v1.77.0")
	push(t, target, "system/deckhouse:stable")
	push(t, target, "system/deckhouse:my-build")

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, report.Kept[ReasonCurrentRelease])
	assert.Equal(t, 1, report.Kept[ReasonNewerRelease])
	assert.Equal(t, 2, report.Kept[ReasonNotAVersion])
	assert.Empty(t, report.Deleted)
}

func TestInScope(t *testing.T) {
	collector := &Collector{Registry: Registry{Scope: "system/deckhouse"}}

	assert.True(t, collector.inScope("system/deckhouse"))
	assert.True(t, collector.inScope("system/deckhouse/install"))
	assert.True(t, collector.inScope("/system/deckhouse/"))
	assert.False(t, collector.inScope("system/deckhouse-extra"))
	assert.False(t, collector.inScope("tenant/app"))
	assert.False(t, collector.inScope("system"))

	// No scope means the whole registry, which is what a store dedicated to this module is.
	assert.True(t, (&Collector{}).inScope("anything/at/all"))
}

func TestBinarySweepWithoutABinary(t *testing.T) {
	sweeper := &Binary{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	err := sweeper.Sweep(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no registry binary")
}

// TestRunKeepsTagsOutsideThePlatformVersionSpace is the pass judging things it cannot judge.
//
// The rule of the tag pass is "older than the release the cluster runs", and it is meaningful for
// exactly one set of tags: the platform's own releases, which live on the scope root. Everything
// beneath it is versioned by somebody else — a module package by the module, a node package by the
// software it carries — and comparing those numbers with the platform's is comparing nothing.
//
// Measured before this guard existed: with the platform at v1.76.6, a module package at v0.6.10 and
// a node package at v1.7.28 were both read as superseded and deleted. The manifest pass then went
// for the same tags, got 404, and every later run failed on it. In an air-gapped cluster a module
// package cannot be fetched again — the next `d8 mirror push` is the only way back.
func TestRunKeepsTagsOutsideThePlatformVersionSpace(t *testing.T) {
	target := startRegistry(t)

	push(t, target, "system/deckhouse:v1.76.6")
	push(t, target, "system/deckhouse:v1.75.0")
	// Numerically older than the platform, and entirely unrelated to it.
	push(t, target, "system/deckhouse/modules/sds-node-configurator:v0.6.10")
	push(t, target, "system/deckhouse/packages/containerd/version:v1.7.28")

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6", Previous: "v1.75.0"}, &countingSweeper{})

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.True(t, exists(t, target, "system/deckhouse/modules/sds-node-configurator:v0.6.10"),
		"a module package is versioned by the module, and the platform's version says nothing about it")
	assert.True(t, exists(t, target, "system/deckhouse/packages/containerd/version:v1.7.28"),
		"a node package is versioned by the software it carries")
	assert.Empty(t, report.Deleted, "the two platform releases are both kept, and nothing else may be judged")
}

// startRefusingRegistry is the registry answering a deletion the way the serving listener does.
//
// The serving listener is a pull-through cache, and the proxy store in front of it implements no
// deletion at all: every DELETE comes back 405 Unsupported. Addressed there, a collection reads the
// store, decides correctly, is refused on every single reference, and reports the refusals as
// warnings — a run that looks like it ran.
func startRefusingRegistry(t *testing.T) store {
	t.Helper()

	real := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		real.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)

	return store{
		Registry: Registry{Address: parsed.Host, Insecure: true, Scope: "system/deckhouse"},
		dataDir:  t.TempDir(),
	}
}

// TestRunFailsWhenEveryDeletionIsRefused is the run that did nothing and said so quietly.
//
// One refusal is a warning: a reference that would not go is not a reason to abandon the rest. But a
// run in which everything was refused and nothing was deleted is not a partial success, it is a run
// against a registry that does not accept deletions — the wrong address — and it has to be reported
// as a failure. Silently, this is a store that grows forever while the collection reports "kept" and
// the disk fills.
func TestRunFailsWhenEveryDeletionIsRefused(t *testing.T) {
	target := startRefusingRegistry(t)

	push(t, target, "system/deckhouse:v1.76.6")
	push(t, target, "system/deckhouse:v1.60.0")

	sweeper := &countingSweeper{}
	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, sweeper)

	report, err := collector.Run(context.Background())

	require.Error(t, err, "a run refused on every reference is a failed run, not a quiet one")
	assert.Empty(t, report.Deleted)
	assert.NotEmpty(t, report.Failed)
	assert.Zero(t, sweeper.runs, "there is nothing to sweep when nothing was deleted")
}
