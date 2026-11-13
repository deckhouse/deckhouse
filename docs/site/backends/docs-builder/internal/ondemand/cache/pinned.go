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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

// resolveParallel is how many modules ResolveChannels looks up at once.
const resolveParallel = 8

// resolveTimeout bounds the list of the modules and the lookup of the version
// of one module, a few requests to the API server and the registry. A
// registry that took a request and sends nothing more fails the module, which
// keeps its last target, instead of holding the pass for good.
const resolveTimeout = 2 * time.Minute

// Resolver picks the target version of every module with a registry source
// by its policy. A module whose lookup fails with ErrNoVersions is
// unpublished.
type Resolver struct {
	Modules func(ctx context.Context) ([]registry.Module, error)
	Policy  TargetPolicy
	// Upcoming, when set (a cluster), lists the versions the cluster is about
	// to update to: they are rendered ahead, without the search.
	Upcoming func(ctx context.Context) ([]registry.Release, error)
	Logger   *log.Logger
	// timeout is resolveTimeout when zero, shorter in the tests.
	timeout time.Duration
}

// TargetPolicy picks the target of a module and the versions of its release
// channels: a cluster (ClusterPolicy) and the public site (SitePolicy) pick
// them differently.
type TargetPolicy interface {
	// Resolve returns the target of the module, nil when nothing it has
	// published can be rendered, and the versions of its release channels.
	Resolve(ctx context.Context, m registry.Module) (*ondemand.Target, ondemand.Channels, error)
}

// Resolution is what a pass of the Resolver found.
type Resolution struct {
	// Targets sorted by module.
	Targets []ondemand.Target
	// Channels are the release channels of the modules on the public site.
	// In a cluster a module has one, stable, pointing at its target: the
	// address of the module without a version serves the target.
	Channels map[string]ondemand.Channels
	// Unresolved is how many modules could not be resolved this time; such
	// a module has no channels, the caller keeps the ones it had.
	Unresolved int
	// Modules are all the modules, for the list of modules.
	Modules []ondemand.Listed
	// Unpublished are the modules with no source or no release repository
	// this time: no target, no documentation on their cards.
	Unpublished map[string]bool
}

// ResolveChannels returns the targets sorted by module, the release
// channels of the modules and the list of modules. A module whose version
// can not be found this time keeps its target from prev, so a registry
// hiccup does not drop it from the search.
func (r Resolver) ResolveChannels(ctx context.Context, prev map[string]ondemand.Target) (Resolution, error) {
	timeout := cmp.Or(r.timeout, resolveTimeout)
	lctx, cancel := context.WithTimeout(ctx, timeout)
	modules, err := r.Modules(lctx)
	cancel()
	if err != nil {
		return Resolution{}, err
	}

	// Each module costs a few requests to the API server and the registry;
	// one by one they took most of a minute for 90 modules.
	found := make([]*ondemand.Target, len(modules))
	foundChannels := make([]ondemand.Channels, len(modules))
	noDocs := make([]bool, len(modules))
	unpublished := make([]bool, len(modules))
	var failed atomic.Int32
	// The lookups report their errors themselves: Wait has none to return.
	var g errgroup.Group
	g.SetLimit(resolveParallel)
	for i, m := range modules {
		g.Go(func() error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			t, chans, err := r.Policy.Resolve(ctx, m)
			if ondemand.Unpublished(err) {
				// No source, no release repository: nothing published, as
				// with no versions. Not a failure: it stays so.
				t, chans, err = nil, nil, nil
				unpublished[i] = true
			}
			switch {
			case err != nil:
				failed.Add(1)
				r.Logger.Warn("resolve pinned version", slog.String("module", m.Name), log.Err(err))
				if p, ok := prev[m.Name]; ok {
					t = &p
				}
			case t == nil:
				noDocs[i] = true
			}
			found[i], foundChannels[i] = t, chans
			return nil
		})
	}
	_ = g.Wait()

	res := Resolution{
		Targets:    make([]ondemand.Target, 0, len(modules)),
		Channels:   make(map[string]ondemand.Channels),
		Unresolved: int(failed.Load()),
		Modules:    make([]ondemand.Listed, 0, len(modules)),

		Unpublished: make(map[string]bool),
	}
	for i, t := range found {
		if unpublished[i] {
			res.Unpublished[modules[i].Name] = true
		}
		if t != nil {
			res.Targets = append(res.Targets, *t)
		}
		if len(foundChannels[i]) > 0 {
			res.Channels[modules[i].Name] = foundChannels[i]
		}
		res.Modules = append(res.Modules, ondemand.Listed{Module: modules[i].Name, Enabled: modules[i].Enabled, Stage: modules[i].Stage, NoDocs: noDocs[i]})
	}

	slices.SortFunc(res.Targets, func(a, b ondemand.Target) int { return strings.Compare(a.Module, b.Module) })
	slices.SortFunc(res.Modules, func(a, b ondemand.Listed) int { return strings.Compare(a.Module, b.Module) })

	return res, nil
}

