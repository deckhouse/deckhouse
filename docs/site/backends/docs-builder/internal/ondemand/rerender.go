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

package ondemand

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/metrics"
)

// rerenderQuietPasses is how many passes in a row finding no archive of an
// older template stop Rerender: every replica runs the new template by then,
// so none uploads such archives any more. The next leader starts it again.
// ponytail: a replica of the old build still alive after the stop (a rollout
// slower than these passes) can upload an old archive that stays until the
// next leader; wake Rerender when a pod serves one if that shows up.
const rerenderQuietPasses = 3

// rerenderIdlePoll is how often a pass waiting for the renders of readers and
// pinned versions to finish looks again.
const rerenderIdlePoll = time.Second

// errCannotRerender marks a version the template revision can not render:
// its image is gone, Hugo fails on it or, for an archive with the AI export,
// the export does. Its archive of an older revision is kept and not tried
// again with this revision.
var errCannotRerender = errors.New("the template can not render the version")

// rerenderFailedKey keeps the archives the revision could not render, so a
// new leader does not try them again.
func rerenderFailedKey(revision string) string { return "rerender/" + revision + ".json" }

type rerenderFailed struct {
	Archives []string `json:"archives"`
}

// rerenderer renders anew, on the leader, the archives of the cold cache
// that another template revision rendered, in the render slots of the store.
type rerenderer struct {
	cold *coldArchives
	// slots are the render slots of the store; busy reports a render of a
	// reader or of a pinned version queued or running; replace publishes a
	// version rendered anew in place of the one the store serves from an
	// archive of another revision.
	slots   chan struct{}
	busy    func() bool
	replace func(module, version, outDir string, withAI bool)
	// idlePoll is rerenderIdlePoll, shorter in the tests.
	idlePoll time.Duration
	// render renders a version (Config.Render); timeout bounds the render
	// of an archive with its upload (Config.RenderTimeout).
	render  RenderFunc
	timeout time.Duration
	dir     string
	logger  *log.Logger
	// gauge sets a gauge without labels.
	gauge func(name string, v float64)
}

