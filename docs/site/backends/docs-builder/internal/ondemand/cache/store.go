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

// Package cache keeps the rendered versions of modules on the cache volume
// nginx serves: the Store renders a version on request, the Pinner keeps the
// target versions rendered and pinned, and a Follower keeps a pod that is not
// the leader in step with it.
package cache

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/ondemand/catalog"
	"github.com/flant/docs-builder/internal/ondemand/channels"
	"github.com/flant/docs-builder/internal/ondemand/index"
)

// indexPages are the names of the main page of a directory: Hugo renders the
// README of a module as readme.html, other sections as index.html.
var indexPages = []string{"index.html", "readme.html"}

// firstPinWait bounds WaitForPin, so a store whose pins never come (modules
// can not be listed) still evicts.
const firstPinWait = 30 * time.Minute

const (
	tmpDir   = ".tmp"
	trashDir = ".trash"
	// siteDir mirrors the URL layout for nginx with links to the versions:
	// .site/<lang>/modules/<module>/<version> -> <module>/<version>/<lang>.
	siteDir = ".site"
)

// Config of a Store. Sizes and durations are fixed in the module, chosen by
// measuring the documentation volume.
type Config struct {
	// Dir is the cache volume shared with nginx: a rendered version lives in
	// Dir/<module>/<version>/<lang>/, and nginx reads it through the links in
	// Dir/.site/<lang>/modules/<module>/<version>.
	Dir string
	// MaxSize is the total size of the rendered versions that are not pinned;
	// the least recently opened ones are evicted above it. Pinned versions
	// are kept on top of it: the target of every module and, on the public
	// site, the versions of its release channels, up to five per module.
	MaxSize int64
	// PinnedBudget is the room the pinned versions have on the cache volume:
	// its size less MaxSize. Zero when the size of the volume is not known.
	PinnedBudget int64
	// MaxIdle evicts a version that has not been opened for that long.
	MaxIdle time.Duration
	// MaxRenders limits the renders running at once.
	MaxRenders int
	// MaxQueue limits the renders started and not finished yet, queued
	// ones included; Start refuses new ones above it with ErrBusy.
	MaxQueue int
	// RenderTimeout bounds a single render.
	RenderTimeout time.Duration
	// RetryAfter keeps a failed render reported as failed before it may run again.
	RetryAfter time.Duration
	// WaitForPin keeps versions on the volume after a restart until the first
	// Pin, at most firstPinWait: pinned versions are not known before it, and
	// evicting them would mean rendering them all again.
	WaitForPin bool
	// NotFoundRetryAfter is RetryAfter for a version that does not exist or
	// has no documentation: images of a published version do not change, so
	// they are not downloaded again a minute later.
	NotFoundRetryAfter time.Duration

	Render ondemand.RenderFunc
	// RenderList, when set, builds the page of the list of modules
	// (WriteList).
	RenderList ondemand.RenderListFunc
	// Relisted, when set, is told of a module that joins or leaves the list
	// of modules, e.g. one Deckhouse runs the embedded copy of now or no
	// more: its cached versions are no longer right (versions.Versions.Forget).
	Relisted func(module string)
	Logger   *log.Logger
	Metrics  *metricsstorage.MetricStorage // optional

	// Cold, when set, keeps archives of rendered release versions outside
	// the pod: a miss looks there before rendering, a render is uploaded.
	// Development branches change with every push and are not kept.
	Cold ondemand.ColdCache
	// IndexAtModuleRoot makes the links of the search index and llms.txt
	// lead to the module (/modules/<module>/) instead of the version of its
	// target: the public site serves the target there, as its stable channel.
	IndexAtModuleRoot bool
	// Revision of the site template, part of the archive keys; "" keeps the
	// cold cache unused. Called on the first miss, not at the start: it hashes
	// the files of the site template, the Hugo version and the render format.
	Revision func() string
	// IsBranch reports whether a ModulePullOverride pulls the module with
	// this tag. Every render without a revision asks it before S3 is used:
	// a reader may open a branch before the Pinner pins it. Without it every
	// version without a revision counts as a release.
	IsBranch func(ctx context.Context, module, version string) (bool, error)
}

// ErrBusy means too many versions are being rendered to start one more.
var ErrBusy = errors.New("too many module versions are being prepared, try again in a minute")

type published struct {
	size       int64
	lastAccess time.Time
	// ai marks a version rendered with the AI export (Markdown, llms.txt).
	ai bool
	// rev is the image digest a development branch was rendered from; empty
	// for a release version, whose content never changes.
	rev string
	// stale marks a version that came from an archive of another template
	// revision: the leader renders it anew in the background (Rerender).
	stale bool
	// lost marks a stale pinned version the cold cache has no archive of
	// another revision of any more: Rerender, which renders those, never
	// replaces it, so the Pinner renders it anew (uploadMissing).
	lost bool
	// checked is when an access to a stale version last looked for it
	// rendered anew in the cold cache (Touch).
	checked time.Time
}

type render struct {
	status   ondemand.Status
	failedAt time.Time
	// done is closed when the render finishes, either way.
	done chan struct{}
}

