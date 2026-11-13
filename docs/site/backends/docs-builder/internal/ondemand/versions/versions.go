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

// Package versions lists the published versions of a module from its release
// tags, cached for the pages that show them and the pinner.
package versions

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"golang.org/x/mod/semver"
	"golang.org/x/sync/singleflight"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/ondemand"
)

// listTimeout bounds one listing of the release tags of a module.
const listTimeout = time.Minute

// Versions lists the published versions of a module, newest first.
//
// ponytail: a stand-in that reads release tags from the registry, until the
// cluster has its own list of module versions.
type Versions struct {
	list      ondemand.TagLister
	ttl       time.Duration
	failedTTL time.Duration
	now       func() time.Time
	ms        *metricsstorage.MetricStorage // optional
	// Logger, when set, gets the failed listings a cached list hides.
	Logger *log.Logger
	// hasWait bounds the wait of Has for a relisting.
	hasWait time.Duration

	mu    sync.Mutex
	cache map[string]cachedVersions
	// forgotten counts the Forget calls of a module: a listing that started
	// before one does not cache what it found.
	forgotten map[string]uint64
	// group makes concurrent misses of one module share one listing.
	group singleflight.Group
}

type cachedVersions struct {
	versions []string
	err      error
	// until is when the registry is listed again; listed is when it was
	// last asked.
	until  time.Time
	listed time.Time
	// relist is the age of the list at which a version missing from it
	// makes the registry be listed again (Has).
	relist time.Duration
	// failed marks a list kept after a failed listing.
	failed bool
}

// A version missing from a list minRelist old makes the registry be listed
// again: a version published since opens right away. A relisting that finds
// nothing new doubles the wait for the next one, up to the TTL, so a crawler
// walking made-up versions of a module lists it a few times per TTL; a
// changed list starts from minRelist again. A reader waits for a relisting
// hasWait at most, then gets "not found" while it goes on.
const (
	minRelist = time.Minute
	hasWait   = 10 * time.Second
)

// New caches a listing for ttl and a failed one for failedTTL, so a module
// whose registry can not be listed does not send every page view to the
// registry.
func New(list ondemand.TagLister, ttl, failedTTL time.Duration, ms *metricsstorage.MetricStorage) *Versions {
	return &Versions{list: list, ttl: ttl, failedTTL: failedTTL, now: time.Now, ms: ms, hasWait: hasWait, cache: make(map[string]cachedVersions)}
}

// List returns the versions of the module that can be rendered, cached for
// the TTL: a release repository holds hundreds of tags. Once the TTL is over
// the cached list is returned at once and the registry is listed in the
// background; a module without a list, or whose last listing failed, waits
// for the listing.
func (v *Versions) List(ctx context.Context, module string) ([]string, error) {
	c, ok := v.cached(module)
	if ok && v.now().Before(c.until) {
		return c.versions, c.err
	}

	ch := v.listing(ctx, module)
	if ok && c.err == nil {
		return c.versions, nil
	}

	return v.wait(ctx, ch)
}

// Latest returns the newest published version of the module, empty when
// there is none.
func (v *Versions) Latest(ctx context.Context, module string) (string, error) {
	list, err := v.List(ctx, module)
	if err != nil || len(list) == 0 {
		return "", err
	}

	return list[0], nil
}

// Incomplete reports whether the cached list of the module may miss
// published versions: it is kept after a failed listing, or some sources of
// the module did not answer.
func (v *Versions) Incomplete(module string) bool {
	c, ok := v.cached(module)

	return ok && c.failed
}

// errStaleList means the version is missing from a list kept after a failed
// listing: whether it is published is not known.
var errStaleList = errors.New("the list of versions is the last one the registry gave")

// Has reports whether the version is published. A version missing from the
// list may have been published since: see minRelist. Missing from a list kept
// after a failed listing, it is not known: the error is errStaleList.
func (v *Versions) Has(ctx context.Context, module, version string) (bool, error) {
	versions, err := v.List(ctx, module)
	if err != nil || slices.Contains(versions, version) {
		return err == nil, err
	}

	c, ok := v.cached(module)
	switch {
	case !ok || c.err != nil:
		return false, nil
	case c.failed:
		return false, errStaleList
	}
	wait := max(c.relist, minRelist)
	if v.now().Sub(c.listed) < wait {
		return false, nil
	}

	waitCtx, cancel := context.WithTimeout(ctx, v.hasWait)
	defer cancel()
	relisted, err := v.wait(waitCtx, v.listing(ctx, module))
	switch {
	case ctx.Err() != nil:
		return false, ctx.Err()
	case waitCtx.Err() != nil:
		// The relisting goes on; this reader gets "not found".
		return false, nil
	case err != nil:
		return false, err
	}
	found := slices.Contains(relisted, version)

	v.mu.Lock()
	defer v.mu.Unlock()
	cur, ok := v.cache[module]
	if !ok || cur.err != nil {
		return found, nil
	}
	if cur.failed {
		return found, errStaleListIfMissing(found)
	}
	if slices.Equal(relisted, versions) {
		cur.relist = min(2*wait, v.ttl)
	} else {
		cur.relist = minRelist
	}
	v.cache[module] = cur

	return found, nil
}