// SitePolicy picks the target of a module of the public site, where every
// release channel has pages of its own: the versions of all its channels,
// and the version of the most stable one for the target. A module without
// channel tags gets its newest version as stable. No module is enabled
// there: the site documents what the registries publish.
type SitePolicy struct {
	Channels func(ctx context.Context, module string) (ondemand.Channels, error)
	Latest   func(ctx context.Context, module string) (string, error)
}

// Resolve picks the versions of the release channels of the module and the
// target among them (TargetPolicy).
func (p SitePolicy) Resolve(ctx context.Context, m registry.Module) (*ondemand.Target, ondemand.Channels, error) {
	chans, err := p.Channels(ctx, m.Name)
	if errors.Is(err, ondemand.ErrNoChannel) {
		var latest string
		if latest, err = p.Latest(ctx, m.Name); err == nil && latest != "" {
			chans = ondemand.Channels{"stable": latest}
		}
	}
	if err != nil {
		return nil, nil, err
	}

	for _, channel := range ondemand.TargetChannels {
		if version, ok := chans[channel]; ok {
			return &ondemand.Target{Module: m.Name, Version: version}, chans, nil
		}
	}
	if version, ok := chans["rock-solid"]; ok {
		return &ondemand.Target{Module: m.Name, Version: version}, chans, nil
	}

	// Nothing published that docs-builder can render.
	return nil, nil, nil
}

// ClusterPolicy picks the target of a module of a cluster:
//   - an enabled module pulled by a ModulePullOverride: the branch it runs;
//   - an enabled module: the installed version;
//   - otherwise the version of the most stable release channel, and the
//     newest version when there are no channel tags (a mirror made with a
//     version filter has none).
//
// The module has one release channel, stable, pointing at its target: the
// address of the module without a version serves the target. The local
// modules of a developer's machine are picked so too: each is the branch of
// its files (local.Source).
type ClusterPolicy struct {
	ChannelVersion func(ctx context.Context, module string) (string, error)
	Latest         func(ctx context.Context, module string) (string, error)
	Logger         *log.Logger
}

// Resolve picks the target of the module (TargetPolicy).
func (p ClusterPolicy) Resolve(ctx context.Context, m registry.Module) (*ondemand.Target, ondemand.Channels, error) {
	t, err := p.target(ctx, m)
	if err != nil || t == nil {
		return nil, nil, err
	}

	return t, ondemand.Channels{"stable": t.Version}, nil
}

func (p ClusterPolicy) target(ctx context.Context, m registry.Module) (*ondemand.Target, error) {
	if m.Enabled && m.Override != nil {
		if err := docs.ValidateModuleTag(m.Name, m.Override.Tag); err == nil {
			// A branch always has a revision, so it never counts as a release:
			// the digest, or the tag until the cluster has pulled the image.
			return &ondemand.Target{Module: m.Name, Version: m.Override.Tag, Rev: cmp.Or(m.Override.Digest, m.Override.Tag)}, nil
		}
		// A tag that can not be a page path (an upper-case branch name, which
		// Hugo would lower-case): the module is shown as if it were disabled.
		p.Logger.Warn("pull override tag is not a page path, using a release version", slog.String("module", m.Name), slog.String("tag", m.Override.Tag))
	}

	if m.Enabled && docs.ValidateModuleVersion(m.Name, m.Version) == nil {
		return &ondemand.Target{Module: m.Name, Version: m.Version}, nil
	}

	version, err := p.ChannelVersion(ctx, m.Name)
	if errors.Is(err, ondemand.ErrNoChannel) {
		version, err = p.Latest(ctx, m.Name)
	}
	if err != nil {
		return nil, err
	}
	if version == "" {
		// Nothing published that docs-builder can render.
		return nil, nil
	}

	return &ondemand.Target{Module: m.Name, Version: version}, nil
}