// run renders anew, one at a time, the archives of the cold cache that
// another template revision rendered, while no render of a reader or of a
// pinned version is running: those go first. Archives of pinned versions go
// before those of readers, the most recently needed first. A version
// rendered anew replaces its archive, and the published version when this pod
// serves the old one. A version the revision can not render keeps its
// archive. It runs on the leader until ctx is done or rerenderQuietPasses
// passes in a row find nothing to render; every is the pause between passes
// over the bucket.
func (r *rerenderer) run(ctx context.Context, every time.Duration) {
	for quiet := 0; quiet < rerenderQuietPasses; {
		if revision := r.cold.revision(); revision != "" {
			found, err := r.pass(ctx, revision)
			if err == nil && found == 0 {
				quiet++
			} else {
				quiet = 0
				if err != nil && ctx.Err() == nil && !coldMissing(err) {
					r.logger.Warn("render archives of older templates", log.Err(err))
				}
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
	r.logger.Info("no archives of older templates left, rendering them anew stops until the next leader")
}

// pass renders anew the archives of older templates and returns how many it
// found.
func (r *rerenderer) pass(ctx context.Context, revision string) (int, error) {
	objects, err := r.cold.List(ctx, "")
	if err != nil {
		return 0, err
	}
	failed, err := r.loadFailed(ctx, revision)
	if err != nil {
		return 0, err
	}

	current := make(map[string]bool)
	for _, o := range objects {
		if a, ok := parseColdKey(o.Key); ok && a.revision == revision {
			current[coldVersionPrefix(a.module, a.version)+coldSuffix(a.withAI)] = true
		}
	}
	// The archive of a pinned version covers the one of a reader's version.
	covered := func(a coldArchive) bool {
		prefix := coldVersionPrefix(a.module, a.version)
		return current[prefix+coldAISuffix] || !a.withAI && current[prefix+coldArchiveSuffix]
	}

	var stale []ColdObject
	for _, o := range objects {
		if a, ok := parseColdKey(o.Key); ok && a.revision != revision && !covered(a) && !slices.Contains(failed.Archives, o.Key) {
			stale = append(stale, o)
		}
	}
	slices.SortFunc(stale, func(a, b ColdObject) int {
		aAI, bAI := isAIKey(a.Key), isAIKey(b.Key)
		if aAI != bAI {
			if aAI {
				return -1
			}
			return 1
		}
		return b.LastModified.Compare(a.LastModified)
	})
	if len(stale) > 0 {
		r.logger.Info("rendering archives of older templates", slog.Int("archives", len(stale)))
	}
	left := len(stale)
	r.gauge(metrics.DocsBuilderOnDemandStaleArchives, float64(left))
	settled := func() {
		left--
		r.gauge(metrics.DocsBuilderOnDemandStaleArchives, float64(left))
	}
	r.gauge(metrics.DocsBuilderOnDemandRerenderFailed, float64(len(failed.Archives)))

	for _, o := range stale {
		a, _ := parseColdKey(o.Key)
		if covered(a) {
			settled()
			continue
		}
		// A ModulePullOverride pulls this tag now: its render would be the
		// branch, which is not kept in S3, so the old archive goes too. When
		// the cluster does not answer, the next pass asks again.
		if r.cold.branch != nil {
			branch, err := r.cold.branch(ctx, a.module, a.version)
			if err != nil {
				return len(stale), fmt.Errorf("check for a module pull override: %w", err)
			}
			if branch {
				if err := r.cold.Delete(ctx, o.Key); err != nil {
					r.logger.Warn("delete the archive of a tag a module pull override pulls", slog.String("key", o.Key), log.Err(err))
				}
				settled()
				continue
			}
		}
		if err := r.waitIdle(ctx); err != nil {
			return len(stale), err
		}

		withAI, err := r.renderAnew(ctx, revision, a, o.Key)
		switch {
		case err == nil:
			current[coldVersionPrefix(a.module, a.version)+coldSuffix(withAI)] = true
			settled()
		case ctx.Err() != nil:
			return len(stale), ctx.Err()
		case errors.Is(err, errCannotRerender):
			r.logger.Warn("the template can not render the version: its archive of an older template stays",
				slog.String("module", a.module), slog.String("version", a.version), log.Err(err))
			failed.Archives = append(failed.Archives, o.Key)
			settled()
			r.gauge(metrics.DocsBuilderOnDemandRerenderFailed, float64(len(failed.Archives)))
			if err := r.saveFailed(ctx, revision, failed); err != nil {
				return len(stale), err
			}
		default:
			// The registry or S3 failed: the next pass tries again.
			r.logger.Warn("render an archive of an older template", slog.String("key", o.Key), log.Err(err))
		}
	}

	return len(stale), nil
}

func isAIKey(key string) bool {
	a, _ := parseColdKey(key)
	return a.withAI
}

// waitIdle returns once no render of a reader or of a pinned version is
// queued or running.
func (r *rerenderer) waitIdle(ctx context.Context) error {
	for r.busy() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(r.idlePoll):
		}
	}

	return nil
}

// busy reports whether a render of a reader or of a pinned version is queued
// or running.
func (s *Store) busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range s.renders {
		if r.status.Stage != StageFailed {
			return true
		}
	}

	return false
}

