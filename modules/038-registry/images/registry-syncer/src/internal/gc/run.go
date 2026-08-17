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
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/registry-syncer/internal/fill"
)

// Registry is the store to collect, as this replica reaches its own.
type Registry struct {
	// Address is where this replica's registry answers. The loopback address: a replica
	// collects its own store and nobody else's.
	Address string

	// Insecure allows plain HTTP, for a store served without TLS.
	Insecure bool

	// Scope limits which repositories are considered, so that a store holding anything
	// beyond this module's images is not judged by this module's rules.
	Scope string

	// Options are how to authenticate and whom to trust.
	Options []remote.Option
}

// Collector reclaims the disk of one replica.
type Collector struct {
	Log *slog.Logger

	// Registry is this replica's own store.
	Registry Registry

	// DataDir is where that store keeps its files, which is where the collector reads what it
	// holds. A field rather than a constant so the judgement can be exercised against a store a
	// test lays out — every deletion this component makes is justified by what it reads here.
	DataDir string

	// Releases is what the cluster is running, which decides what may go.
	Releases Releases

	// Modules are the modules the cluster keeps, whose images are as much part of what it needs as
	// the platform's own. Deleting them because a release does not mention them is how a cluster
	// loses the images of a module it is running — and in air-gap there is nowhere to get them back
	// from.
	Modules []fill.ModuleRef

	// Sweep reclaims the blobs left unreachable by the deleted manifests. Optional: with
	// no sweeper the manifests are deleted and the disk is not reclaimed, which is
	// useless but not harmful.
	Sweep Sweeper

	// DryRun decides what would go without touching anything.
	DryRun bool
}

// Sweeper reclaims unreachable blobs.
type Sweeper interface {
	Sweep(ctx context.Context) error
}

// Report is what a run did.
type Report struct {
	// Considered is how many tags were examined.
	Considered int

	// Deleted are the references that were removed.
	Deleted []string

	// Kept counts the tags left alone, by the reason they were kept. The interesting
	// number when a disk is still full after a run.
	Kept map[string]int

	// Failed are the references that could not be removed, which is not a failure of the
	// run: the disk is simply less reclaimed than it could have been.
	Failed []string

	// Swept reports whether blob reclamation ran.
	Swept bool

	// KeptVersions are the release versions the tag pass decided to keep, which the manifest
	// pass then enumerates the image sets of. Recorded rather than recomputed so that the two
	// passes cannot disagree — a manifest deleted while a kept tag points at it would leave the
	// tag dangling.
	KeptVersions []string
}