// Store keeps rendered module versions on the cache volume: it renders a
// version on request, publishes it atomically and evicts versions by total
// size and by time since the last access.
type Store struct {
	cfg Config
	sem chan struct{}
	now func() time.Time

	// site is the link tree links go to: siteDir, or a new tree while Load
	// rebuilds it next to the one nginx serves.
	site string

	mu       sync.Mutex
	versions map[string]*published
	renders  map[string]*render
	// pinned versions feed the search index and llms.txt and are not evicted.
	pinned map[string]bool
	// pinnedChannels are the pinned versions of release channels that are no
	// target of their module, for the metrics.
	pinnedChannels map[string]bool
	// failedBranches are the development branches among the targets whose
	// last render failed for a lasting reason, with how it failed.
	failedBranches map[ondemand.Target]ondemand.Status
	// pinnedOnce is set by the first Pin; started is when the store was made.
	pinnedOnce bool
	started    time.Time
	// pinSuccessInit reports the start of the first Pinner of the pod as the
	// last update of the pinned versions (initPinSuccessMetric).
	pinSuccessInit sync.Once

	// catalog, channels and index are the list of modules, the links of the
	// release channels and the search index: they are made of the versions
	// the store publishes. indexed keeps the targets the index was built
	// from (targets.json).
	catalog  *catalog.Catalog
	channels *channels.Links
	index    *index.Index
	indexed  *indexedTargets
	// cold keeps the archives of the versions outside the pod; rerender
	// renders anew those of older templates in the render slots.
	cold     *coldArchives
	rerender *rerenderer

	// background is the stale versions readers open looked for rendered
	// anew in the cold cache (Touch). Nothing in the pod waits for it, the
	// tests do.
	background sync.WaitGroup
}

func NewStore(cfg Config) *Store {
	s := &Store{
		cfg:      cfg,
		sem:      make(chan struct{}, cfg.MaxRenders),
		now:      time.Now,
		site:     siteDir,
		started:  time.Now(),
		versions: make(map[string]*published),
		renders:  make(map[string]*render),
		pinned:   make(map[string]bool),

		failedBranches: make(map[ondemand.Target]ondemand.Status),
	}
	s.cold = &coldArchives{
		ColdCache:  cfg.Cold,
		revisionOf: cfg.Revision,
		branch:     cfg.IsBranch,
		dir:        cfg.Dir,
		timeout:    cfg.RenderTimeout,
		logger:     cfg.Logger,
		metrics:    cfg.Metrics,
		putPause:   coldPutPause,
	}
	s.channels = channels.New(cfg.Dir, cfg.Logger, s.published)
	s.catalog = catalog.New(catalog.Config{
		Dir:       cfg.Dir,
		Pages:     filepath.Join(cfg.Dir, indexDir),
		Tmp:       filepath.Join(cfg.Dir, tmpDir),
		Logger:    cfg.Logger,
		Render:    cfg.RenderList,
		Relisted:  cfg.Relisted,
		Published: s.published,
		Gauge:     s.SetGauge,
	})
	s.index = index.New(index.Config{
		Dir:          cfg.Dir,
		Out:          filepath.Join(cfg.Dir, indexDir),
		AtModuleRoot: cfg.IndexAtModuleRoot,
		Publication:  s.publication,
		Stable:       s.channels.Stable,
	})
	s.indexed = &indexedTargets{dir: cfg.Dir, logger: cfg.Logger, cold: s.cold}
	s.rerender = &rerenderer{
		cold:    s.cold,
		slots:   s.sem,
		busy:    s.busy,
		replace: s.replaceStale,
		render:  cfg.Render,
		timeout: cfg.RenderTimeout,
		dir:     cfg.Dir,
		logger:  cfg.Logger,
		gauge:   s.SetGauge,

		idlePoll: rerenderIdlePoll,
	}

	return s
}

func key(module, version string) string {
	return module + "/" + version
}

// Load registers the versions already on the volume, e.g. after a restart of
// docs-builder while nginx kept the volume, removes leftovers of renders and
// evictions that did not finish and rebuilds the links for nginx. The new
// link tree is built next to the one nginx serves and replaces it at the end,
// so the pages stay served while docs-builder starts.
func (s *Store) Load() error {
	newSite, oldSite := siteDir+".new", siteDir+".old"
	for _, dir := range []string{tmpDir, trashDir, newSite, oldSite} {
		if err := os.RemoveAll(filepath.Join(s.cfg.Dir, dir)); err != nil {
			return fmt.Errorf("clean %s: %w", dir, err)
		}
	}

	s.site = newSite
	defer func() { s.site = siteDir }()

	if err := s.loadVersions(); err != nil {
		return err
	}
	s.channels.Keep(siteDir, s.site)

	return s.swapSite(newSite, oldSite)
}

// swapSite puts the rebuilt link tree in place of the served one: two
// renames, the second right after the first.
func (s *Store) swapSite(newSite, oldSite string) error {
	served := filepath.Join(s.cfg.Dir, siteDir)
	if err := os.MkdirAll(filepath.Join(s.cfg.Dir, newSite), 0755); err != nil {
		return fmt.Errorf("mkdir links: %w", err)
	}

	if err := os.Rename(served, filepath.Join(s.cfg.Dir, oldSite)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("replace links: %w", err)
	}
	if err := os.Rename(filepath.Join(s.cfg.Dir, newSite), served); err != nil {
		return fmt.Errorf("replace links: %w", err)
	}

	return os.RemoveAll(filepath.Join(s.cfg.Dir, oldSite))
}

