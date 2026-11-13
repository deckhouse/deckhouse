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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/registry"
)

// Target is the version of a module that represents it in the search index
// and llms.txt. The JSON names are those of targets.json; decoding ignores their case, so
// files written before the tags still read.
type Target struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	// Rev changes when the same version name gets new content: the image
	// digest of a development branch. Empty for a release version.
	Rev string `json:"rev,omitempty"`
}

// errNoChannel means no source of the module has a release channel tag.
var errNoChannel = errors.New("module has no release channel tags")

// errChannelsGone means no source of the module has a release channel tag,
// though the last lookup found some: a release repository uploaded anew
// answers "not found" for a while, and the newest version a module without
// channels gets may be a pre-release. Like a registry failure, it keeps the
// last target and channels of the module for the pass; gone on the next
// lookup too, the tags are errNoChannel.
var errChannelsGone = errors.New("release channel tags found last time are gone")

// channels are tried in this order for a module that is not enabled: the
// most stable one first. rock-solid is left out, as on the module pages.
var channels = []string{"stable", "early-access", "beta", "alpha"}

// siteChannels are the release channels with pages of their own on the
// public site, /modules/<module>/<channel>/.
var siteChannels = []string{"alpha", "beta", "early-access", "stable", "rock-solid"}

// Channels maps the release channels of a module to the versions they point
// at.
type Channels map[string]string

// resolveParallel is how many modules ResolveChannels looks up at once.
const resolveParallel = 8

// resolveTimeout bounds the list of the modules and the lookup of the version
// of one module, a few requests to the API server and the registry. A
// registry that took a request and sends nothing more fails the module, which
// keeps its last target, instead of holding the pass for good. A variable for
// the tests.
var resolveTimeout = 2 * time.Minute

// Resolver picks the target version of every module with a registry source:
//   - an enabled module pulled by a ModulePullOverride: the branch it runs;
//   - an enabled module: the installed version;
//   - otherwise the version of the most stable release channel, and the
//     newest version when there are no channel tags (a mirror made with a
//     version filter has none).
//
// With Channels set (the public site) a module that is not enabled also gets
// the versions of all its release channels, and the target is the version of
// the most stable one; a module without channel tags gets its newest version
// as stable.
type Resolver struct {
	Modules        func(ctx context.Context) ([]registry.Module, error)
	ChannelVersion func(ctx context.Context, module string) (string, error)
	Channels       func(ctx context.Context, module string) (Channels, error)
	Latest         func(ctx context.Context, module string) (string, error)
	// Upcoming, when set (a cluster), lists the versions the cluster is about
	// to update to: they are rendered ahead, without the search.
	Upcoming func(ctx context.Context) ([]registry.Release, error)
	Logger   *log.Logger
}

// Resolution is what a pass of the Resolver found.
type Resolution struct {
	// Targets sorted by module.
	Targets []Target
	// Channels are the release channels of the modules on the public site.
	// In a cluster a module has one, stable, pointing at its target: the
	// address of the module without a version serves the target.
	Channels map[string]Channels
	// Unresolved is how many modules could not be resolved this time; such
	// a module has no channels, the caller keeps the ones it had.
	Unresolved int
	// Modules are all the modules, for the list of modules.
	Modules []Listed
	// Unpublished are the modules with no source or no release repository
	// this time: no target, no documentation on their cards.
	Unpublished map[string]bool
}