// renderAnew renders the version of an archive of another revision in one
// render slot, uploads it under this revision and removes the old archive.
// The version this pod serves from the old archive is replaced. It returns
// whether the new archive has the AI export.
func (r *rerenderer) renderAnew(ctx context.Context, revision string, a coldArchive, oldKey string) (bool, error) {
	select {
	case r.slots <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() { <-r.slots }()

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	tmp := filepath.Join(r.dir, tmpDir)
	if err := os.MkdirAll(tmp, 0700); err != nil {
		return false, fmt.Errorf("mkdir tmp: %w", err)
	}
	workDir, err := os.MkdirTemp(tmp, a.module+"-"+a.version+"-")
	if err != nil {
		return false, fmt.Errorf("mkdir work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	outDir := filepath.Join(workDir, "out")
	if err := r.render(ctx, a.module, a.version, workDir, outDir, a.withAI, func(Stage) {}); err != nil {
		// Only what the new template can not do: a failure of the registry,
		// of S3 or of the pod is tried again on the next pass.
		if ctx.Err() == nil && lasting(err) {
			return false, fmt.Errorf("%w: %w", errCannotRerender, err)
		}
		return false, err
	}
	withAI := hasAIExport(outDir)
	// An archive with the AI export is not replaced by one without it: the
	// followers take a target from S3 only and would serve it without one.
	if a.withAI && !withAI {
		return false, fmt.Errorf("%w: %w", errCannotRerender, errNoAIExport)
	}

	archive := r.cold.pack(outDir)
	if archive == "" {
		return false, errors.New("pack the rendered version")
	}
	defer os.Remove(archive)

	// An override may have appeared while it waited and rendered; the next
	// pass removes the old archive then.
	if r.cold.isBranch(ctx, a.module, a.version) {
		return false, errors.New("a module pull override pulls the tag now")
	}

	r.replace(a.module, a.version, outDir, withAI)

	if err := r.cold.Put(ctx, coldKey(revision, a.module, a.version, withAI), archive); err != nil {
		return false, fmt.Errorf("put to S3: %w", err)
	}
	if err := r.cold.Delete(ctx, oldKey); err != nil {
		r.logger.Warn("delete an archive of an older template", slog.String("key", oldKey), log.Err(err))
	}
	r.logger.Info("rendered an archive of an older template anew", slog.String("module", a.module), slog.String("version", a.version))

	return withAI, nil
}

// replaceStale publishes the version rendered in outDir when this pod serves
// it from an archive of another revision.
func (s *Store) replaceStale(module, version, outDir string, withAI bool) {
	s.mu.Lock()
	v, ok := s.versions[key(module, version)]
	stale := ok && v.stale
	s.mu.Unlock()
	if !stale {
		return
	}

	size, err := dirSize(outDir)
	if err == nil {
		err = os.Chmod(outDir, 0755)
	}
	var old string
	if err == nil {
		old, err = s.publish(module, version, outDir, &published{size: size, ai: withAI})
	}
	if old != "" {
		os.RemoveAll(old)
	}
	if err != nil {
		s.cfg.Logger.Warn("publish the version rendered anew", slog.String("module", module), slog.String("version", version), log.Err(err))
	}
}

// loadFailed reads the archives the revision could not render. S3 not
// answering is an error: the list is still there. A list that does not
// parse is reported and taken for an empty one: the pass writes it anew, at
// the price of rendering its archives once more. Keeping it would stop the
// passes of the revision until someone deletes the object by hand.
func (r *rerenderer) loadFailed(ctx context.Context, revision string) (rerenderFailed, error) {
	var failed rerenderFailed
	raw, err := r.cold.getList(ctx, rerenderFailedKey(revision))
	if err != nil {
		if coldMissing(err) {
			return failed, nil
		}
		return failed, fmt.Errorf("get the failed renders from S3: %w", err)
	}
	if err := json.Unmarshal(raw, &failed); err != nil {
		r.logger.Error("the list of failed renders in S3 can not be read, it is made anew", slog.String("revision", revision), log.Err(err))
		return rerenderFailed{}, nil
	}

	return failed, nil
}

func (r *rerenderer) saveFailed(ctx context.Context, revision string, failed rerenderFailed) error {
	path, err := writeTemp(filepath.Join(r.dir, tmpDir, "rerender.json"), func(w io.Writer) error { return json.NewEncoder(w).Encode(failed) })
	if err != nil {
		return err
	}
	defer os.Remove(path)

	if err := r.cold.Put(ctx, rerenderFailedKey(revision), path); err != nil {
		return fmt.Errorf("put the failed renders to S3: %w", err)
	}

	return nil
}

// Rerender renders anew the archives of the cold cache that another template
// revision rendered (rerenderer).
func (s *Store) Rerender(ctx context.Context, every time.Duration) {
	s.rerender.run(ctx, every)
}