func (s *Store) loadVersions() error {
	modules, err := os.ReadDir(s.cfg.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read cache dir: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range modules {
		if !m.IsDir() || strings.HasPrefix(m.Name(), ".") {
			continue
		}

		versions, err := os.ReadDir(filepath.Join(s.cfg.Dir, m.Name()))
		if err != nil {
			// One bad directory must not keep docs-builder from starting.
			s.cfg.Logger.Warn("skip cached module", slog.String("module", m.Name()), log.Err(err))
			continue
		}

		for _, v := range versions {
			if !v.IsDir() || docs.ValidateModuleTag(m.Name(), v.Name()) != nil {
				continue
			}

			path := filepath.Join(s.cfg.Dir, m.Name(), v.Name())
			if err := s.loadVersion(m.Name(), v); err != nil {
				s.cfg.Logger.Warn("skip cached version", slog.String("path", path), log.Err(err))
			}
		}
	}

	s.reportCacheLocked()

	return nil
}

func (s *Store) loadVersion(module string, v fs.DirEntry) error {
	path := filepath.Join(s.cfg.Dir, module, v.Name())
	info, err := v.Info()
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	size, err := dirSize(path)
	if err != nil {
		return fmt.Errorf("size: %w", err)
	}

	if err := s.link(module, v.Name()); err != nil {
		return err
	}

	// The access time is not kept across restarts; the render time is the
	// closest known one. The revision is not kept either, so a pinned
	// development branch is rendered once more.
	_, staleErr := os.Stat(filepath.Join(path, staleMark))
	s.versions[key(module, v.Name())] = &published{size: size, lastAccess: info.ModTime(), ai: hasAIExport(path), stale: staleErr == nil}

	return nil
}

// Status reports whether the version is rendered, being rendered or failed.
// An unknown version has an empty Stage.
func (s *Store) Status(module, version string) ondemand.Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.statusLocked(key(module, version))
}

func (s *Store) statusLocked(k string) ondemand.Status {
	if _, ok := s.versions[k]; ok {
		return ondemand.Status{Stage: ondemand.StageDone}
	}

	if r, ok := s.renders[k]; ok {
		st := r.status
		if st.Stage == ondemand.StageFailed && !st.NotFound {
			// At least a second: past RetryAfter the next request starts it.
			left := s.cfg.RetryAfter - s.now().Sub(r.failedAt)
			st.RetryIn = max(1, int((left+time.Second-1)/time.Second))
		}

		return st
	}

	return ondemand.Status{}
}

// Start begins rendering the version in the background unless it is already
// rendered, being rendered or failed less than RetryAfter ago, and returns
// its status. Concurrent requests for one version share one render. With
// MaxQueue renders unfinished it returns ErrBusy.
func (s *Store) Start(module, version string) (ondemand.Status, error) {
	if err := docs.ValidateModuleVersion(module, version); err != nil {
		return ondemand.Status{}, err
	}

	k := key(module, version)

	s.mu.Lock()
	defer s.mu.Unlock()

	if r, ok := s.renders[k]; ok && s.retryable(r) {
		delete(s.renders, k)
	}

	if st := s.statusLocked(k); st.Stage != "" {
		return st, nil
	}

	active := 0
	for _, r := range s.renders {
		if r.status.Stage != ondemand.StageFailed {
			active++
		}
	}
	if active >= s.cfg.MaxQueue {
		return ondemand.Status{}, ErrBusy
	}

	r := &render{status: ondemand.Status{Stage: ondemand.StageQueued}, done: make(chan struct{})}
	s.renders[k] = r

	// A pinned version a reader opens first gets the AI export right away, so
	// the Pinner does not render it once more.
	go s.run(context.Background(), module, version, r, s.pinned[k], "") //nolint:errcheck // the result is in r

	return r.status, nil
}

// HasPage reports whether a published version has the page at the URL path
// below its root (a directory counts with its index page). nginx sends a
// request here when it does not find the page, so a page that is there was
// published while the request was on its way.
func (s *Store) HasPage(module, version, lang, page string) bool {
	if docs.ValidateModuleVersion(module, version) != nil || !slices.Contains(docs.Languages, lang) {
		return false
	}

	p := filepath.Join(s.cfg.Dir, module, version, lang, filepath.FromSlash(path.Clean("/"+page)))
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}

	for _, page := range indexPages {
		if _, err := os.Stat(filepath.Join(p, page)); err == nil {
			return true
		}
	}

	return false
}

// staleCheckEvery is how often the accesses to a version served from an
// archive of an older template look for it rendered anew in the cold cache:
// the leader renders such archives anew one by one (Rerender).
const staleCheckEvery = 10 * time.Minute

// Touch records an access to the version, which delays its eviction. A
// version served from an archive of an older template is replaced in the
// background once the cold cache has it rendered anew, looked for at most
// every staleCheckEvery; a pinned one is replaced as it is restored
// (restorePinned).
func (s *Store) Touch(module, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key(module, version)
	v, ok := s.versions[k]
	if !ok {
		return
	}
	v.lastAccess = s.now()
	if !v.stale || s.pinned[k] || s.now().Sub(v.checked) < staleCheckEvery {
		return
	}

	v.checked = s.now()
	withAI := v.ai
	s.background.Add(1)
	go func() {
		defer s.background.Done()
		s.refreshStale(module, version, withAI)
	}()
}