// ResolveChannels returns the targets sorted by module, the release
// channels of the modules and the list of modules. A module whose version
// can not be found this time keeps its target from prev, so a registry
// hiccup does not drop it from the search.
func (r Resolver) ResolveChannels(ctx context.Context, prev map[string]Target) (Resolution, error) {
	lctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	modules, err := r.Modules(lctx)
	cancel()
	if err != nil {
		return Resolution{}, err
	}

	// Each module costs a few requests to the API server and the registry;
	// one by one they took most of a minute for 90 modules.
	found := make([]*Target, len(modules))
	foundChannels := make([]Channels, len(modules))
	noDocs := make([]bool, len(modules))
	unpublished := make([]bool, len(modules))
	var failed atomic.Int32
	// The lookups report their errors themselves: Wait has none to return.
	var g errgroup.Group
	g.SetLimit(resolveParallel)
	for i, m := range modules {
		g.Go(func() error {
			ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
			defer cancel()
			t, chans, err := r.resolve(ctx, m)
			if Unpublished(err) {
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
			case r.Channels == nil:
				chans = Channels{"stable": t.Version}
			}
			found[i], foundChannels[i] = t, chans
			return nil
		})
	}
	_ = g.Wait()

	res := Resolution{
		Targets:    make([]Target, 0, len(modules)),
		Channels:   make(map[string]Channels),
		Unresolved: int(failed.Load()),
		Modules:    make([]Listed, 0, len(modules)),

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
		res.Modules = append(res.Modules, Listed{Module: modules[i].Name, Enabled: modules[i].Enabled, Stage: modules[i].Stage, NoDocs: noDocs[i]})
	}

	slices.SortFunc(res.Targets, func(a, b Target) int { return strings.Compare(a.Module, b.Module) })
	slices.SortFunc(res.Modules, func(a, b Listed) int { return strings.Compare(a.Module, b.Module) })

	return res, nil
}

func (r Resolver) resolve(ctx context.Context, m registry.Module) (*Target, Channels, error) {
	if r.Channels != nil && !m.Enabled {
		return r.resolveChannels(ctx, m.Name)
	}
	t, err := r.resolveTarget(ctx, m)

	return t, nil, err
}

// resolveChannels picks the versions of the release channels of the module
// and the target among them.
func (r Resolver) resolveChannels(ctx context.Context, module string) (*Target, Channels, error) {
	chans, err := r.Channels(ctx, module)
	if errors.Is(err, errNoChannel) {
		var latest string
		if latest, err = r.Latest(ctx, module); err == nil && latest != "" {
			chans = Channels{"stable": latest}
		}
	}
	if err != nil {
		return nil, nil, err
	}

	for _, channel := range channels {
		if version, ok := chans[channel]; ok {
			return &Target{Module: module, Version: version}, chans, nil
		}
	}
	if version, ok := chans["rock-solid"]; ok {
		return &Target{Module: module, Version: version}, chans, nil
	}

	// Nothing published that docs-builder can render.
	return nil, nil, nil
}

func (r Resolver) resolveTarget(ctx context.Context, m registry.Module) (*Target, error) {
	if m.Enabled && m.Override != nil {
		if err := docs.ValidateModuleTag(m.Name, m.Override.Tag); err == nil {
			// A branch always has a revision, so it never counts as a release:
			// the digest, or the tag until the cluster has pulled the image.
			return &Target{Module: m.Name, Version: m.Override.Tag, Rev: cmp.Or(m.Override.Digest, m.Override.Tag)}, nil
		}
		// A tag that can not be a page path (an upper-case branch name, which
		// Hugo would lower-case): the module is shown as if it were disabled.
		r.Logger.Warn("pull override tag is not a page path, using a release version", slog.String("module", m.Name), slog.String("tag", m.Override.Tag))
	}

	if m.Enabled && docs.ValidateModuleVersion(m.Name, m.Version) == nil {
		return &Target{Module: m.Name, Version: m.Version}, nil
	}

	version, err := r.ChannelVersion(ctx, m.Name)
	if errors.Is(err, errNoChannel) {
		version, err = r.Latest(ctx, m.Name)
	}
	if err != nil {
		return nil, err
	}
	if version == "" {
		// Nothing published that docs-builder can render.
		return nil, nil
	}

	return &Target{Module: m.Name, Version: version}, nil
}