// Run collects the store.
//
// Deletes manifests first and reclaims blobs second, because that is the only order that
// works: distribution's collector reclaims what no manifest references, so the references
// have to be gone before it runs.
//
// A failure to delete one manifest does not stop the run. The blobs of the ones that were
// deleted are still reclaimable, and a store that is partly collected is the ordinary
// outcome on a registry that is also serving.
func (c *Collector) Run(ctx context.Context) (Report, error) {
	report := Report{Kept: map[string]int{}}

	// What the tag pass may delete, and whether it may delete anything at all.
	//
	// Its rule is ordering — "older than what is deployed" — so it needs a deployed value that can
	// be ordered. A cluster running a tag rather than a release version has none: measured on such a
	// cluster, `the deployed version "pr21788" is not a version, so no tag can be judged older than
	// it`, and the whole run stopped there.
	//
	// Stopping the whole run is the wrong response, and this is the difference between the two
	// passes. Ordering is what the TAG pass needs; the manifest pass needs only the set that version
	// declares, which is readable for a tag exactly as for a version. So an unorderable version
	// disables the pass that cannot work and leaves the one that can — every tag is kept, and the
	// digests are still judged against what the running version declares. Refusing both left the
	// store growing with nothing ever reclaimed.
	// Nothing known at all is still a refusal, and it has to stay one: with no version to compare
	// against, neither pass has any justification for a deletion, and the run must do nothing rather
	// than its best.
	if strings.TrimSpace(c.Releases.Deployed) == "" {
		return report, fmt.Errorf(
			"cannot decide what to keep: the deployed version is unknown, so there is nothing to keep against")
	}

	keep, err := c.Releases.Keep()
	unorderable := err != nil
	if unorderable {
		c.Log.Info("tags cannot be ordered against what the cluster runs, so none are deleted",
			"deployed", c.Releases.Deployed, "reason", err.Error())
	}

	tags, err := c.tags(ctx)
	if err != nil {
		return report, err
	}
	report.Considered = len(tags)

	var doomed, survivors []name.Tag
	for _, tag := range tags {
		decision := Decision{Reason: ReasonUnorderable}
		if !unorderable {
			decision = Judge(tag.TagStr(), keep, c.Releases.Deployed)
		}
		if !decision.Delete {
			report.Kept[decision.Reason]++
			survivors = append(survivors, tag)
			// A version tag that survives names a set the manifest pass must not touch: the
			// deployed release, the one a rollback goes to, or an update newer than either.
			if decision.Reason == ReasonCurrentRelease ||
				decision.Reason == ReasonPreviousRelease ||
				decision.Reason == ReasonNewerRelease {
				report.KeptVersions = append(report.KeptVersions, tag.TagStr())
			}
			continue
		}
		doomed = append(doomed, tag)
	}

	for _, tag := range doomed {
		if c.DryRun {
			report.Deleted = append(report.Deleted, tag.String())
			continue
		}
		if err := remote.Delete(tag, c.Registry.Options...); err != nil {
			// One reference that would not go is not a reason to abandon the rest, nor to
			// skip the sweep: what did get deleted is still reclaimable.
			c.Log.Warn("cannot delete a reference", "reference", tag.String(), "error", err.Error())
			report.Failed = append(report.Failed, tag.String())
			continue
		}
		report.Deleted = append(report.Deleted, tag.String())
	}

	// The manifests, which is where the store's weight actually is.
	//
	// Reached whatever the tag pass decided, and that is not a detail: the two passes answer
	// different questions, and the tag pass finding nothing to delete is the ORDINARY case — a store
	// holding only the current release has no old version tags at all, while every superseded
	// digest is still in it. Returning early on "no tags to delete", which is what this did, is
	// exactly how the growth stayed invisible.
	//
	// Everything above judges tags, and the platform's own images have none: they are written by
	// digest, `<base>@sha256:…`, so a store filled by the syncer is almost entirely untagged. Judged
	// by tags alone, those manifests are invisible — they are never considered, never deleted, and
	// distribution's own sweep cannot help either, because every manifest keeps its own blobs
	// reachable. The store then grows by one release set per update, forever.
	//
	// The rule is the one the operator stated: keep what the current set needs, and what a set the
	// cluster might switch to needs. Everything else is a release the cluster has moved past.
	if err := c.reclaimManifests(ctx, &report, survivors); err != nil {
		return report, err
	}

	if c.DryRun {
		c.Log.Info("dry run, nothing was touched",
			"would_delete", len(report.Deleted), "kept", report.Kept)
		return report, nil
	}

	if len(report.Deleted) == 0 {
		c.Log.Info("nothing to reclaim", "considered", report.Considered, "kept", report.Kept)
		return report, nil
	}

	if c.Sweep == nil {
		return report, nil
	}

	if err := c.Sweep.Sweep(ctx); err != nil {
		// The manifests are gone either way, so the run did something. Reported so that a
		// disk that never shrinks is attributable to the sweep rather than to the
		// judgement.
		return report, fmt.Errorf("reclaiming the blobs of %d deleted references: %w", len(report.Deleted), err)
	}
	report.Swept = true

	c.Log.Info("reclaimed the store",
		"deleted", len(report.Deleted), "failed", len(report.Failed), "kept", report.Kept)
	return report, nil
}

// reclaimManifests deletes the manifests no release the cluster keeps declares.
//
// Refuses to delete anything at all if the keep-set cannot be established. That is not caution for
// its own sake: this function deletes the images the cluster runs on, and an empty keep-set is
// indistinguishable, to the loop below, from "nothing is needed". The same principle guards the tag
// pass above — every deletion is justified by a comparison, and without the comparison there is no
// deletion.
func (c *Collector) reclaimManifests(ctx context.Context, report *Report, survivors []name.Tag) error {
	held, err := fill.HeldManifests(c.DataDir, strings.Trim(c.Registry.Scope, "/"))
	if err != nil {
		return err
	}
	if len(held) == 0 {
		return nil
	}
	report.Considered += len(held)

	keep, err := c.declaredDigests(ctx, c.keptVersions(report))
	if err != nil {
		return fmt.Errorf("cannot establish which manifests are needed, so none were deleted: %w", err)
	}

	// And whatever the surviving tags point at right now.
	//
	// Without this the two passes could contradict each other: a tag kept above whose manifest is
	// not declared by any enumerable version would have that manifest deleted underneath it, leaving
	// a tag that resolves to nothing. It matters most in exactly the case that brought this about —
	// a cluster running a tag, where ordering is impossible and every tag is therefore kept, some of
	// them from releases whose sets cannot be read at all.
	if err := c.protectSurvivingTags(ctx, survivors, keep); err != nil {
		return fmt.Errorf("cannot establish which manifests the kept tags need, so none were deleted: %w", err)
	}

	registry, err := c.registry()
	if err != nil {
		return err
	}

	for _, manifest := range held {
		if _, needed := keep[manifest.Digest]; needed {
			report.Kept[ReasonDeclared]++
			continue
		}

		reference, err := name.NewDigest(
			fmt.Sprintf("%s/%s@%s", registry.Name(), manifest.Repository, manifest.Digest),
			c.nameOptions()...)
		if err != nil {
			report.Failed = append(report.Failed, manifest.Repository+"@"+manifest.Digest)
			continue
		}

		if c.DryRun {
			report.Deleted = append(report.Deleted, reference.String())
			continue
		}

		if err := remote.Delete(reference, c.Registry.Options...); err != nil {
			c.Log.Warn("cannot delete a manifest",
				"reference", reference.String(), "error", err.Error())
			report.Failed = append(report.Failed, reference.String())
			continue
		}
		report.Deleted = append(report.Deleted, reference.String())
	}

	return nil
}