// refreshStale publishes the version from an archive of this template
// revision in place of the one an older revision rendered, when the cold
// cache has it.
func (s *Store) refreshStale(module, version string, withAI bool) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RenderTimeout)
	defer cancel()

	if _, err := s.renderAndPublish(ctx, module, version, withAI, "", coldFresh, func(ondemand.Stage) {}); err != nil && !errors.Is(err, errNotInCold) {
		s.cfg.Logger.Warn("replace a version served from an archive of an older template", slog.String("module", module), slog.String("version", version), log.Err(err))
	}
}

// Run evicts idle versions periodically until ctx is done.
func (s *Store) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.evict()
		}
	}
}

// run renders a version registered in s.renders as r, records the result in
// r and closes r.done.
func (s *Store) run(ctx context.Context, module, version string, r *render, withAI bool, rev string) error {
	defer close(r.done)

	k := key(module, version)
	logger := s.cfg.Logger.With(slog.String("module", module), slog.String("version", version))
	start := s.now()

	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.renders, k)
		s.mu.Unlock()
		return ctx.Err()
	}
	defer func() { <-s.sem }()

	ctx, cancel := context.WithTimeout(ctx, s.cfg.RenderTimeout)
	defer cancel()

	size, err := s.renderAndPublish(ctx, module, version, withAI, rev, coldOrRender, func(stage ondemand.Stage) {
		s.mu.Lock()
		r.status.Stage = stage
		s.mu.Unlock()
	})

	s.mu.Lock()
	result := ondemand.StageDone
	if err != nil {
		r.status = failedStatus(err)
		r.failedAt = s.now()
		result = r.status.Stage
	} else {
		delete(s.renders, k)
	}
	notFound := r.status.NotFound
	s.mu.Unlock()

	s.reportRender(result, notFound, s.now().Sub(start))

	if err != nil {
		logger.Warn("render version", slog.Bool("pinned", withAI), log.Err(err))
		return err
	}

	logger.Info("rendered version", slog.Bool("pinned", withAI), slog.Duration("duration", s.now().Sub(start)), slog.Int64("size", size))
	s.evict()

	return nil
}