// RegistryChannelVersion reads the version a release channel tag points to,
// trying the sources of the module in order and the channels from the most
// stable one. A channel pointing at something that is not a version, or at a
// release image without version.json, is skipped; with no usable channel the
// error is errNoChannel, errChannelsGone right after a lookup that found one.
// The version comes from version.json of the release image; the last one of
// every source, module and channel is kept with its image digest, so an
// unchanged channel costs one HEAD request.
func RegistryChannelVersion(sources ModuleSources, clients *registry.Clients) func(ctx context.Context, module string) (string, error) {
	releaseVersion := releaseVersionCache()
	// The modules whose last lookup found a channel (noChannel).
	var tagged sync.Map

	return func(ctx context.Context, module string) (string, error) {
		srcs, err := sources.ForModule(ctx, module)
		if err != nil {
			return "", fmt.Errorf("find sources: %w", err)
		}

		var errs []error
		for _, src := range srcs {
			client, err := clients.For(src)
			if err != nil {
				errs = append(errs, fmt.Errorf("source %s: %w", src.Name, err))
				continue
			}

			for _, channel := range channels {
				version, err := releaseVersion(ctx, client, src.Name, module, channel)
				if isNotFound(err) {
					continue
				}
				if err != nil {
					// The next source may have it.
					errs = append(errs, fmt.Errorf("source %s, channel %s: %w", src.Name, channel, err))
					break
				}
				// A development registry may point a channel at a build of a
				// branch ("mr10", "0.0.1"): not a version, try the next channel.
				if docs.ValidateModuleVersion(module, version) != nil {
					continue
				}
				tagged.Store(module, true)
				return version, nil
			}
		}

		if len(errs) > 0 {
			return "", errors.Join(errs...)
		}

		return "", noChannel(&tagged, module)
	}
}

// RegistryChannels reads the versions of all the release channels of the
// public site from the first source that has any of them. A channel pointing
// at something that is not a version is left out; with none left the error
// is errNoChannel, errChannelsGone right after a lookup that found some. A
// source failing on a channel is passed over for the next one, so the
// channels never mix two sources.
func RegistryChannels(sources ModuleSources, clients *registry.Clients) func(ctx context.Context, module string) (Channels, error) {
	releaseVersion := releaseVersionCache()
	// The modules whose last lookup found channels (noChannel).
	var tagged sync.Map

	return func(ctx context.Context, module string) (Channels, error) {
		srcs, err := sources.ForModule(ctx, module)
		if err != nil {
			return nil, fmt.Errorf("find sources: %w", err)
		}

		var errs []error
	sources:
		for _, src := range srcs {
			client, err := clients.For(src)
			if err != nil {
				errs = append(errs, fmt.Errorf("source %s: %w", src.Name, err))
				continue
			}

			found := make(Channels)
			for _, channel := range siteChannels {
				version, err := releaseVersion(ctx, client, src.Name, module, channel)
				if isNotFound(err) {
					continue
				}
				if err != nil {
					errs = append(errs, fmt.Errorf("source %s, channel %s: %w", src.Name, channel, err))
					continue sources
				}
				if docs.ValidateModuleVersion(module, version) == nil {
					found[channel] = version
				}
			}
			if len(found) > 0 {
				tagged.Store(module, true)
				return found, nil
			}
		}

		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}

		return nil, noChannel(&tagged, module)
	}
}

// noChannel is the error of a lookup that found no release channel of the
// module: errChannelsGone if the last lookup found one (tagged), errNoChannel
// otherwise. A lookup that failed is not recorded, so it changes nothing.
func noChannel(tagged *sync.Map, module string) error {
	if _, ok := tagged.LoadAndDelete(module); ok {
		return errChannelsGone
	}

	return errNoChannel
}