// keptVersions is which releases' image sets must survive: the deployed one, the one a rollback
// would go to, and any newer one the store already holds — an update the cluster may switch to.
//
// Derived from the tag pass rather than from a second source, so that the two cannot disagree. A
// manifest deleted while a kept tag still points at it would leave that tag dangling, and the
// version it names unpullable.
func (c *Collector) keptVersions(report *Report) []string {
	versions := make([]string, 0, 3)
	seen := make(map[string]struct{})
	add := func(version string) {
		if version == "" {
			return
		}
		if _, already := seen[version]; already {
			return
		}
		seen[version] = struct{}{}
		versions = append(versions, version)
	}

	// The running version first, whether or not it looks like one: it is what the store is for, and
	// on a cluster running a tag it is the only thing the manifest pass can judge against.
	add(c.Releases.Deployed)
	add(c.Releases.Previous)
	for _, version := range report.KeptVersions {
		add(version)
	}
	return versions
}

// protectSurvivingTags adds the manifests the kept tags resolve to, so that keeping a tag and
// deleting what it points at cannot both happen.
func (c *Collector) protectSurvivingTags(
	ctx context.Context, survivors []name.Tag, keep map[string]struct{},
) error {
	if len(survivors) == 0 {
		return nil
	}

	puller, err := remote.NewPuller(c.Registry.Options...)
	if err != nil {
		return fmt.Errorf("building the client: %w", err)
	}

	for _, tag := range survivors {
		descriptor, err := puller.Head(ctx, tag)
		if err != nil {
			// A tag the registry will not resolve is one this store does not really serve. Not a
			// reason to abandon the pass, and not a reason to protect anything either.
			c.Log.Warn("a kept tag could not be resolved", "tag", tag.String(), "error", err.Error())
			continue
		}
		keep[descriptor.Digest.String()] = struct{}{}
	}
	return nil
}

// declaredDigests is every manifest those versions declare, read out of their installers in this
// store — the same enumeration the fill copies by, so the collector cannot delete what a fill would
// immediately put back.
func (c *Collector) declaredDigests(ctx context.Context, versions []string) (map[string]struct{}, error) {
	if len(versions) == 0 {
		return nil, fmt.Errorf("no release version is known")
	}

	puller, err := remote.NewPuller(c.Registry.Options...)
	if err != nil {
		return nil, fmt.Errorf("building the client: %w", err)
	}

	source := fill.Registry{
		Address:    c.Registry.Address,
		Repository: strings.Trim(c.Registry.Scope, "/"),
		Insecure:   c.Registry.Insecure,
		Options:    c.Registry.Options,
	}

	// Enumerated per version rather than in one call, because one version that cannot be read must
	// not decide the fate of the rest. The running version is the exception: it is what the store is
	// for, so failing to read ITS set means nothing here can be judged.
	keep := make(map[string]struct{})
	for _, version := range versions {
		references, err := fill.Release{
			Versions: []string{version},
			Modules:  c.Modules,
		}.Discover(ctx, source, puller)
		if err != nil {
			if version == c.Releases.Deployed {
				return nil, err
			}
			// An older or newer release whose set is unreadable: its images cannot be told apart
			// from anything else, so nothing is claimed for it. What its tag points at is still
			// protected — see protectSurvivingTags.
			c.Log.Warn("the image set of a kept version could not be read, so it claims nothing",
				"version", version, "error", err.Error())
			continue
		}
		if err := c.addDigests(ctx, puller, references, keep); err != nil {
			return nil, err
		}
	}
	return keep, nil
}