// pinnerStore is what a Pinner needs of the Store: the pinned versions, the
// cold cache, the list of modules, the channel links, the search index, the
// metrics, and where its state is recorded.
type pinnerStore interface {
	Pin(targets, channels []ondemand.Target)
	renderPinned(ctx context.Context, t ondemand.Target, withAI bool) error
	published(t ondemand.Target) bool

	restoreAll(ctx context.Context, targets []ondemand.Target, chans map[string]ondemand.Channels, logger *log.Logger) (int, bool)

	SetTargets(targets []ondemand.Target)
	WriteList(ctx context.Context, modules []ondemand.Listed, targets []ondemand.Target) error
	SetChannels(chans map[string]ondemand.Channels)

	WriteIndex(targets []ondemand.Target) error

	SetGauge(name string, v float64)
	initPinSuccessMetric(at time.Time)

	recordStore
}

// recordStore is the Store as a stateRecord keeps the state of a Pinner in
// it: targets.json, what the index was built from, and the archives of the
// versions it names.
type recordStore interface {
	loadIndexed(ctx context.Context) (indexedState, error)
	saveIndexed(ctx context.Context, state indexedState) error
	saveIndexedLocal(state indexedState) (string, error)
	uploadMissing(ctx context.Context, pinned []ondemand.Target) error
}

// Pinner keeps the target versions rendered and pinned in the store and the
// search index and llms.txt built from them.
type Pinner struct {
	resolver Resolver
	store    pinnerStore
	// record is where the state is recorded for the next pods.
	record     stateRecord
	retryAfter time.Duration
	logger     *log.Logger
	now        func() time.Time

	targets map[string]ondemand.Target
	// indexed is the target of every module the index was last built from;
	// the first sync reads it from the volume, so a restart keeps it.
	indexed     map[string]ondemand.Target
	indexedRead bool
	// channels are the versions of the release channels of every module on
	// the public site; a module the registry does not answer for keeps its
	// last ones.
	channels map[string]ondemand.Channels
	// saved is what saveIndexed recorded last, so an unchanged pass does not
	// upload it again.
	saved []ondemand.Target
	// savedChannels is the JSON of the channels saveIndexed recorded last.
	savedChannels string

	// OnRestored, when set, is called once the first sync has restored from
	// the cold cache what the index was last built from: the pod is ready.
	// A cold cache that does not answer does not hold it back.
	OnRestored func()
	ready      bool
	// Watch, when set, sees every pass, for the liveness probe.
	Watch *PassWatch
	// passTimeout is the const, shorter in the tests.
	passTimeout time.Duration
	// failed keeps a target that does not exist, has no documentation, fails
	// the Hugo build or the AI export from being downloaded again on every
	// sync: the images of a version do not change. Other failures are not
	// kept and are tried on the next sync.
	failed map[ondemand.Target]time.Time
	// upcoming are the versions the cluster is about to update to, as of the
	// last pass; warmed are those of them rendered once.
	upcoming []ondemand.Target
	warmed   map[ondemand.Target]bool
	// listed are the modules of the last pass for the list of modules;
	// noDocs are the targets whose versions have no documentation pages.
	listed []ondemand.Listed
	noDocs map[ondemand.Target]bool
	// unpublished are the modules of the last pass with no source or no
	// release repository.
	unpublished map[string]bool
	// savedModules is the JSON of the modules saveIndexed recorded last.
	savedModules string
	// served are the channels as the last pass linked them: a channel keeps
	// its version there until the new one is published, over many passes.
	served map[string]ondemand.Channels
	// progress shows readers what a pass has published so far, at most
	// every progressEvery; set while a pass renders.
	progress      func()
	progressAt    time.Time
	progressEvery time.Duration
	// kick starts the next pass ahead of time (Kick).
	kick chan struct{}
}

// progressEvery is how often a pass shows what it published: the list of
// modules gets the titles and the descriptions, the menus the channels. A
// first pass on an empty cache renders every module for minutes.
const progressEvery = 10 * time.Second

