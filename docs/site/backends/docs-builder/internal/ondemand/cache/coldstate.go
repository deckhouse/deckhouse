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

package cache

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/ondemand"
)

// restoreAll publishes from the cold cache the targets, with the AI export,
// and the versions of their channels, and links the channels. It returns
// how many of them are not served and whether they are served from this
// template revision too (fresh), the targets with the AI export: a version
// missing, served from an archive of an older revision, or a target served
// without the export, which may have failed on the leader, is looked for
// again on the next call. A development branch is never in the cold cache
// and counts for neither.
// ponytail: a release version restored here keeps being served when a
// ModulePullOverride later pulls the same tag (a tag shaped vX.Y.Z), until
// the pod restarts; unpublish it here if such overrides become common.
func (s *Store) restoreAll(ctx context.Context, targets []ondemand.Target, chans map[string]ondemand.Channels, logger *log.Logger) (int, bool) {
	chanTargets := channelTargets(ondemand.ByModule(targets), chans)
	s.Pin(targets, chanTargets)

	missing, fresh := 0, true
	restore := func(t ondemand.Target, withAI bool) {
		ok, err := s.restorePinned(ctx, t, withAI)
		if err != nil && ctx.Err() == nil {
			logger.Warn("restore a version from S3", slog.String("module", t.Module), slog.String("version", t.Version), log.Err(err))
		}
		if t.Rev != "" {
			return
		}
		if !ok {
			missing++
		}
		fresh = fresh && s.publishedAsWanted(t, withAI)
	}
	for _, t := range targets {
		restore(t, true)
	}
	for _, t := range chanTargets {
		restore(t, false)
	}
	if ctx.Err() != nil {
		return missing, false
	}

	s.SetChannels(chans)

	return missing, fresh
}

// coldGetList reads a list kept in the cold cache (coldArchives).
func (s *Store) coldGetList(ctx context.Context, key string) ([]byte, error) {
	return s.cold.getList(ctx, key)
}

// uploadMissing uploads the archives of the pinned release versions this pod
// serves and the cold cache lacks, from the published pages. The followers
// take the versions targets.json names from the cold cache only: without it
// an upload that failed, a version taken for a branch while the cluster did
// not answer, or an archive evicted would stay missing on them for good. An
// archive of this template revision is enough, with the AI export when the
// version is served with it. A version served from an archive of an older
// revision is left to Rerender, which renders such archives anew; once the
// cold cache has none of an older revision of it, the version is marked lost
// and the pass renders it anew (renderPinned).
func (s *Store) uploadMissing(ctx context.Context, pinned []ondemand.Target) error {
	revision := s.cold.revision()
	if revision == "" {
		return nil
	}
	objects, err := s.cold.List(ctx, "")
	if err != nil {
		return err
	}
	inCold := make(map[string]bool, len(objects))
	// older are the versions with an archive of another revision.
	older := make(map[string]bool)
	for _, o := range objects {
		inCold[o.Key] = true
		if a, ok := parseColdKey(o.Key); ok && a.revision != revision {
			older[key(a.module, a.version)] = true
		}
	}
	if err := os.MkdirAll(filepath.Join(s.cfg.Dir, tmpDir), 0700); err != nil {
		return fmt.Errorf("mkdir tmp: %w", err)
	}

	for _, t := range pinned {
		s.mu.Lock()
		v, ok := s.versions[key(t.Module, t.Version)]
		served := ok && !v.stale && v.rev == "" && t.Rev == ""
		withAI := ok && v.ai
		lost := ok && v.stale && !older[key(t.Module, t.Version)]
		marked := lost && !v.lost
		if ok {
			v.lost = lost
		}
		s.mu.Unlock()
		if marked {
			s.cfg.Logger.Info("a version served from an archive of an older template has none left in S3: it is rendered anew", slog.String("module", t.Module), slog.String("version", t.Version))
		}
		k := coldKey(revision, t.Module, t.Version, withAI)
		// The archive with the AI export holds the one without.
		if !served || inCold[k] || inCold[coldKey(revision, t.Module, t.Version, true)] || s.cold.isBranch(ctx, t.Module, t.Version) {
			continue
		}

		snapshot := s.cold.snapshot(filepath.Join(s.cfg.Dir, t.Module, t.Version))
		if snapshot == "" {
			continue
		}
		drop := ""
		if withAI {
			drop = coldKey(revision, t.Module, t.Version, false)
		}
		s.cfg.Logger.Info("upload a version missing from S3", slog.String("module", t.Module), slog.String("version", t.Version))
		s.cold.upload(k, drop, snapshot)
	}

	return nil
}

// EvictCold keeps the archives of the cold cache below high of its quota
// (coldArchives).
func (s *Store) EvictCold(ctx context.Context, quota int64, high, low float64) error {
	return s.cold.evict(ctx, quota, high, low)
}

// busy reports whether a render of a reader or of a pinned version is queued
// or running.
func (s *Store) busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range s.renders {
		if r.status.Stage != ondemand.StageFailed {
			return true
		}
	}

	return false
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

// Rerender renders anew the archives of the cold cache that another template
// revision rendered (rerenderer).
func (s *Store) Rerender(ctx context.Context, every time.Duration) {
	s.rerender.run(ctx, every)
}