// addDigests reduces references to the digests they name, resolving tags against this store.
func (c *Collector) addDigests(
	ctx context.Context, puller *remote.Puller, references []name.Reference, keep map[string]struct{},
) error {
	for _, reference := range references {
		if digest, ok := reference.(name.Digest); ok {
			keep[digest.DigestStr()] = struct{}{}
			continue
		}

		// A tag — the release image and its installer. What has to survive is whatever it points
		// at now, so it is resolved here rather than assumed absent.
		descriptor, err := puller.Head(ctx, reference)
		if err != nil {
			return fmt.Errorf("resolving %s: %w", reference, err)
		}
		keep[descriptor.Digest.String()] = struct{}{}
	}

	return nil
}

// registry is this store as a name.Registry, parsed once for the references built against it.
func (c *Collector) registry() (name.Registry, error) {
	registry, err := name.NewRegistry(c.Registry.Address, c.nameOptions()...)
	if err != nil {
		return name.Registry{}, fmt.Errorf("parsing the registry address %q: %w", c.Registry.Address, err)
	}
	return registry, nil
}

func (c *Collector) nameOptions() []name.Option {
	if c.Registry.Insecure {
		return []name.Option{name.Insecure}
	}
	return nil
}

// tags lists every tag of every in-scope repository of this replica's own store.
//
// Read off the store's own filesystem rather than asked of the registry, and that is the whole
// point: while pull-through is configured, a listing is answered with the UPSTREAM's contents. A
// collector fed that answer judges thousands of tags the store never held — every development build
// of the upstream — decides most of them are old releases, and then tries to delete them from a
// registry that is read-only in that arrangement, while the few local tags it exists to judge are
// lost among them. What may be deleted has to be what is actually here.
func (c *Collector) tags(_ context.Context) ([]name.Tag, error) {
	var options []name.Option
	if c.Registry.Insecure {
		options = append(options, name.Insecure)
	}
	registry, err := name.NewRegistry(c.Registry.Address, options...)
	if err != nil {
		return nil, fmt.Errorf("parsing the registry address %q: %w", c.Registry.Address, err)
	}

	held, err := fill.HeldTags(c.DataDir, strings.Trim(c.Registry.Scope, "/"))
	if err != nil {
		return nil, err
	}

	tags := make([]name.Tag, 0, len(held))
	for _, entry := range held {
		tags = append(tags, registry.Repo(entry.Repository).Tag(entry.Tag))
	}
	return tags, nil
}

// inScope keeps the collector from judging repositories this module does not own.
//
// A store may hold more than this module put there, and the rules here are about Deckhouse
// releases. Applying them to somebody else's images would delete by a measure that has
// nothing to do with them.
func (c *Collector) inScope(repository string) bool {
	scope := strings.Trim(c.Registry.Scope, "/")
	if scope == "" {
		return true
	}

	trimmed := strings.Trim(repository, "/")
	return trimmed == scope || strings.HasPrefix(trimmed, scope+"/")
}

// Binary reclaims blobs by running the registry's own collector.
//
// The registry binary rather than an implementation of our own, because the store layout is
// distribution's and the cost of misunderstanding it is deleting a blob that is still
// referenced. It is the same binary the serving container runs, imported into this image at
// build time.
type Binary struct {
	Log *slog.Logger

	// Path of the registry binary.
	Path string

	// ConfigPath is the rendered configuration, which is where the collector learns the
	// store's location.
	ConfigPath string
}

// Sweep runs the collector.
//
// `--delete-untagged` is deliberately NOT passed. In a pass-through cache the manifests
// fetched on a cache miss are untagged by nature, and removing them on every run would empty
// the cache of exactly the images it was asked to hold — turning a garbage collection into a
// cache flush. What this run reclaims is what the deleted tags made unreachable.
func (b *Binary) Sweep(ctx context.Context) error {
	if b.Path == "" || b.ConfigPath == "" {
		return errors.New("no registry binary to reclaim blobs with")
	}

	command := exec.CommandContext(ctx, b.Path, "garbage-collect", b.ConfigPath)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s garbage-collect: %w: %s", b.Path, err, strings.TrimSpace(string(output)))
	}

	b.Log.Info("the registry reclaimed its unreachable blobs",
		"output", strings.TrimSpace(string(output)))
	return nil
}