// passTimeout bounds a pass. Its steps are bounded on their own (a render by
// RenderTimeout, a lookup by resolveTimeout, a request to S3 by its timeout),
// but they add up: a registry answering every request just in time would
// hold the pass for hours. A pass cut short is not lost: the next one starts
// at once and skips what is published.
const passTimeout = time.Hour

// NewPinner renders a target that has no documentation again no sooner than
// retryAfter, or when the module moves to another target.
func NewPinner(resolver Resolver, store pinnerStore, retryAfter time.Duration, logger *log.Logger) *Pinner {
	return &Pinner{
		resolver:   resolver,
		store:      store,
		record:     coldRecord{store},
		retryAfter: retryAfter,
		logger:     logger,
		now:        time.Now,
		targets:    make(map[string]ondemand.Target),
		indexed:    make(map[string]ondemand.Target),
		channels:   make(map[string]ondemand.Channels),
		failed:     make(map[ondemand.Target]time.Time),
		warmed:     make(map[ondemand.Target]bool),
		noDocs:     make(map[ondemand.Target]bool),

		progressEvery: progressEvery,
		passTimeout:   passTimeout,
		kick:          make(chan struct{}, 1),
	}
}

// NewFallbackPinner is NewPinner for a follower that reads the registry
// itself while S3 does not answer: it keeps targets.json on the volume, so it
// never replaces the file of the leader in S3.
func NewFallbackPinner(resolver Resolver, store pinnerStore, retryAfter time.Duration, logger *log.Logger) *Pinner {
	p := NewPinner(resolver, store, retryAfter, logger)
	p.record = volumeRecord{store}

	return p
}

// stateRecord is where a Pinner records its state for the next pods: what
// the index was built from (targets.json) and the archives of the versions
// it names.
type stateRecord interface {
	// load returns what was recorded last; an error means it can not be
	// read, and nothing is recorded until it is.
	load(ctx context.Context) (indexedState, error)
	save(ctx context.Context, state indexedState) error
	// uploadMissing uploads the archives of the pinned versions the cold
	// cache lacks.
	uploadMissing(ctx context.Context, pinned []ondemand.Target) error
}

// coldRecord records the state on the volume and in S3, for the followers
// and the next pods: the leader.
type coldRecord struct{ store recordStore }

func (r coldRecord) load(ctx context.Context) (indexedState, error) {
	return r.store.loadIndexed(ctx)
}

func (r coldRecord) save(ctx context.Context, state indexedState) error {
	return r.store.saveIndexed(ctx, state)
}

func (r coldRecord) uploadMissing(ctx context.Context, pinned []ondemand.Target) error {
	return r.store.uploadMissing(ctx, pinned)
}

// volumeRecord records the state on the volume only: a follower reading the
// registry itself. It never records over the file of the leader, so a file
// it can not read does not hold it back, and it uploads nothing.
type volumeRecord struct{ store recordStore }

func (r volumeRecord) load(ctx context.Context) (indexedState, error) {
	state, _ := r.store.loadIndexed(ctx)

	return state, nil
}

func (r volumeRecord) save(_ context.Context, state indexedState) error {
	_, err := r.store.saveIndexedLocal(state)

	return err
}

func (volumeRecord) uploadMissing(context.Context, []ondemand.Target) error { return nil }

// Run syncs the targets right away and then periodically, or when kicked,
// until ctx is done.
func (p *Pinner) Run(ctx context.Context, every time.Duration) {
	p.store.initPinSuccessMetric(p.now())
	t := time.NewTicker(every)
	defer t.Stop()

	for {
		p.pass(ctx)

		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-p.kick:
		}
	}
}

// pass syncs the targets and renders the versions to update to, within
// passTimeout.
func (p *Pinner) pass(ctx context.Context) {
	pctx, cancel := context.WithTimeout(ctx, p.passTimeout)
	defer cancel()
	if p.Watch != nil {
		p.Watch.start(p, p.passTimeout)
		defer p.Watch.done(p)
	}

	p.sync(pctx)
	p.warmUp(pctx)
	if pctx.Err() != nil && ctx.Err() == nil {
		p.logger.Warn("the pass of the pinned versions ran out of time, the next one goes on", slog.Duration("timeout", p.passTimeout))
	}
}