// renderAndPublish renders into a temporary directory on the cache volume and
// renames the result into place, so nginx never sees a partial version. A
// version already published is replaced: it goes to the trash first. With
// coldOnly or coldFresh a version missing from the cold cache is not
// rendered: errNotInCold.
func (s *Store) renderAndPublish(ctx context.Context, module, version string, withAI bool, rev string, mode coldMode, stage func(ondemand.Stage)) (int64, error) {
	tmp := filepath.Join(s.cfg.Dir, tmpDir)
	if err := os.MkdirAll(tmp, 0700); err != nil {
		return 0, fmt.Errorf("mkdir tmp: %w", err)
	}

	workDir, err := os.MkdirTemp(tmp, module+"-"+version+"-")
	if err != nil {
		return 0, fmt.Errorf("mkdir work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	outDir := filepath.Join(workDir, "out")
	// A development branch (rev) is kept in the pod only: it is neither read
	// from nor written to S3, and is rendered again after a restart.
	revision := ""
	if rev == "" {
		revision = s.cold.revision()
	}
	if revision != "" && s.cold.isBranch(ctx, module, version) {
		revision = ""
	}
	var found, archive string
	var stale bool
	if revision != "" {
		stage(ondemand.StageDownload)
		start := time.Now()
		found, archive, stale = s.cold.fetch(ctx, revision, module, version, outDir, withAI, mode)
		if found != "" {
			ondemand.ObserveStage(s.cfg.Metrics, "download", "cold", start)
		}
	}
	defer func() {
		if archive != "" {
			os.Remove(archive)
		}
	}()
	fromCold := found != ""
	if !fromCold && mode != coldOrRender {
		return 0, errNotInCold
	}
	if stale {
		if err := os.WriteFile(filepath.Join(outDir, staleMark), nil, 0644); err != nil {
			return 0, fmt.Errorf("mark stale version: %w", err)
		}
	}
	if !fromCold {
		if err := s.cfg.Render(ctx, module, version, workDir, outDir, withAI, stage); err != nil {
			return 0, err
		}
	}
	publishStart := time.Now()
	// The export degrades to a warning, and a reader may get the archive of a
	// pinned version, with the export: check what is there.
	withAI = hasAIExport(outDir)

	size, err := dirSize(outDir)
	if err != nil {
		return 0, fmt.Errorf("size of rendered version: %w", err)
	}

	target := filepath.Join(s.cfg.Dir, module, version)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return 0, fmt.Errorf("mkdir module dir: %w", err)
	}

	// Pages are served by nginx from another container.
	if err := os.Chmod(outDir, 0755); err != nil {
		return 0, fmt.Errorf("chmod rendered version: %w", err)
	}

	var snapshot string
	if revision != "" && !fromCold {
		snapshot = s.cold.snapshot(outDir)
	}
	defer func() {
		if snapshot != "" {
			os.RemoveAll(snapshot)
		}
	}()

	old, err := s.publish(module, version, outDir, &published{size: size, ai: withAI, rev: rev, stale: stale})
	if old != "" {
		if err := os.RemoveAll(old); err != nil {
			s.cfg.Logger.Warn("remove replaced version", slog.String("path", old), log.Err(err))
		}
	}
	if err != nil {
		return 0, err
	}
	ondemand.ObserveStage(s.cfg.Metrics, "publish", "", publishStart)

	switch {
	case fromCold:
		s.cfg.Logger.Info("version from S3", slog.String("module", module), slog.String("version", version), slog.Bool("stale", stale))
		// S3 does not track reads: the archive goes up again, so its age
		// is when a pod last needed it, which the eviction goes by.
		s.cold.uploadAgain(found, archive)
		archive = ""
	// An override may have appeared during the render.
	case snapshot != "" && !s.cold.isBranch(ctx, module, version):
		drop := ""
		if withAI {
			drop = coldKey(revision, module, version, false)
		}
		s.cold.upload(coldKey(revision, module, version, withAI), drop, snapshot)
		snapshot = ""
	}

	return size, nil
}

// publish renames a rendered version into place and links it for nginx under
// the lock, so an eviction never sees it half done. It returns the trash path
// of the version it replaced, for the caller to remove outside the lock. When
// the new version can not take the place, the replaced one is put back.
func (s *Store) publish(module, version, outDir string, p *published) (string, error) {
	k := key(module, version)
	target := filepath.Join(s.cfg.Dir, module, version)

	s.mu.Lock()
	defer s.mu.Unlock()

	var old string
	prev := s.versions[k]
	if _, err := os.Stat(target); err == nil {
		if old, err = s.moveToTrash(k); err != nil {
			return "", fmt.Errorf("replace rendered version: %w", err)
		}
		delete(s.versions, k)
	}

	if err := os.Rename(outDir, target); err != nil {
		return s.putBackLocked(k, old, prev, fmt.Errorf("publish rendered version: %w", err))
	}

	if err := s.link(module, version); err != nil {
		// Unreachable for nginx anyway; leave the path free for the replaced
		// version or the next render.
		s.unlink(module, version)
		os.RemoveAll(target)
		return s.putBackLocked(k, old, prev, err)
	}

	p.lastAccess = s.now()
	s.versions[k] = p
	s.reportCacheLocked()

	return old, nil
}

// putBackLocked returns the version publish moved to the trash entry old to
// its place after the new one failed with cause, so the last good version
// stays served; prev is how it was registered. It returns what publish
// does: the trash entry is left to the caller only when the version could
// not be put back.
func (s *Store) putBackLocked(k, old string, prev *published, cause error) (string, error) {
	if old == "" {
		return "", cause
	}
	if err := os.Rename(filepath.Join(old, "version"), filepath.Join(s.cfg.Dir, k)); err != nil {
		return old, fmt.Errorf("%w; put the replaced version back: %w", cause, err)
	}
	if err := os.Remove(old); err != nil {
		s.cfg.Logger.Warn("remove trash entry", slog.String("path", old), log.Err(err))
	}
	// A directory that was on the volume without being a version stays one.
	if prev == nil {
		return "", cause
	}

	s.versions[k] = prev
	s.reportCacheLocked()
	module, version, _ := strings.Cut(k, "/")
	if err := s.link(module, version); err != nil {
		return "", fmt.Errorf("%w; restore links: %w", cause, err)
	}

	return "", cause
}

// Pin replaces the set of pinned versions: the targets, those the search
// index and llms.txt are built from, and the versions of their release
// channels. They are not evicted; a version that is no longer pinned is
// evicted by the usual rules.
func (s *Store) Pin(targets, channels []ondemand.Target) {
	pinned := make(map[string]bool, len(targets)+len(channels))
	for _, t := range targets {
		pinned[key(t.Module, t.Version)] = true
	}
	pinnedChannels := make(map[string]bool, len(channels))
	for _, t := range channels {
		k := key(t.Module, t.Version)
		if !pinned[k] {
			pinned[k] = true
			pinnedChannels[k] = true
		}
	}

	s.mu.Lock()
	s.pinned = pinned
	s.pinnedChannels = pinnedChannels
	s.pinnedOnce = true
	s.reportCacheLocked()
	s.mu.Unlock()
}

// LinkChannels points the release channels of the module at their published
// versions for nginx (channels.Links).
func (s *Store) LinkChannels(module string, chans ondemand.Channels) {
	s.channels.Link(s.site, module, chans)
}

// SetChannels links the release channels of every module in chans
// (LinkChannels) and removes the channel links of the modules no longer in
// it.
func (s *Store) SetChannels(chans map[string]ondemand.Channels) {
	s.channels.Set(s.site, chans)
}

// WriteIndex merges the search indexes and the AI exports of the published
// targets into the files of indexDir (index).
func (s *Store) WriteIndex(targets []ondemand.Target) error {
	return s.index.WriteIndex(targets)
}

// WriteList builds the page of the list of modules (catalog).
func (s *Store) WriteList(ctx context.Context, modules []ondemand.Listed, targets []ondemand.Target) error {
	return s.catalog.WriteList(ctx, modules, targets)
}

// SetTargets records the target of every module (catalog). A development
// branch that is no target any more, or has moved on, fails no more.
func (s *Store) SetTargets(targets []ondemand.Target) {
	s.catalog.SetTargets(targets)

	byName := ondemand.ByModule(targets)
	s.mu.Lock()
	defer s.mu.Unlock()

	for t := range s.failedBranches {
		if byName[t.Module] != t {
			delete(s.failedBranches, t)
		}
	}
	s.SetGauge(metrics.DocsBuilderOnDemandFailedBranches, float64(len(s.failedBranches)))
}

// Target returns the target of the module (catalog).
func (s *Store) Target(module string) (ondemand.Target, bool) {
	return s.catalog.Target(module)
}

// Pending reports a module on the list whose target this pod does not know
// yet (catalog).
func (s *Store) Pending(module string) bool {
	return s.catalog.Pending(module)
}

// NoDocs reports a module on the list whose published versions have no
// documentation (catalog).
func (s *Store) NoDocs(module string) bool {
	return s.catalog.NoDocs(module)
}

// Menu returns the version menu of the module: whether it is enabled, from
// the list, and the channels as their links point; false when the module is
// neither on the list nor linked.
func (s *Store) Menu(module string) (ondemand.ModuleMenu, bool) {
	m, listed := s.catalog.Listed(module)
	chans, linked := s.channels.Served(module)
	if !listed && !linked {
		return ondemand.ModuleMenu{}, false
	}
	if chans == nil {
		chans = ondemand.Channels{}
	}

	return ondemand.ModuleMenu{Enabled: m.Enabled, Channels: chans}, true
}

// RenderPinned renders a pinned version with the AI export unless it is
// already published that way from the same revision. A version published
// from an archive of an older template is replaced once the cold cache has
// one of this template (restorePinned), and rendered anew when the cold cache
// has lost the archives of older templates (lost). It waits for the render
// and takes one render slot, so a caller rendering pinned versions one by one
// leaves the other slots to the readers. The render is registered like a
// reader's one: a reader opening the version meanwhile shares it, and a
// reader's render already running is waited for first.
func (s *Store) RenderPinned(ctx context.Context, t ondemand.Target) error {
	return s.renderPinned(ctx, t, true)
}

func (s *Store) renderPinned(ctx context.Context, t ondemand.Target, withAI bool) (err error) {
	if err := docs.ValidateModuleTag(t.Module, t.Version); err != nil {
		return err
	}
	if t.Rev != "" {
		defer func() { s.recordBranch(t, err) }()
	}

	k := key(t.Module, t.Version)
	for {
		s.mu.Lock()
		if v, ok := s.versions[k]; ok && (v.ai || !withAI) && v.rev == t.Rev && !v.lost {
			stale := v.stale
			s.mu.Unlock()
			if stale {
				_, err := s.restorePinned(ctx, t, withAI)
				return err
			}
			return nil
		}

		if r, ok := s.renders[k]; ok && r.status.Stage != ondemand.StageFailed {
			done := r.done
			s.mu.Unlock()

			select {
			case <-done:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		r := &render{status: ondemand.Status{Stage: ondemand.StageQueued}, done: make(chan struct{})}
		s.renders[k] = r
		s.mu.Unlock()

		if err := s.run(ctx, t.Module, t.Version, r, withAI, t.Rev); err != nil {
			return err
		}

		s.mu.Lock()
		v, ok := s.versions[k]
		s.mu.Unlock()
		if ok && withAI && !v.ai {
			return ondemand.ErrNoAIExport
		}

		return nil
	}
}

// failedStatus is the status of a render that failed with err.
func failedStatus(err error) ondemand.Status {
	k := ondemand.KindOf(err)

	return ondemand.Status{
		Stage:    ondemand.StageFailed,
		Error:    reason(err),
		NotFound: k == ondemand.Missing || k == ondemand.NoDocs,
	}
}

func reason(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return ondemand.ReasonTimeout
	// The image is fetched, its files are the reason.
	case ondemand.KindOf(err) == ondemand.Unrenderable:
		return ondemand.ReasonBuild
	case errors.Is(err, ondemand.ErrFetch):
		return ondemand.ReasonRegistry
	default:
		return ondemand.ReasonBuild
	}
}

// recordBranch keeps how the render of a development branch failed for a
// lasting reason, so its address tells it instead of waiting for it; a
// render that went well forgets it, one failing for a passing reason leaves
// it as it was.
func (s *Store) recordBranch(t ondemand.Target, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	// Published without the AI export: the pages are served.
	case err == nil || errors.Is(err, ondemand.ErrNoAIExport):
		delete(s.failedBranches, t)
	case ondemand.Lasting(err):
		s.failedBranches[t] = failedStatus(err)
	default:
		return
	}
	s.SetGauge(metrics.DocsBuilderOnDemandFailedBranches, float64(len(s.failedBranches)))
}

// BranchFailure reports how the development branch failed to render for a
// lasting reason; false while it may still be published.
func (s *Store) BranchFailure(t ondemand.Target) (ondemand.Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.failedBranches[t]

	return st, ok
}

// restorePinned publishes a pinned release version from the cold cache
// unless it is published already, and reports whether it is published now.
// A target whose archive has no AI export, as the leader serves it when the
// export failed, is published without it. A version published from an
// archive of an older template, or without the AI export it is pinned with,
// is replaced once the cold cache has an archive of this revision, with the
// export: the leader rendered it anew, or a reader's version became a
// target. A version missing from the cold cache, a development branch and a
// version being rendered are left to RenderPinned. It takes no render slot:
// it is a download of a few megabytes.
func (s *Store) restorePinned(ctx context.Context, t ondemand.Target, withAI bool) (bool, error) {
	if err := docs.ValidateModuleTag(t.Module, t.Version); err != nil {
		return false, err
	}

	k := key(t.Module, t.Version)
	s.mu.Lock()
	v, published := s.versions[k]
	replace := published && (v.stale || withAI && !v.ai)
	_, rendering := s.renders[k]
	s.mu.Unlock()
	if published && !replace || rendering || t.Rev != "" || s.cold.revision() == "" {
		return published, nil
	}

	mode := coldOnly
	if published {
		mode = coldFresh
	}
	_, err := s.renderAndPublish(ctx, t.Module, t.Version, withAI, "", mode, func(ondemand.Stage) {})
	if errors.Is(err, errNotInCold) && withAI && !published {
		_, err = s.renderAndPublish(ctx, t.Module, t.Version, false, "", coldOnly, func(ondemand.Stage) {})
	}
	// A published version stays served as it was until the archive is there.
	if errors.Is(err, errNotInCold) {
		return published, nil
	}
	if err != nil {
		return published, err
	}

	return true, nil
}

// publishedAsWanted reports whether the version of the target is published
// from this template revision, with the AI export when withAI.
func (s *Store) publishedAsWanted(t ondemand.Target, withAI bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.versions[key(t.Module, t.Version)]

	return ok && !v.stale && (v.ai || !withAI)
}

// published reports whether a version of the target is published, from
// any revision.
func (s *Store) published(t ondemand.Target) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.versions[key(t.Module, t.Version)]

	return ok
}