// releaseVersionCache reads the version a release channel of the module
// points to, from version.json of its release image, and keeps the last one
// of every source, module and channel with the image digest. An unchanged
// channel costs one HEAD request. A moved one is read by its digest, so the
// tag moving again meanwhile does not give the version of another image. A
// registry that does not answer HEAD with the digest is read by the tag, as
// before; a missing channel is an error isNotFound reports.
func releaseVersionCache() func(ctx context.Context, client *registry.Client, source, module, channel string) (string, error) {
	type cached struct{ digest, version string }
	var mu sync.Mutex
	cache := make(map[string]cached)

	return func(ctx context.Context, client *registry.Client, source, module, channel string) (string, error) {
		k := source + "/" + module + "/" + channel
		digest, err := client.ReleaseDigest(ctx, module, channel)
		if isNotFound(err) {
			return "", err
		}

		var img v1.Image
		switch {
		case err == nil:
			mu.Lock()
			c, ok := cache[k]
			mu.Unlock()
			if ok && c.digest == digest {
				return c.version, nil
			}
			img, err = client.ReleaseImageAt(ctx, module, digest)
			// The channel exists: the image went away after the tag moved
			// once more. Not wrapped, so it is not taken for a missing
			// channel; the next pass reads the channel again.
			if isNotFound(err) {
				err = fmt.Errorf("image %s of the channel %s is gone: %v", digest, channel, err) //nolint:errorlint // see above
			}
		case headUnsupported(ctx, err):
			img, err = client.ReleaseImage(ctx, module, channel)
		}
		if err != nil {
			return "", err
		}

		// The digest HEAD gave is kept: for an index it is the digest of the
		// index, not of the image of the platform img is.
		if digest == "" {
			d, err := img.Digest()
			if err != nil {
				return "", fmt.Errorf("digest: %w", err)
			}
			digest = d.String()
		}

		version, err := registry.ReleaseVersion(img)
		// An image without version.json stays so; it is no version.
		if errors.Is(err, registry.ErrNoVersion) {
			version, err = "", nil
		}
		if err != nil {
			return "", err
		}

		mu.Lock()
		cache[k] = cached{digest: digest, version: version}
		mu.Unlock()

		return version, nil
	}
}

// headUnsupported tells a registry that answered HEAD but not as the API
// asks (no Docker-Content-Digest, or no HEAD at all): its channels are read
// by the tag. A registry that did not answer, or a request given up, is
// asked no more.
func headUnsupported(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var terr *transport.Error
	if errors.As(err, &terr) {
		return terr.StatusCode == http.StatusMethodNotAllowed || terr.StatusCode == http.StatusNotImplemented
	}
	var nerr net.Error

	return !errors.As(err, &nerr)
}

// LatestVersion returns the newest published version of the module, empty
// when there is none.
func LatestVersion(versions *Versions) func(ctx context.Context, module string) (string, error) {
	return func(ctx context.Context, module string) (string, error) {
		list, err := versions.List(ctx, module)
		if err != nil || len(list) == 0 {
			return "", err
		}

		return list[0], nil
	}
}