// Kick starts a pass once the running one, if any, is over: the files of a
// local module changed (local.Source). Kicks during a pass make one more.
func (p *Pinner) Kick() {
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

func (p *Pinner) sync(ctx context.Context) {
	if !p.indexedRead {
		p.readIndexed(ctx)
		if !p.ready && p.OnRestored != nil && ctx.Err() == nil {
			p.ready = true
			p.OnRestored()
		}
	}

	// A module the registry does not answer for keeps its last target: the
	// one of the last pass, else the one the index was built from (right
	// after a restart, or once targets.json is read after a failed read).
	prev := maps.Clone(p.indexed)
	maps.Copy(prev, p.targets)
	// The channels as they are linked now: a channel keeps its version
	// until the new one is published.
	linked := p.served
	if linked == nil {
		// The first pass after a restart: the channels of the published
		// versions.
		linked = p.servedChannels(nil)
	}
	p.uploadMissing(ctx, linked)
	res, err := p.resolver.ResolveChannels(ctx, prev)
	if err != nil {
		p.logger.Warn("list modules for the search index", log.Err(err))
		return
	}
	targets, resolved, unresolved := p.keepUnpublishedOnce(res, prev), res.Channels, res.Unresolved
	p.listed = res.Modules
	// Before the list is written: its cards tell the new targets.
	p.targets = make(map[string]ondemand.Target, len(targets))
	for _, t := range targets {
		p.targets[t.Module] = t
	}
	p.store.SetTargets(targets)
	// Every module is on the list right away, before its target renders,
	// and so on the followers: targets.json gets the modules now.
	cards := p.cardTargets(targets)
	p.save(ctx, p.publishedOf(cards), linked, p.writeList(ctx, cards))

	for module := range p.channels {
		if _, ok := p.targets[module]; !ok {
			delete(p.channels, module)
		}
	}
	for module, chans := range resolved {
		p.channels[module] = chans
	}
	chanTargets := channelTargets(p.targets, p.channels)
	p.upcoming = p.upcomingTargets(ctx, chanTargets)
	// A failure is kept while the version is still wanted, so a version
	// without documentation waits for retryAfter.
	for t := range p.failed {
		if p.targets[t.Module] != t && !slices.Contains(chanTargets, t) && !slices.Contains(p.upcoming, t) {
			delete(p.failed, t)
			delete(p.noDocs, t)
		}
	}
	// Until the new target of a module is published, the one in the index
	// stays pinned: the module must not leave the search meanwhile.
	keep := slices.Clone(targets)
	for _, t := range targets {
		if old, ok := p.indexed[t.Module]; ok && old != t {
			keep = append(keep, old)
		}
	}
	p.store.Pin(keep, slices.Concat(chanTargets, channelTargets(ondemand.ByModule(targets), linked)))

	// One by one: a reader opening another version gets the other render slot.
	// Development branches go last: they are rendered from the registry every
	// time, while release versions mostly come from the cold cache. The
	// versions of the channels go after the search.
	releases := slices.DeleteFunc(slices.Clone(targets), func(t ondemand.Target) bool { return t.Rev != "" })
	branches := slices.DeleteFunc(slices.Clone(targets), func(t ondemand.Target) bool { return t.Rev == "" })
	p.progress = func() {
		if p.now().Sub(p.progressAt) < p.progressEvery {
			return
		}
		p.progressAt = p.now()
		served := p.servedChannels(linked)
		p.store.SetChannels(served)
		cards := p.cardTargets(targets)
		// The followers take what is published from targets.json too.
		p.save(ctx, p.publishedOf(cards), served, p.writeList(ctx, cards))
	}
	rendered := p.renderEach(ctx, releases, true)
	rendered = p.renderEach(ctx, branches, true) && rendered
	rendered = p.renderEach(ctx, chanTargets, false) && rendered
	p.progress = nil
	if ctx.Err() != nil {
		return
	}

	index := p.cardTargets(targets)
	p.indexed = ondemand.ByModule(index)
	served := p.servedChannels(linked)
	p.served = served
	p.store.SetChannels(served)
	p.store.Pin(index, channelTargets(p.indexed, served))

	if err := p.store.WriteIndex(index); err != nil {
		p.logger.Warn("write search index and llms.txt", log.Err(err))
		return
	}
	modules := p.writeList(ctx, index)
	saved := p.publishedOf(index)
	// Updated only by a pass that resolved every module and published every
	// target, those that can not be aside (renderEach): a registry that does
	// not answer keeps the alert on it going. A pass that publishes nothing
	// counts only when there is nothing to publish (nothingToPublish): the
	// metric is there from the start (initPinSuccessMetric).
	if unresolved == 0 && rendered && (len(saved) > 0 || p.nothingToPublish(targets)) {
		p.store.SetGauge(metrics.DocsBuilderOnDemandPinSuccessSeconds, float64(p.now().Unix()))
	}

	p.save(ctx, saved, served, modules)
}

// uploadMissing uploads to the cold cache what the last pass recorded in
// targets.json, the targets and the versions of the channels as linked, and
// the cold cache lacks (Store.uploadMissing). It runs before the pass renders
// anything: the uploads of the last pass are over by then, and one still
// running is only uploaded twice. A follower reading the registry itself
// records nothing in the cold cache (volumeRecord).
func (p *Pinner) uploadMissing(ctx context.Context, linked map[string]ondemand.Channels) {
	pinned := slices.Concat(slices.Collect(maps.Values(p.indexed)), channelTargets(p.indexed, linked))
	if err := p.record.uploadMissing(ctx, pinned); err != nil && ctx.Err() == nil && !ondemand.ColdMissing(err) {
		p.logger.Warn("upload the versions missing from S3", log.Err(err))
	}
}

// keepUnpublishedOnce returns the targets with the previous target of a
// module unpublished for the first pass in a row, its card not marked: a
// registry uploading the module anew or a Module made again answers "not
// found" for a while. Unpublished on the next pass too, it goes.
func (p *Pinner) keepUnpublishedOnce(res Resolution, prev map[string]ondemand.Target) []ondemand.Target {
	targets := res.Targets
	for module := range res.Unpublished {
		t, ok := prev[module]
		if !ok || p.unpublished[module] {
			continue
		}
		targets = append(targets, t)
		for i := range res.Modules {
			if res.Modules[i].Module == module {
				res.Modules[i].NoDocs = false
			}
		}
	}
	p.unpublished = res.Unpublished
	slices.SortFunc(targets, func(a, b ondemand.Target) int { return strings.Compare(a.Module, b.Module) })

	return targets
}

// readIndexed takes what the index was last built from and restores it. A
// cold cache that does not answer leaves it unread: recorded over, the file
// would lose the targets of the followers, so nothing is saved and the next
// pass reads it again. What a pass found meanwhile is newer and stays.
func (p *Pinner) readIndexed(ctx context.Context) {
	state, err := p.record.load(ctx)
	if err != nil {
		p.logger.Error("read indexed targets, nothing is saved until they are read", log.Err(err))
		return
	}
	for _, t := range state.Targets {
		if _, ok := p.indexed[t.Module]; !ok {
			p.indexed[t.Module] = t
		}
	}
	for module, chans := range state.Channels {
		if _, ok := p.channels[module]; !ok {
			p.channels[module] = chans
		}
	}
	if p.listed == nil {
		p.listed = state.Modules
	}
	p.indexedRead = true
	p.restore(ctx)
}

// publishedOf are the targets that are published: only they are worth
// restoring after a restart.
func (p *Pinner) publishedOf(targets []ondemand.Target) []ondemand.Target {
	out := make([]ondemand.Target, 0, len(targets))
	for _, t := range targets {
		if p.store.published(t) {
			out = append(out, t)
		}
	}

	return out
}

// nothingToPublish reports a pass with nothing to publish: no module from a
// registry, or modules without documentation only. A module with no source
// or no release repository is not taken for one without documentation here:
// a wrong path to the registry turns every module so, and the second pass
// takes them all off the site (keepUnpublishedOnce). Nor is a target failing
// for another lasting reason, such as the Hugo build.
func (p *Pinner) nothingToPublish(targets []ondemand.Target) bool {
	return len(p.unpublished) == 0 && !slices.ContainsFunc(targets, func(t ondemand.Target) bool { return !p.noDocs[t] })
}

// save records the state in targets.json unless it is what was recorded
// last.
func (p *Pinner) save(ctx context.Context, saved []ondemand.Target, chans map[string]ondemand.Channels, modules []ondemand.Listed) {
	if !p.indexedRead {
		return
	}
	chansJSON, _ := json.Marshal(chans)
	modulesJSON, _ := json.Marshal(modules)
	if slices.Equal(saved, p.saved) && string(chansJSON) == p.savedChannels && string(modulesJSON) == p.savedModules {
		return
	}
	state := indexedState{Targets: saved, Channels: chans, Modules: modules}
	if err := p.record.save(ctx, state); err != nil {
		// Not recorded as saved: the next pass uploads it again, so the
		// followers get it once S3 is ready.
		if !errors.Is(err, errNotUploaded) {
			p.logger.Warn("save indexed targets", log.Err(err))
		}
		return
	}
	p.saved, p.savedChannels, p.savedModules = saved, string(chansJSON), string(modulesJSON)
}

// cardTargets are the targets as they are served: until the new target of
// a module is published, the one the index was built from.
func (p *Pinner) cardTargets(targets []ondemand.Target) []ondemand.Target {
	out := make([]ondemand.Target, 0, len(targets))
	for _, t := range targets {
		if !p.store.published(t) {
			if old, ok := p.indexed[t.Module]; ok && p.store.published(old) {
				t = old
			}
		}
		out = append(out, t)
	}

	return out
}

// writeList builds the page of the list of modules from the modules of the
// last pass and returns them as targets.json keeps them. A card showing a
// published version is neither without documentation nor failed, whatever
// its new target is.
func (p *Pinner) writeList(ctx context.Context, targets []ondemand.Target) []ondemand.Listed {
	cards := ondemand.ByModule(targets)
	modules := make([]ondemand.Listed, 0, len(p.listed))
	for _, m := range p.listed {
		t, ok := p.targets[m.Module]
		if card, shown := cards[m.Module]; ok && (!shown || !p.store.published(card)) {
			if p.noDocs[t] {
				m.NoDocs = true
			} else if p.failedNow(t) {
				m.Failed = true
			}
		}
		modules = append(modules, m)
	}
	if err := p.store.WriteList(ctx, modules, targets); err != nil && ctx.Err() == nil {
		p.logger.Warn("write the list of modules", log.Err(err))
	}

	return modules
}

// upcomingTargets are the versions the cluster is about to update to that
// are neither targets nor versions of channels.
func (p *Pinner) upcomingTargets(ctx context.Context, chanTargets []ondemand.Target) []ondemand.Target {
	if p.resolver.Upcoming == nil {
		return nil
	}
	releases, err := p.resolver.Upcoming(ctx)
	if err != nil {
		// Not known this time: the last ones stay, with what was rendered
		// and what failed.
		p.logger.Warn("list the versions to update to", log.Err(err))
		return p.upcoming
	}

	// A release of a module off the list, e.g. one Deckhouse runs the
	// embedded copy of, is not offered.
	listed := make(map[string]bool, len(p.listed))
	for _, m := range p.listed {
		listed[m.Module] = true
	}
	var out []ondemand.Target
	for _, r := range releases {
		t := ondemand.Target{Module: r.Module, Version: r.Version}
		if !listed[t.Module] || docs.ValidateModuleVersion(t.Module, t.Version) != nil || p.targets[t.Module] == t || slices.Contains(chanTargets, t) || slices.Contains(out, t) {
			continue
		}
		out = append(out, t)
	}
	slices.SortFunc(out, compareTargets)

	return out
}

// warmUp renders the versions the cluster is about to update to, after the
// pass, so they open at once. They are not pinned: a version rendered once
// is not rendered again after an eviction, a reader gets it on demand.
func (p *Pinner) warmUp(ctx context.Context) {
	for t := range p.warmed {
		if !slices.Contains(p.upcoming, t) {
			delete(p.warmed, t)
		}
	}
	for _, t := range p.upcoming {
		if p.warmed[t] {
			continue
		}
		if p.renderEach(ctx, []ondemand.Target{t}, false) && !p.failedNow(t) {
			p.warmed[t] = true
		}
		if ctx.Err() != nil {
			return
		}
	}
}

// failedNow reports a version that failed for a reason that does not change
// and waits for retryAfter.
func (p *Pinner) failedNow(t ondemand.Target) bool {
	at, ok := p.failed[t]

	return ok && p.now().Sub(at) < p.retryAfter
}

func compareTargets(a, b ondemand.Target) int {
	return strings.Compare(a.Module+"/"+a.Version, b.Module+"/"+b.Version)
}

// channelTargets are the versions release channels point at besides the
// targets, one per version: they get pages of their own, not the search.
func channelTargets(targets map[string]ondemand.Target, channels map[string]ondemand.Channels) []ondemand.Target {
	var out []ondemand.Target
	for module, chans := range channels {
		for _, version := range chans {
			t := ondemand.Target{Module: module, Version: version}
			// The target itself is rendered as one: in a cluster its stable
			// channel is the target, a development branch included.
			if targets[module].Version != version && !slices.Contains(out, t) {
				out = append(out, t)
			}
		}
	}
	slices.SortFunc(out, compareTargets)

	return out
}

// renderEach renders the targets one by one, with the AI export or
// without. It reports whether every target is published, those that can not
// be (no documentation, Hugo failing on it) aside: a failure of the
// registry or of the pod is not.
func (p *Pinner) renderEach(ctx context.Context, targets []ondemand.Target, withAI bool) bool {
	ok := true
	for _, t := range targets {
		if p.failedNow(t) {
			continue
		}
		if err := p.store.renderPinned(ctx, t, withAI); err != nil {
			if ctx.Err() != nil {
				return false
			}
			// What does not change until the template or the image does:
			// waits for retryAfter and does not hold the update back.
			if ondemand.Lasting(err) {
				p.failed[t] = p.now()
				if ondemand.KindOf(err) == ondemand.NoDocs {
					p.noDocs[t] = true
				}
			} else {
				ok = false
			}
			p.logger.Warn("render pinned version", slog.String("module", t.Module), slog.String("version", t.Version), slog.Bool("search", withAI), log.Err(err))
			continue
		}
		delete(p.failed, t)
		delete(p.noDocs, t)
		if p.progress != nil {
			p.progress()
		}
	}

	return ok
}

// servedChannels are the channels as they are served after a pass: the new
// version of a channel once it is published, the one it had before
// (linked) until then. With no linked it is the channels whose versions are
// published.
func (p *Pinner) servedChannels(linked map[string]ondemand.Channels) map[string]ondemand.Channels {
	out := make(map[string]ondemand.Channels)
	for module, chans := range p.channels {
		for channel, version := range chans {
			if !p.store.published(ondemand.Target{Module: module, Version: version}) {
				old, ok := linked[module][channel]
				if !ok || !p.store.published(ondemand.Target{Module: module, Version: old}) {
					continue
				}
				version = old
			}
			if out[module] == nil {
				out[module] = make(ondemand.Channels)
			}
			out[module][channel] = version
		}
	}

	return out
}

// restore publishes the release versions the index was last built from and
// writes the index, before the registry is asked what changed: after a
// restart they come from the cold cache in seconds, so the search does not
// wait for a whole pass. Nothing is rendered here: development branches and
// versions missing from the cold cache are left to the pass.
func (p *Pinner) restore(ctx context.Context) {
	if len(p.indexed) == 0 && len(p.channels) == 0 && len(p.listed) == 0 {
		return
	}
	targets := make([]ondemand.Target, 0, len(p.indexed))
	for _, t := range p.indexed {
		targets = append(targets, t)
	}
	slices.SortFunc(targets, func(a, b ondemand.Target) int { return strings.Compare(a.Module, b.Module) })

	// Every module is on the list at once, by its name until its version
	// comes from S3; nothing recorded yet (a new volume and bucket) waits
	// for the pass instead of showing no modules.
	p.store.SetTargets(targets)
	if len(p.listed) > 0 || len(targets) > 0 {
		p.writeList(ctx, targets)
	}

	p.store.restoreAll(ctx, targets, p.channels, p.logger)
	if ctx.Err() != nil {
		return
	}

	if err := p.store.WriteIndex(targets); err != nil {
		p.logger.Warn("write search index and llms.txt", log.Err(err))
	}
	if len(p.listed) > 0 || len(targets) > 0 {
		p.writeList(ctx, targets)
	}
}