// publication returns how the version of the target is published, for the
// index; false when it is not.
func (s *Store) publication(t ondemand.Target) (index.Published, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.versions[key(t.Module, t.Version)]
	if !ok {
		return index.Published{}, false
	}

	return index.Published{Rev: v.rev, AI: v.ai}, true
}

// hasAIExport reports whether a version on the volume was rendered with the
// AI export: every language it has pages in has llms.txt. The export runs
// and fails per language.
func hasAIExport(path string) bool {
	langs := 0
	for _, lang := range docs.Languages {
		if _, err := os.Stat(filepath.Join(path, lang)); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(path, lang, "llms.txt")); err != nil {
			return false
		}
		langs++
	}

	return langs > 0
}

// link makes a published version visible to nginx under the URL layout:
// .site/<lang>/modules/<module>/<version> -> ../../../../<module>/<version>/<lang>.
// The version stays in one directory, so it is published and evicted as a
// whole; the links appear right after it and go away before it.
func (s *Store) link(module, version string) error {
	for _, lang := range docs.Languages {
		if _, err := os.Stat(filepath.Join(s.cfg.Dir, module, version, lang)); err != nil {
			continue
		}

		dir := filepath.Join(s.cfg.Dir, s.site, lang, "modules", module)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir links: %w", err)
		}

		link := filepath.Join(dir, version)
		// A link left by an eviction that did not finish.
		if err := os.Remove(link); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove old link: %w", err)
		}

		if err := os.Symlink(filepath.Join("..", "..", "..", "..", module, version, lang), link); err != nil {
			return fmt.Errorf("link %s pages: %w", lang, err)
		}
	}

	return nil
}

