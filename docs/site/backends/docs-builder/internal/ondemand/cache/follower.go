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
	"bytes"
	"context"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/ondemand"
)

// FollowFirstEvery is how often a follower looks for the targets of the
// leader until it finds them: with an empty bucket the leader writes them a
// few seconds after it starts, and the list of modules waits for them.
const FollowFirstEvery = 5 * time.Second

// FollowFallbackAfter is how long a follower goes without reading the
// targets of the leader before it reads the registry for all of them.
const FollowFallbackAfter = 10 * time.Minute

// followerStore is what a Follower needs of the Store: the targets.json of
// the leader and the archives in the cold cache, the pinned versions, the
// list of modules, the channel links, the search index and the metrics.
type followerStore interface {
	coldGetList(ctx context.Context, key string) ([]byte, error)
	restoreAll(ctx context.Context, targets []ondemand.Target, chans map[string]ondemand.Channels, logger *log.Logger) (int, bool)

	renderPinned(ctx context.Context, t ondemand.Target, withAI bool) error
	published(t ondemand.Target) bool

	SetTargets(targets []ondemand.Target)
	WriteList(ctx context.Context, modules []ondemand.Listed, targets []ondemand.Target) error
	SetChannels(chans map[string]ondemand.Channels)

	WriteIndex(targets []ondemand.Target) error
	saveIndexedLocal(state indexedState) (string, error)

	SetGauge(name string, v float64)
}

// Follower keeps a pod that is not the leader in step with it: it takes the
// targets and the release channels the leader recorded in the cold cache
// (targets.json), publishes their versions from the archives there, links
// the channels and builds the search index and llms.txt from the versions,
// without Hugo, and the list of modules. It reads the registry only for a
// development branch (ModulePullOverride), which is never in S3.
type Follower struct {
	store  followerStore
	logger *log.Logger
	now    func() time.Time

	// applied is the targets.json whose versions are all published; a change
	// of it, or a version missing from it, is applied on the next sync.
	applied []byte
	// retryAt is when the targets are applied again although applied is
	// unchanged: a development branch failed for a lasting reason, and
	// waits for retryAfter, as the Pinner of the leader does.
	retryAt    time.Time
	retryAfter time.Duration
	// found is set once targets.json was there; until then Run looks for it
	// more often than every, from firstEvery on.
	found      bool
	firstEvery time.Duration
}

func NewFollower(store followerStore, retryAfter time.Duration, logger *log.Logger) *Follower {
	return &Follower{store: store, retryAfter: retryAfter, logger: logger, now: time.Now, firstEvery: FollowFirstEvery}
}

// Sync applies the targets of the leader once. It reports whether the cold
// cache answered (answered: a missing targets.json counts, the leader has
// not written one yet) and whether everything the leader recorded is served
// here (complete), from the current template or, until the leader renders it
// anew, from an older one.
func (f *Follower) Sync(ctx context.Context) (bool, bool) {
	raw, err := f.store.coldGetList(ctx, indexedFile)
	if err != nil {
		// The leader has written no targets.json yet. A cold cache that is
		// not set up yet, unlike for the store, is S3 not answering: the
		// fallback reads the registry then.
		if ondemand.KindOf(err) == ondemand.Missing {
			return true, true
		}
		f.logger.Warn("get the targets of the leader from S3", log.Err(err))
		return false, false
	}
	f.found = true
	if bytes.Equal(raw, f.applied) && (f.retryAt.IsZero() || f.now().Before(f.retryAt)) {
		return true, true
	}

	state, err := parseIndexed(raw, f.logger)
	if err != nil {
		// Counts as S3 not answering: unchanged, the fallback starts.
		f.logger.Warn("read the targets of the leader", log.Err(err))
		return false, false
	}

	// Every module is on the list at once, by its name until its version
	// comes from S3.
	f.store.SetTargets(state.Targets)
	if err := f.store.WriteList(ctx, state.Modules, state.Targets); err != nil && ctx.Err() == nil {
		f.logger.Warn("write the list of modules", log.Err(err))
	}
	missing, fresh := f.store.restoreAll(ctx, state.Targets, state.Channels, f.logger)
	if ctx.Err() != nil {
		return true, false
	}
	f.store.SetGauge(metrics.DocsBuilderOnDemandFollowMissing, float64(missing))
	complete := missing == 0
	// A development branch (ModulePullOverride) is never in S3: the
	// follower renders it from the registry, in its hot cache only, and
	// links the channels again so the address of the module serves it.
	linked, passing, lastingFailure := f.renderBranches(ctx, state.Targets)
	if linked {
		f.store.SetChannels(state.Channels)
	}
	// A branch the registry did not give is tried again on the next sync,
	// one that failed for a lasting reason after retryAfter.
	fresh = fresh && !passing
	f.retryAt = time.Time{}
	if lastingFailure {
		f.retryAt = f.now().Add(f.retryAfter)
	}
	if ctx.Err() != nil {
		return true, false
	}
	if err := f.store.WriteIndex(state.Targets); err != nil {
		f.logger.Warn("write search index and llms.txt", log.Err(err))
		complete = false
	}
	if err := f.store.WriteList(ctx, state.Modules, state.Targets); err != nil && ctx.Err() == nil {
		f.logger.Warn("write the list of modules", log.Err(err))
		complete = false
	}
	// A pod that becomes the leader starts from what it follows.
	if _, err := f.store.saveIndexedLocal(state); err != nil {
		f.logger.Warn("save the targets of the leader", log.Err(err))
	}

	// Applied in full only once nothing is left to replace.
	if complete && fresh {
		f.applied = bytes.Clone(raw)
	}

	return true, complete
}

