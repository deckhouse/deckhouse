// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

func TestCLIRegistryRepository(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		// Edition repositories collapse to the shared root.
		"registry.deckhouse.io/deckhouse/ee":            "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/ce":            "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/be":            "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/se":            "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/se-plus":       "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/fe/":           "registry.deckhouse.io/deckhouse",
		"registry.company.com:5000/mirror/deckhouse/ee": "registry.company.com:5000/mirror/deckhouse",
		// Repositories without an edition segment are the root already.
		"dev-registry.deckhouse.io/sys/deckhouse-oss": "dev-registry.deckhouse.io/sys/deckhouse-oss",
		"registry.company.com/deckhouse":              "registry.company.com/deckhouse",
		"registry.company.com/ee-mirror":              "registry.company.com/ee-mirror",
		"registry.company.com/deckhouse/EE":           "registry.company.com/deckhouse/EE",
		"registry.local:5000":                         "registry.local:5000",
		// CSE keeps its editionless artifacts (the installer) under
		// deckhouse/cse, so that path is a root of its own.
		"registry-cse.deckhouse.ru/deckhouse/cse": "registry-cse.deckhouse.ru/deckhouse/cse",
		// Stripping never eats the last path segment and leaves a bare host,
		// nor empties the repository outright.
		"registry.example.com/ee": "registry.example.com/ee",
		"registry.local/se":       "registry.local/se",
		"/ee":                     "/ee",
		"//ee":                    "//ee",
	}

	for in, want := range cases {
		in, want := in, want
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, cliRegistryRepository(in))
		})
	}
}

func TestCLIRepoCandidates(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		// Edition repositories yield two candidates: as-is, then the root.
		"registry.deckhouse.io/deckhouse/ee": {"registry.deckhouse.io/deckhouse/ee", "registry.deckhouse.io/deckhouse"},
		"registry.local/dest/se-plus/":       {"registry.local/dest/se-plus", "registry.local/dest"},
		// No edition suffix: the single candidate is the repository itself.
		"dev-registry.deckhouse.io/sys/deckhouse-oss": {"dev-registry.deckhouse.io/sys/deckhouse-oss"},
		"registry-cse.deckhouse.ru/deckhouse/cse":     {"registry-cse.deckhouse.ru/deckhouse/cse"},
		// Trimming refuses to leave a bare host, so one candidate again.
		"registry.example.com/ee": {"registry.example.com/ee"},
	}

	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, cliRepoCandidates(in))
		})
	}
}

func TestCLIRepoMemo(t *testing.T) {
	t.Parallel()

	var memo cliRepoMemo

	_, ok := memo.get("repo-a")
	assert.False(t, ok, "empty memo must not return a root")

	assert.True(t, memo.set("repo-a", "root-1"), "first set is a change")
	root, ok := memo.get("repo-a")
	require.True(t, ok)
	assert.Equal(t, "root-1", root)

	assert.False(t, memo.set("repo-a", "root-1"), "same value again is not a change")

	_, ok = memo.get("repo-b")
	assert.False(t, ok, "another cluster repository must not match")

	assert.True(t, memo.set("repo-b", "root-2"), "new repository displaces the old entry")
	_, ok = memo.get("repo-a")
	assert.False(t, ok)
}

// cliRepoProbeOp is a requestCLIArtifacts op stub: per-root results and
// errors, plus a record of the roots tried, in order. A root present in
// neither map answers ErrPackageNotFound.
type cliRepoProbeOp struct {
	results map[string]string
	errs    map[string]error
	tried   []string
}

func (o *cliRepoProbeOp) run(cfg *registry.ClientConfig) (string, error) {
	o.tried = append(o.tried, cfg.Repository)
	if err, ok := o.errs[cfg.Repository]; ok {
		return "", err
	}
	if res, ok := o.results[cfg.Repository]; ok {
		return res, nil
	}
	return "", registry.ErrPackageNotFound
}

func TestRequestCLIArtifacts_MirroredLayoutServedAsIs(t *testing.T) {
	t.Parallel()

	op := &cliRepoProbeOp{results: map[string]string{"registry.local/dest/ee": "tags"}}
	var memo cliRepoMemo
	base := &registry.ClientConfig{Repository: "registry.local/dest/ee", Scheme: "https"}

	res, cfg, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.NoError(t, err)
	assert.Equal(t, "tags", res)
	assert.Equal(t, "registry.local/dest/ee", cfg.Repository)
	assert.Equal(t, "https", cfg.Scheme, "the rest of the config travels unchanged")
	assert.Equal(t, []string{"registry.local/dest/ee"}, op.tried, "the trimmed root must not be consulted")
}

func TestRequestCLIArtifacts_OfficialLayoutFallsToTrimmedRoot(t *testing.T) {
	t.Parallel()

	op := &cliRepoProbeOp{results: map[string]string{"registry.deckhouse.io/deckhouse": "tags"}}
	var memo cliRepoMemo
	base := &registry.ClientConfig{Repository: "registry.deckhouse.io/deckhouse/ee"}

	res, cfg, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.NoError(t, err)
	assert.Equal(t, "tags", res)
	assert.Equal(t, "registry.deckhouse.io/deckhouse", cfg.Repository)
	assert.Equal(t, []string{"registry.deckhouse.io/deckhouse/ee", "registry.deckhouse.io/deckhouse"}, op.tried)

	// The next request goes straight to the remembered root.
	op.tried = nil
	_, _, err = requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.NoError(t, err)
	assert.Equal(t, []string{"registry.deckhouse.io/deckhouse"}, op.tried)
}