// unlink hides a version from nginx before it is removed.
func (s *Store) unlink(module, version string) {
	for _, lang := range docs.Languages {
		link := filepath.Join(s.cfg.Dir, s.site, lang, "modules", module, version)
		if err := os.Remove(link); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.cfg.Logger.Warn("remove link", slog.String("path", link), log.Err(err))
		}
	}
}

// evict removes versions idle for longer than MaxIdle, then the least
// recently opened ones while the total size of the versions that are not
// pinned is above MaxSize. A version is
// first renamed into the trash under the lock, so a new render of it can not
// collide with the removal.
func (s *Store) evict() {
	type candidate struct {
		key string
		*published
	}

	s.mu.Lock()

	candidates := make([]candidate, 0, len(s.versions))
	var total int64
	for k, v := range s.versions {
		if !s.pinned[k] {
			total += v.size
			candidates = append(candidates, candidate{k, v})
		}
	}

	slices.SortFunc(candidates, func(a, b candidate) int { return a.lastAccess.Compare(b.lastAccess) })

	now := s.now()

	// Forget failed renders that may run again, so requests for versions that
	// do not exist do not pile up.
	for k, r := range s.renders {
		if s.retryable(r) {
			delete(s.renders, k)
		}
	}

	// Before the first Pin pinned versions are not known; keep everything.
	waiting := s.cfg.WaitForPin && !s.pinnedOnce && now.Sub(s.started) < firstPinWait

	var trash []string
	for _, c := range candidates {
		if waiting {
			break
		}
		if total <= s.cfg.MaxSize && now.Sub(c.lastAccess) <= s.cfg.MaxIdle {
			break
		}

		path, err := s.moveToTrash(c.key)
		if err != nil {
			s.cfg.Logger.Warn("evict version", slog.String("version", c.key), log.Err(err))
			continue
		}

		delete(s.versions, c.key)
		total -= c.size
		trash = append(trash, path)
	}

	s.reportCacheLocked()
	s.mu.Unlock()

	for _, path := range trash {
		if err := os.RemoveAll(path); err != nil {
			s.cfg.Logger.Warn("remove evicted version", slog.String("path", path), log.Err(err))
		}
	}
}