// Run syncs every interval until ctx is done, while leading reports false.
// onFirst is called after the first complete sync: the pod is ready (main
// also makes it ready after a while, so a version missing from S3 does not
// keep it out). When the cold cache does not answer for FollowFallbackAfter,
// fallback runs until it answers again: the pod reads the registry itself.
func (f *Follower) Run(ctx context.Context, every time.Duration, leading func() bool, onFirst func(), fallback func(context.Context)) {
	var failingSince time.Time
	var stopFallback context.CancelFunc
	defer func() {
		if stopFallback != nil {
			stopFallback()
		}
	}()

	endFallback := func() {
		if stopFallback != nil {
			stopFallback()
			stopFallback = nil
			f.store.SetGauge(metrics.DocsBuilderOnDemandFollowFallback, 0)
			// The fallback may have served its own targets after the last
			// sync: the next one applies those of the leader again.
			f.applied = nil
		}
	}

	ready := false
	// While S3 answers without targets.json, it is looked for every
	// firstEvery, twice as long after each miss, up to every: a leader that
	// can not read the registry writes it no sooner than its next pass.
	fast := time.Duration(0)
	for {
		if leading() {
			// The leader reads the registry: no fallback beside it, and it
			// misses none of the versions it records.
			failingSince = time.Time{}
			endFallback()
			f.store.SetGauge(metrics.DocsBuilderOnDemandFollowMissing, 0)
		} else if answered, complete := f.Sync(ctx); answered {
			failingSince = time.Time{}
			switch {
			case f.found:
				fast = 0
			case fast == 0:
				fast = f.firstEvery
			default:
				fast = min(fast*2, every)
			}
			if complete && !ready {
				ready = true
				onFirst()
			}
			if stopFallback != nil {
				f.logger.Info("S3 answers again: the targets come from the leader")
			}
			endFallback()
		} else {
			// S3 does not answer: every, the warning is not repeated faster.
			fast = 0
			if failingSince.IsZero() {
				failingSince = f.now()
			}
			if stopFallback == nil && f.now().Sub(failingSince) >= FollowFallbackAfter {
				f.logger.Warn("no targets of the leader for too long: reading the registry")
				stopFallback = startFallback(ctx, fallback)
				f.store.SetGauge(metrics.DocsBuilderOnDemandFollowFallback, 1)
			}
		}

		wait := every
		if fast > 0 && !leading() {
			wait = min(every, fast)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// startFallback runs fallback until the returned function is called or ctx
// is done; the function returns once fallback has returned, so it changes
// nothing after.
func startFallback(ctx context.Context, fallback func(context.Context)) context.CancelFunc {
	fctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		fallback(fctx)
	}()

	return func() {
		cancel()
		<-done
	}
}

// renderBranches renders the development branches among the targets,
// unless the same revision is published here already. It reports that a
// branch is published (linked), so its channels are to be linked again, and
// that a branch failed for a reason that passes (the registry, the pod) or
// for one that does not (see Lasting).
func (f *Follower) renderBranches(ctx context.Context, targets []ondemand.Target) (bool, bool, bool) {
	var linked, passing, lastingFailure bool
	for _, t := range targets {
		if t.Rev == "" {
			continue
		}
		err := f.store.renderPinned(ctx, t, true)
		if f.store.published(t) {
			linked = true
		}
		if err == nil {
			continue
		}
		if ctx.Err() != nil {
			return linked, true, lastingFailure
		}
		if ondemand.Lasting(err) {
			lastingFailure = true
		} else {
			passing = true
		}
		f.logger.Warn("render a development branch", slog.String("module", t.Module), slog.String("branch", t.Version), log.Err(err))
	}

	return linked, passing, lastingFailure
}