func TestRequestCLIArtifacts_BothRootsPresentAsIsWins(t *testing.T) {
	t.Parallel()

	op := &cliRepoProbeOp{results: map[string]string{
		"registry.local/dest/ee": "mirrored",
		"registry.local/dest":    "foreign",
	}}
	var memo cliRepoMemo
	base := &registry.ClientConfig{Repository: "registry.local/dest/ee"}

	res, cfg, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.NoError(t, err)
	assert.Equal(t, "mirrored", res)
	assert.Equal(t, "registry.local/dest/ee", cfg.Repository)
	assert.Equal(t, []string{"registry.local/dest/ee"}, op.tried)
}

func TestRequestCLIArtifacts_AllNotFound(t *testing.T) {
	t.Parallel()

	op := &cliRepoProbeOp{}
	var memo cliRepoMemo
	base := &registry.ClientConfig{Repository: "registry.deckhouse.io/deckhouse/ee"}

	_, _, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.ErrorIs(t, err, registry.ErrPackageNotFound)
	assert.Equal(t, []string{"registry.deckhouse.io/deckhouse/ee", "registry.deckhouse.io/deckhouse"}, op.tried)

	_, ok := memo.get("registry.deckhouse.io/deckhouse/ee")
	assert.False(t, ok, "failures must not poison the memo")
}

func TestRequestCLIArtifacts_NonNotFoundErrorFallsThrough(t *testing.T) {
	t.Parallel()

	errDenied := errors.New("access denied")
	op := &cliRepoProbeOp{
		errs:    map[string]error{"registry.deckhouse.io/deckhouse/ee": errDenied},
		results: map[string]string{"registry.deckhouse.io/deckhouse": "tags"},
	}
	var memo cliRepoMemo
	base := &registry.ClientConfig{Repository: "registry.deckhouse.io/deckhouse/ee"}

	res, cfg, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.NoError(t, err)
	assert.Equal(t, "tags", res)
	assert.Equal(t, "registry.deckhouse.io/deckhouse", cfg.Repository)
}

func TestRequestCLIArtifacts_FirstNonNotFoundErrorSurfaces(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	base := &registry.ClientConfig{Repository: "registry.deckhouse.io/deckhouse/ee"}

	// The as-is candidate fails hard, the trimmed root answers 404.
	op := &cliRepoProbeOp{errs: map[string]error{"registry.deckhouse.io/deckhouse/ee": errBoom}}
	var memo cliRepoMemo
	_, _, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.ErrorIs(t, err, errBoom)

	// The as-is candidate answers 404, the trimmed root fails hard.
	op = &cliRepoProbeOp{errs: map[string]error{"registry.deckhouse.io/deckhouse": errBoom}}
	memo = cliRepoMemo{}
	_, _, err = requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.ErrorIs(t, err, errBoom)
}

func TestRequestCLIArtifacts_RepoChangeInvalidatesMemo(t *testing.T) {
	t.Parallel()

	op := &cliRepoProbeOp{results: map[string]string{
		"registry.deckhouse.io/deckhouse": "official",
		"registry.local/dest/ee":          "mirrored",
	}}
	var memo cliRepoMemo

	// Remember the trimmed root for the official repository.
	_, _, err := requestCLIArtifacts(&memo, nopCLILogger{}, &registry.ClientConfig{Repository: "registry.deckhouse.io/deckhouse/ee"}, op.run)
	require.NoError(t, err)

	// A registry switch probes the new repository in canonical order.
	op.tried = nil
	res, _, err := requestCLIArtifacts(&memo, nopCLILogger{}, &registry.ClientConfig{Repository: "registry.local/dest/ee"}, op.run)
	require.NoError(t, err)
	assert.Equal(t, "mirrored", res)
	assert.Equal(t, []string{"registry.local/dest/ee"}, op.tried)
}

func TestRequestCLIArtifacts_ReprobesWhenRememberedRootGoesMissing(t *testing.T) {
	t.Parallel()

	base := &registry.ClientConfig{Repository: "registry.local/dest/ee"}
	var memo cliRepoMemo

	// The artifacts start at the trimmed root.
	op := &cliRepoProbeOp{results: map[string]string{"registry.local/dest": "old"}}
	_, _, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.NoError(t, err)

	// The user re-mirrors and the artifacts move under the cluster repo.
	op = &cliRepoProbeOp{results: map[string]string{"registry.local/dest/ee": "new"}}
	res, _, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.NoError(t, err)
	assert.Equal(t, "new", res)
	assert.Equal(t, []string{"registry.local/dest", "registry.local/dest/ee"}, op.tried, "the remembered root probes first, then the other candidate")

	// The memo follows the move.
	op.tried = nil
	_, _, err = requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.NoError(t, err)
	assert.Equal(t, []string{"registry.local/dest/ee"}, op.tried)
}

func TestRequestCLIArtifacts_SingleCandidateProbedOnce(t *testing.T) {
	t.Parallel()

	op := &cliRepoProbeOp{}
	var memo cliRepoMemo
	base := &registry.ClientConfig{Repository: "dev-registry.deckhouse.io/sys/deckhouse-oss"}

	_, _, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op.run)
	require.ErrorIs(t, err, registry.ErrPackageNotFound)
	assert.Equal(t, []string{"dev-registry.deckhouse.io/sys/deckhouse-oss"}, op.tried)
}

func TestRequestCLIArtifacts_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	var memo cliRepoMemo
	base := &registry.ClientConfig{Repository: "registry.deckhouse.io/deckhouse/ee"}
	op := func(cfg *registry.ClientConfig) (string, error) {
		if cfg.Repository == "registry.deckhouse.io/deckhouse" {
			return "ok", nil
		}
		return "", registry.ErrPackageNotFound
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, _, err := requestCLIArtifacts(&memo, nopCLILogger{}, base, op)
			assert.NoError(t, err)
		})
	}
	wg.Wait()
}