// Pinner keeps the target versions rendered and pinned in the store and the
// search index and llms.txt built from them.
type Pinner struct {
	resolver   Resolver
	store      *Store
	retryAfter time.Duration
	logger     *log.Logger
	now        func() time.Time

	targets map[string]Target
	// indexed is the target of every module the index was last built from;
	// the first sync reads it from the volume, so a restart keeps it.
	indexed     map[string]Target
	indexedRead bool
	// channels are the versions of the release channels of every module on
	// the public site; a module the registry does not answer for keeps its
	// last ones.
	channels map[string]Channels
	// saved is what saveIndexed recorded last, so an unchanged pass does not
	// upload it again.
	saved []Target
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
	// localOnly keeps targets.json on the volume (NewFallbackPinner).
	localOnly bool
	// failed keeps a target that does not exist, has no documentation, fails
	// the Hugo build or the AI export from being downloaded again on every
	// sync: the images of a version do not change. Other failures are not
	// kept and are tried on the next sync.
	failed map[Target]time.Time
	// upcoming are the versions the cluster is about to update to, as of the
	// last pass; warmed are those of them rendered once.
	upcoming []Target
	warmed   map[Target]bool
	// listed are the modules of the last pass for the list of modules;
	// noDocs are the targets whose versions have no documentation pages.
	listed []Listed
	noDocs map[Target]bool
	// unpublished are the modules of the last pass with no source or no
	// release repository.
	unpublished map[string]bool
	// savedModules is the JSON of the modules saveIndexed recorded last.
	savedModules string
	// served are the channels as the last pass linked them: a channel keeps
	// its version there until the new one is published, over many passes.
	served map[string]Channels
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
func NewPinner(resolver Resolver, store *Store, retryAfter time.Duration, logger *log.Logger) *Pinner {
	return &Pinner{
		resolver:   resolver,
		store:      store,
		retryAfter: retryAfter,
		logger:     logger,
		now:        time.Now,
		targets:    make(map[string]Target),
		indexed:    make(map[string]Target),
		channels:   make(map[string]Channels),
		failed:     make(map[Target]time.Time),
		warmed:     make(map[Target]bool),
		noDocs:     make(map[Target]bool),

		progressEvery: progressEvery,
		passTimeout:   passTimeout,
		kick:          make(chan struct{}, 1),
	}
}

// NewFallbackPinner is NewPinner for a follower that reads the registry
// itself while S3 does not answer: it keeps targets.json on the volume, so it
// never replaces the file of the leader in S3.
func NewFallbackPinner(resolver Resolver, store *Store, retryAfter time.Duration, logger *log.Logger) *Pinner {
	p := NewPinner(resolver, store, retryAfter, logger)
	p.localOnly = true

	return p
}

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
	p.targets = make(map[string]Target, len(targets))
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
	p.store.Pin(keep, slices.Concat(chanTargets, channelTargets(byModule(targets), linked)))

	// One by one: a reader opening another version gets the other render slot.
	// Development branches go last: they are rendered from the registry every
	// time, while release versions mostly come from the cold cache. The
	// versions of the channels go after the search.
	releases := slices.DeleteFunc(slices.Clone(targets), func(t Target) bool { return t.Rev != "" })
	branches := slices.DeleteFunc(slices.Clone(targets), func(t Target) bool { return t.Rev == "" })
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
	p.indexed = byModule(index)
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
// records nothing in the cold cache.
func (p *Pinner) uploadMissing(ctx context.Context, linked map[string]Channels) {
	if p.localOnly {
		return
	}
	pinned := slices.Concat(slices.Collect(maps.Values(p.indexed)), channelTargets(p.indexed, linked))
	if err := p.store.uploadMissing(ctx, pinned); err != nil && ctx.Err() == nil && !p.store.coldMissing(err) {
		p.logger.Warn("upload the versions missing from S3", log.Err(err))
	}
}

// keepUnpublishedOnce returns the targets with the previous target of a
// module unpublished for the first pass in a row, its card not marked: a
// registry uploading the module anew or a Module made again answers "not
// found" for a while. Unpublished on the next pass too, it goes.
func (p *Pinner) keepUnpublishedOnce(res Resolution, prev map[string]Target) []Target {
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
	slices.SortFunc(targets, func(a, b Target) int { return strings.Compare(a.Module, b.Module) })

	return targets
}

// readIndexed takes what the index was last built from and restores it. A
// cold cache that does not answer leaves it unread: recorded over, the file
// would lose the targets of the followers, so nothing is saved and the next
// pass reads it again. What a pass found meanwhile is newer and stays.
func (p *Pinner) readIndexed(ctx context.Context) {
	state, err := p.store.loadIndexed(ctx)
	// A follower reading the registry itself never records over the file
	// of the leader.
	if err != nil && !p.localOnly {
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
func (p *Pinner) publishedOf(targets []Target) []Target {
	out := make([]Target, 0, len(targets))
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
func (p *Pinner) nothingToPublish(targets []Target) bool {
	return len(p.unpublished) == 0 && !slices.ContainsFunc(targets, func(t Target) bool { return !p.noDocs[t] })
}

// save records the state in targets.json unless it is what was recorded
// last.
func (p *Pinner) save(ctx context.Context, saved []Target, chans map[string]Channels, modules []Listed) {
	if !p.indexedRead {
		return
	}
	chansJSON, _ := json.Marshal(chans)
	modulesJSON, _ := json.Marshal(modules)
	if slices.Equal(saved, p.saved) && string(chansJSON) == p.savedChannels && string(modulesJSON) == p.savedModules {
		return
	}
	state := indexedState{Targets: saved, Channels: chans, Modules: modules}
	save := p.store.saveIndexed
	if p.localOnly {
		save = func(_ context.Context, state indexedState) error {
			_, err := p.store.saveIndexedLocal(state)
			return err
		}
	}
	if err := save(ctx, state); err != nil {
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
func (p *Pinner) cardTargets(targets []Target) []Target {
	out := make([]Target, 0, len(targets))
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
func (p *Pinner) writeList(ctx context.Context, targets []Target) []Listed {
	cards := byModule(targets)
	modules := make([]Listed, 0, len(p.listed))
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
func (p *Pinner) upcomingTargets(ctx context.Context, chanTargets []Target) []Target {
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
	var out []Target
	for _, r := range releases {
		t := Target{Module: r.Module, Version: r.Version}
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
		if p.renderEach(ctx, []Target{t}, false) && !p.failedNow(t) {
			p.warmed[t] = true
		}
		if ctx.Err() != nil {
			return
		}
	}
}

// failedNow reports a version that failed for a reason that does not change
// and waits for retryAfter.
func (p *Pinner) failedNow(t Target) bool {
	at, ok := p.failed[t]

	return ok && p.now().Sub(at) < p.retryAfter
}

func compareTargets(a, b Target) int {
	return strings.Compare(a.Module+"/"+a.Version, b.Module+"/"+b.Version)
}

// channelTargets are the versions release channels point at besides the
// targets, one per version: they get pages of their own, not the search.
func channelTargets(targets map[string]Target, channels map[string]Channels) []Target {
	var out []Target
	for module, chans := range channels {
		for _, version := range chans {
			t := Target{Module: module, Version: version}
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
func (p *Pinner) renderEach(ctx context.Context, targets []Target, withAI bool) bool {
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
			if lasting(err) {
				p.failed[t] = p.now()
				if errors.Is(err, docs.ErrNoPages) {
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

// lasting reports whether a render failed for a reason that does not change
// until the template or the image does: trying it again at once fails again.
func lasting(err error) bool {
	return errors.Is(err, ErrVersionNotFound) || errors.Is(err, docs.ErrNoPages) || errors.Is(err, docs.ErrBuild) || errors.Is(err, docs.ErrDocsTooLarge) || errors.Is(err, errNoAIExport)
}

// servedChannels are the channels as they are served after a pass: the new
// version of a channel once it is published, the one it had before
// (linked) until then. With no linked it is the channels whose versions are
// published.
func (p *Pinner) servedChannels(linked map[string]Channels) map[string]Channels {
	out := make(map[string]Channels)
	for module, chans := range p.channels {
		for channel, version := range chans {
			if !p.store.published(Target{Module: module, Version: version}) {
				old, ok := linked[module][channel]
				if !ok || !p.store.published(Target{Module: module, Version: old}) {
					continue
				}
				version = old
			}
			if out[module] == nil {
				out[module] = make(Channels)
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
	targets := make([]Target, 0, len(p.indexed))
	for _, t := range p.indexed {
		targets = append(targets, t)
	}
	slices.SortFunc(targets, func(a, b Target) int { return strings.Compare(a.Module, b.Module) })

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

func byModule(targets []Target) map[string]Target {
	out := make(map[string]Target, len(targets))
	for _, t := range targets {
		out[t.Module] = t
	}

	return out
}