func errStaleListIfMissing(found bool) error {
	if found {
		return nil
	}
	return errStaleList
}

// listing lists the module in the registry, once for all who ask meanwhile.
// The listing does not depend on the request that triggered it.
func (v *Versions) listing(ctx context.Context, module string) <-chan singleflight.Result {
	return v.group.DoChan(module, func() (any, error) {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), listTimeout)
		defer cancel()
		return v.load(ctx, module)
	})
}

func (v *Versions) wait(ctx context.Context, ch <-chan singleflight.Result) ([]string, error) {
	select {
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}
		return res.Val.([]string), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (v *Versions) load(ctx context.Context, module string) ([]string, error) {
	v.mu.Lock()
	gen := v.forgotten[module]
	v.mu.Unlock()
	start := time.Now()
	tags, err := v.list(ctx, module)
	if v.ms != nil {
		status := "ok"
		if err != nil {
			status = "fail"
		}
		v.ms.HistogramObserve(metrics.DocsBuilderOnDemandVersionsListSeconds, time.Since(start).Seconds(), map[string]string{"status": status}, nil)
	}
	prev, ok := v.cached(module)
	// Some sources answered: their versions are kept as a failed list, read
	// again after failedTTL.
	partial := errors.Is(err, ondemand.ErrPartialList)
	if err != nil && !partial {
		// A failed listing keeps the last list; the registry is asked again
		// after failedTTL. A module gone from the cluster or the registry
		// loses it.
		if ok && prev.err == nil && !ondemand.Unpublished(err) {
			v.warn(module, err)
			v.store(module, gen, cachedVersions{versions: prev.versions, until: v.now().Add(v.failedTTL), listed: v.now(), relist: prev.relist, failed: true})
			return prev.versions, nil
		}
		v.store(module, gen, cachedVersions{err: err, until: v.now().Add(v.failedTTL), listed: v.now()})
		return nil, err
	}

	versions := releaseVersions(module, tags)
	until := v.now().Add(v.ttl)
	if partial {
		v.warn(module, err)
		until = v.now().Add(v.failedTTL)
	}
	// A list that changed lets a missing version relist after minRelist again.
	relist := time.Duration(0)
	if ok && slices.Equal(prev.versions, versions) {
		relist = prev.relist
	}
	v.store(module, gen, cachedVersions{versions: versions, until: until, listed: v.now(), relist: relist, failed: partial})

	return versions, nil
}

func (v *Versions) warn(module string, err error) {
	if v.Logger != nil {
		v.Logger.Warn("list module versions, some versions may be missing", slog.String("module", module), log.Err(err))
	}
}

// Forget drops the cached versions of the module: the next request lists it
// anew.
func (v *Versions) Forget(module string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	delete(v.cache, module)
	if v.forgotten == nil {
		v.forgotten = make(map[string]uint64)
	}
	v.forgotten[module]++
	// A listing in flight may have read the module before the change: the
	// next request starts its own.
	v.group.Forget(module)
}

func (v *Versions) cached(module string) (cachedVersions, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.cache[module]

	return c, ok
}

// store caches a listing of generation gen and drops the expired failures, so names of modules
// that do not exist, taken from request URLs, do not pile up. A list stays:
// it is served while the next listing runs, and only real modules have one.
func (v *Versions) store(module string, gen uint64, c cachedVersions) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.forgotten[module] != gen {
		// Forgotten while it was listed: what it found may be stale.
		return
	}

	for m, old := range v.cache {
		if old.err != nil && !v.now().Before(old.until) {
			delete(v.cache, m)
		}
	}
	v.cache[module] = c
}

// releaseVersions keeps the tags that are module versions, without release
// channels and development tags, deduplicated and newest first.
func releaseVersions(module string, tags []string) []string {
	versions := make([]string, 0, len(tags))
	for _, tag := range tags {
		if semver.IsValid(tag) && docs.ValidateModuleVersion(module, tag) == nil {
			versions = append(versions, tag)
		}
	}

	slices.SortFunc(versions, func(a, b string) int { return semver.Compare(b, a) })

	return slices.Compact(versions)
}