// retryable reports whether a render failed long enough ago to run again.
func (s *Store) retryable(r *render) bool {
	if r.status.Stage != ondemand.StageFailed {
		return false
	}

	after := s.cfg.RetryAfter
	if r.status.NotFound {
		after = s.cfg.NotFoundRetryAfter
	}

	return s.now().Sub(r.failedAt) >= after
}

// moveToTrash hides the version from nginx and renames it into the trash. If
// the rename fails, the version stays published: its links come back, so
// nginx keeps serving it.
func (s *Store) moveToTrash(k string) (string, error) {
	module, version, _ := strings.Cut(k, "/")

	trash := filepath.Join(s.cfg.Dir, trashDir)
	if err := os.MkdirAll(trash, 0700); err != nil {
		return "", fmt.Errorf("mkdir trash: %w", err)
	}

	dst, err := os.MkdirTemp(trash, strings.ReplaceAll(k, "/", "-")+"-")
	if err != nil {
		return "", fmt.Errorf("mkdir trash entry: %w", err)
	}

	s.unlink(module, version)

	if err := os.Rename(filepath.Join(s.cfg.Dir, k), filepath.Join(dst, "version")); err != nil {
		os.Remove(dst)
		if lerr := s.link(module, version); lerr != nil {
			return "", fmt.Errorf("move to trash: %w; restore links: %w", err, lerr)
		}
		return "", fmt.Errorf("move to trash: %w", err)
	}

	return dst, nil
}

// SetGauge sets a gauge without labels, when the store has metrics.
func (s *Store) SetGauge(name string, v float64) {
	if s.cfg.Metrics != nil {
		s.cfg.Metrics.GaugeSet(name, v, nil)
	}
}

// initPinSuccessMetric reports at, the start of the first Pinner of the pod,
// as the last update of the pinned versions until a pass updates them: the
// alert on it then fires for a new pod or a new leader that never finishes a
// pass, as for one that stopped finishing them. The Pinners of later terms of
// the leader leave it: a Lease lost every hour must not keep the alert off.
func (s *Store) initPinSuccessMetric(at time.Time) {
	s.pinSuccessInit.Do(func() { s.SetGauge(metrics.DocsBuilderOnDemandPinSuccessSeconds, float64(at.Unix())) })
}

func (s *Store) reportRender(result ondemand.Stage, notFound bool, d time.Duration) {
	if s.cfg.Metrics == nil {
		return
	}

	status := string(result)
	if notFound {
		status = "not_found"
	}

	labels := map[string]string{"status": status}
	s.cfg.Metrics.CounterAdd(metrics.DocsBuilderOnDemandRenderTotal, 1, labels)
	s.cfg.Metrics.HistogramObserve(metrics.DocsBuilderOnDemandRenderDurationSeconds, d.Seconds(), labels, nil)
}

func (s *Store) reportCacheLocked() {
	if s.cfg.Metrics == nil {
		return
	}

	var total, pinned int64
	kinds := map[string]float64{"target": 0, "channel": 0, "reader": 0}
	for k, v := range s.versions {
		total += v.size
		switch {
		case s.pinnedChannels[k]:
			pinned += v.size
			kinds["channel"]++
		case s.pinned[k]:
			pinned += v.size
			kinds["target"]++
		default:
			kinds["reader"]++
		}
	}

	s.cfg.Metrics.GaugeSet(metrics.DocsBuilderOnDemandCacheVersions, float64(len(s.versions)), nil)
	for kind, n := range kinds {
		s.cfg.Metrics.GaugeSet(metrics.DocsBuilderOnDemandCacheKindVersions, n, map[string]string{"kind": kind})
	}
	s.cfg.Metrics.GaugeSet(metrics.DocsBuilderOnDemandCacheBytes, float64(total), nil)
	// Eviction never takes pinned versions: past their budget they and the
	// readers' versions may fill the cache volume.
	s.cfg.Metrics.GaugeSet(metrics.DocsBuilderOnDemandCachePinnedBytes, float64(pinned), nil)
	s.cfg.Metrics.GaugeSet(metrics.DocsBuilderOnDemandCacheReadersBytes, float64(total-pinned), nil)
	if s.cfg.MaxSize > 0 {
		s.cfg.Metrics.GaugeSet(metrics.DocsBuilderOnDemandCacheReadersBudget, float64(s.cfg.MaxSize), nil)
	}
	if s.cfg.PinnedBudget > 0 {
		s.cfg.Metrics.GaugeSet(metrics.DocsBuilderOnDemandCachePinnedBudget, float64(s.cfg.PinnedBudget), nil)
		s.cfg.Metrics.GaugeSet(metrics.DocsBuilderOnDemandCacheVolumeBytes, float64(s.cfg.PinnedBudget+s.cfg.MaxSize), nil)
	}
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}

		return nil
	})

	return size, err
}
