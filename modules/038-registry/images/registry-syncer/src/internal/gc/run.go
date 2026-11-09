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
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

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

	// SweepAnyway sweeps even when this run deleted nothing and found nothing unreferenced to sweep.
	//
	// For a store that refuses writes, which needs every byte back and cannot afford to trust the
	// count: a run with nothing to delete sweeps whatever that count finds (see sweeps), and this is
	// what makes it sweep when the count fails or says nothing.
	SweepAnyway bool

	// Pushing reports whether somebody is pushing into this store from outside right now. Nil means
	// never.
	//
	// Asked again right before the sweep, which is the step a push cannot survive: its layers are on
	// disk before the manifest that references them, and a sweep removes exactly that. The worker asks
	// before the collection too, but the manifest pass can take minutes, and a push that starts in
	// them is no less a push.
	Pushing func() bool

	// Upstream is where the set of the deployed release is read when this store cannot answer for it,
	// with the credentials to read it. Nil in air-gap.
	//
	// The store answers only for a release whose installer it holds, and a store that refuses writes
	// never got to take in the installer of the release the cluster has moved to — so the set could not
	// be read, nothing was judged, and the store that most needed collecting was never collected. The
	// fill reads the platform's set from here; the modules are still read from this store, as they are
	// whenever the collector judges anything.
	Upstream *fill.Registry

	// DryRun decides what would go without touching anything.
	DryRun bool

	// ReleaseAhead gives back what is ahead of the set: a newer release, and the images of pending
	// module updates, are collected like anything else the cluster has moved past.
	//
	// For a store short of room whose upstream holds all of it again. What is ahead of the set was
	// protected without condition, so room it had taken stayed taken: the copy ahead stops at a room
	// limit, but a collection did not give back what it had already put in, and the next change of the
	// set met a store with no room for it. Never in air-gap — there it was pushed, and nothing could
	// bring it back.
	ReleaseAhead bool
}

// Sweeper reclaims unreachable blobs, and says how many it removed: -1 when it cannot tell.
type Sweeper interface {
	Sweep(ctx context.Context) (int, error)
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

	// deletionsUnsupported is set when a registry answered a deletion by refusing the method, which
	// says nothing about the reference and everything about the address the run was pointed at.
	deletionsUnsupported bool

	// deletionsDenied is set when a registry answered a deletion by refusing the account the right to
	// delete — the same answer for every reference the account asks about.
	deletionsDenied bool

	// Failed are the references that could not be removed, which is not a failure of the
	// run: the disk is simply less reclaimed than it could have been.
	Failed []string

	// Swept reports whether blob reclamation ran.
	Swept bool

	// RemovedVersions are the platform releases whose tags were removed — the readable part of
	// Deleted, which also carries every manifest by digest.
	RemovedVersions []string

	// RemovedManifests is how many manifests were deleted by digest: the images the fill writes with
	// no tag at all, which is where nearly all of a store's weight is.
	RemovedManifests int

	// RemovedBlobs is how many blobs the sweep removed, as distribution counts them; -1 when the sweep
	// did not run or did not say.
	RemovedBlobs int

	// Orphaned is how many bytes of blobs nothing references a run that had nothing to delete found
	// on disk, and swept; zero when it deleted something, or found none.
	Orphaned int64

	// Doomed are the manifests a dry run judged removable, for Reclaimable to weigh. Empty unless
	// DryRun.
	Doomed []fill.HeldManifest

	// ReclaimedBytes is how much the store shrank over the run, nil when it could not be measured.
	// Measured by whoever runs the collection, around it: the collector itself only deletes.
	ReclaimedBytes *int64

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
// deletionUnsupported tells a reference that would not go from a registry that will not delete.
//
// The distinction is the whole point: a 404 is one manifest the registry does not have and the run
// carries on, while a 405 is the method itself being refused — the answer a pull-through cache gives
// every single time, for every reference, forever.
func deletionUnsupported(err error) bool {
	var status *transport.Error
	if !errors.As(err, &status) {
		return false
	}

	if status.StatusCode == http.StatusMethodNotAllowed || status.StatusCode == http.StatusNotImplemented {
		return true
	}

	for _, diagnostic := range status.Errors {
		if diagnostic.Code == transport.UnsupportedErrorCode {
			return true
		}
	}

	return false
}

// absent tells a reference the registry does not hold from a registry that could not be asked.
func absent(err error) bool {
	var status *transport.Error
	if !errors.As(err, &status) {
		return false
	}
	return status.StatusCode == http.StatusNotFound
}

// deletionDenied tells a reference that would not go from an account that may not delete anything.
//
// A 401 or 403 on a DELETE through a token service is the scope it issued: the account was granted
// less than "delete", and every later reference is refused the same way. Seen on a live cluster where
// the replica's account was granted pull and push only: hundreds of manifests judged garbage, every one
// of them refused, and the run reported as a success that had found nothing to reclaim.
func deletionDenied(err error) bool {
	var status *transport.Error
	if !errors.As(err, &status) {
		return false
	}
	if status.StatusCode == http.StatusUnauthorized || status.StatusCode == http.StatusForbidden {
		return true
	}
	for _, diagnostic := range status.Errors {
		if diagnostic.Code == transport.UnauthorizedErrorCode || diagnostic.Code == transport.DeniedErrorCode {
			return true
		}
	}
	return false
}

func (c *Collector) Run(ctx context.Context) (Report, error) {
	report := Report{Kept: map[string]int{}, RemovedBlobs: -1}

	// What the tag pass may delete, and whether it may delete anything at all.
	//
	// Its rule is ordering — "older than what is deployed" — which needs a version that can be
	// ordered, and a cluster running a tag has none. Only that pass needs it, though: the manifest
	// pass needs the set the version declares, which reads for a tag exactly as for a version. So an
	// unorderable version disables the pass that cannot work and leaves the one that can, rather than
	// leaving the store growing with nothing reclaimed.
	//
	// Nothing known at all stays a refusal: with no version to compare against, neither pass has any
	// justification for a deletion.
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

	scope := strings.Trim(c.Registry.Scope, "/")

	var doomed, survivors []name.Tag
	for _, tag := range tags {
		decision := Decision{Reason: ReasonUnorderable}
		switch {
		case unorderable:
		case strings.Trim(tag.RepositoryStr(), "/") != scope:
			// Judged by nobody, because the only rule here is "older than the release the cluster
			// runs" and that rule owns exactly one version space: the platform's own releases, which
			// carry their tags on the scope root. Everything beneath it is versioned by somebody
			// else — a module package by the module, a node package by the software it carries — so
			// comparing those numbers against the platform's compares nothing at all — a module package
			// at v0.6.10 reads as superseded by a platform at v1.76.6 and is deleted. The manifest pass
			// then asks for the same tags, gets 404 and fails every run after that, and in an air-gapped
			// cluster a module package cannot be fetched again short of another `d8 mirror push`.
			decision = Decision{Reason: ReasonForeignVersionSpace}
		default:
			decision = Judge(tag.TagStr(), keep, c.Releases.Deployed)
			if c.ReleaseAhead && decision.Reason == ReasonNewerRelease {
				decision = Decision{Delete: true, Reason: ReasonReleasedAhead}
			}
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

	// Deletions follow the run's context: a cancellation, or the window closing, stops them between
	// one reference and the next instead of after the last.
	deleteOptions := c.deleteOptions(ctx)
	for _, tag := range doomed {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if c.DryRun {
			report.Deleted = append(report.Deleted, tag.String())
			report.RemovedVersions = append(report.RemovedVersions, tag.TagStr())
			continue
		}
		if err := remote.Delete(tag, deleteOptions...); err != nil {
			// One reference that would not go is not a reason to abandon the rest, nor to
			// skip the sweep: what did get deleted is still reclaimable.
			c.Log.Warn("cannot delete a reference", "reference", tag.String(), "error", err.Error())
			report.Failed = append(report.Failed, tag.String())
			report.deletionsUnsupported = report.deletionsUnsupported || deletionUnsupported(err)
			report.deletionsDenied = report.deletionsDenied || deletionDenied(err)
			continue
		}
		report.Deleted = append(report.Deleted, tag.String())
		// Only the scope root reaches here, so every deleted tag is a platform release.
		report.RemovedVersions = append(report.RemovedVersions, tag.TagStr())
	}

	// The manifests, which is where the store's weight actually is.
	//
	// Reached whatever the tag pass decided, because the tag pass finding nothing is the ORDINARY
	// case: the platform's images are written by digest, `<base>@sha256:…`, so a store filled by the
	// syncer is almost entirely untagged. Judged by tags alone those manifests are never considered
	// and never deleted, and distribution's own sweep cannot help — every manifest keeps its own
	// blobs reachable — so the store grows by one release set per update, forever.
	//
	// The rule: keep what the current set needs and what a set the cluster might switch to needs.
	// Everything else is a release the cluster has moved past.
	if err := c.reclaimManifests(ctx, &report, survivors); err != nil {
		return report, err
	}

	// A refusal of the method itself is not one reference that would not go: it is the whole
	// collection addressed at a registry that does not accept deletions, and every later reference
	// will be refused the same way.
	//
	// The listener this pod serves on is a pull-through cache, and the proxy store in front of it
	// implements no deletion at all — every DELETE comes back 405 Unsupported. Addressed there, a
	// collection reads the store from its files, judges correctly, is turned away throughout, and
	// records the refusals as warnings: a run that looks like it ran, on a disk that never stops
	// growing. Writes go to the non-proxying listener for the same reason fills do.
	if report.deletionsUnsupported {
		return report, fmt.Errorf(
			"this registry does not accept deletions at all (%d refused, first: %s): a collection "+
				"has to be addressed at the listener that does not proxy",
			len(report.Failed), report.Failed[0])
	}

	// The same for an account refused the right to delete: not a reference that would not go, but a
	// run that could delete nothing at all, and must not be reported as one that found nothing.
	if report.deletionsDenied {
		return report, fmt.Errorf(
			"this registry refuses the account the right to delete (%d refused, first: %s): the "+
				"replica's account needs the delete action",
			len(report.Failed), report.Failed[0])
	}

	if c.DryRun {
		c.Log.Info("dry run, nothing was touched",
			"would_delete", len(report.Deleted), "kept", report.Kept)
		return report, nil
	}

	if !c.sweeps(&report) {
		c.Log.Info("nothing to reclaim", "considered", report.Considered, "kept", report.Kept)
		return report, nil
	}

	if c.Sweep == nil {
		return report, nil
	}

	if c.Pushing != nil && c.Pushing() {
		// Left for the next run, which finds what is left unreferenced on disk and sweeps it — see
		// sweeps. A run that deleted manifests failed to give their space back, and says so.
		if len(report.Deleted) == 0 {
			c.Log.Info("an image set is being pushed into the store, so blobs nothing references are not swept now")
			return report, nil
		}
		return report, fmt.Errorf("the %d deleted references were not swept: an image set is being pushed "+
			"into the store, and a sweep would remove its layers; the next collection sweeps them", len(report.Deleted))
	}

	blobs, err := c.Sweep.Sweep(ctx)
	if err != nil {
		// The manifests are gone either way, so the run did something. Reported so that a
		// disk that never shrinks is attributable to the sweep rather than to the
		// judgement.
		if len(report.Deleted) == 0 {
			return report, fmt.Errorf("reclaiming the blobs nothing references: %w", err)
		}
		return report, fmt.Errorf("reclaiming the blobs of %d deleted references: %w", len(report.Deleted), err)
	}
	report.Swept = true
	report.RemovedBlobs = blobs

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

	// And the platform manifests of every kept index — see keepChildren.
	if err := keepChildren(c.DataDir, keep); err != nil {
		return fmt.Errorf("cannot read the indexes the kept set names, so none were deleted: %w", err)
	}

	registry, err := c.registry()
	if err != nil {
		return err
	}

	deleteOptions := c.deleteOptions(ctx)
	for _, manifest := range held {
		if err := ctx.Err(); err != nil {
			return err
		}
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
			report.RemovedManifests++
			report.Doomed = append(report.Doomed, manifest)
			continue
		}

		if err := remote.Delete(reference, deleteOptions...); err != nil {
			c.Log.Warn("cannot delete a manifest",
				"reference", reference.String(), "error", err.Error())
			report.Failed = append(report.Failed, reference.String())
			report.deletionsUnsupported = report.deletionsUnsupported || deletionUnsupported(err)
			report.deletionsDenied = report.deletionsDenied || deletionDenied(err)
			continue
		}
		report.Deleted = append(report.Deleted, reference.String())
		report.RemovedManifests++
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
		if err != nil && version == c.Releases.Deployed && c.Upstream != nil {
			c.Log.Info("this store does not answer for the set of the deployed release, so it is read from the upstream",
				"version", version, "upstream", c.Upstream.Address, "reason", err.Error())
			if upstreamErr := c.addUpstreamSet(ctx, version, source, puller, keep); upstreamErr != nil {
				return nil, fmt.Errorf("%w; and from the upstream: %w", err, upstreamErr)
			}
			continue
		}
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
		if err := c.addDigests(ctx, puller, references, keep, true); err != nil {
			return nil, err
		}
	}

	// And the modules the cluster is about to update to. Discover counts only the modules it runs, so
	// what the fill copied ahead for a pending module release — or what `d8 mirror push` brought for
	// it — was unclaimed here and went at the next collection, before the update could use it. The
	// pending platform versions need nothing of the kind: they are tags newer than the deployed one,
	// kept as such, and their sets are read above with the other kept versions.
	//
	// A pending module the store does not hold is no error — it claims nothing, and nothing of it is
	// there to delete. One whose set is there and cannot be read is: its images cannot be told apart from
	// anything else, and in an air-gapped cluster what was pushed for it comes back only with another
	// `d8 mirror push`. So nothing is deleted at all; the modules are read one at a time, and one that
	// cannot be read no longer takes the others' protection with it.
	if c.ReleaseAhead {
		return keep, nil
	}
	pending, err := fill.Release{Modules: c.Modules}.Optional(ctx, source, puller)
	if err != nil {
		return nil, fmt.Errorf("the image set of a pending module could not be read, so nothing can be "+
			"judged safe to delete: %w", err)
	}
	for _, reference := range pending {
		// One at a time and never fatal: a pending module the copy ahead missed is simply not in the
		// store, and that must not stop the collection of everything else. An unplaced one is the
		// exception, as for the modules the cluster runs — see unplacedPackage.
		if err := c.addDigests(ctx, puller, []name.Reference{reference}, keep, true); err != nil {
			if _, unplaced := c.unplacedPackage(reference); unplaced {
				return nil, err
			}
		}
	}
	return keep, nil
}

// addDigests reduces references to the digests they name, resolving tags against the registry the
// puller reads.
//
// local says that registry is this store. A tag it does not hold is then no error: nothing in the store
// is reached through it, so there is nothing of it to protect. The case is a module enabled just before
// the store ran out of room — its package is declared and never arrived — and failing on it failed every
// collection of a store that only a collection could make room in. On the upstream a missing tag is a
// set that cannot be read, and fails as before. The package of an unplaced module fails whether the
// store holds that tag or not — see unplacedPackage.
func (c *Collector) addDigests(
	ctx context.Context, puller *remote.Puller, references []name.Reference, keep map[string]struct{}, local bool,
) error {
	for _, reference := range references {
		if digest, ok := reference.(name.Digest); ok {
			keep[digest.DigestStr()] = struct{}{}
			continue
		}

		if local {
			if module, unplaced := c.unplacedPackage(reference); unplaced {
				return fmt.Errorf("module %s names ModuleSource %s, which the cluster does not have, so %s "+
					"is only a guess at where the store keeps it, and which of the store's images are the "+
					"module's is unknown", module.Name, module.MissingSource, reference)
			}
		}

		// A tag — the release image and its installer. What has to survive is whatever it points
		// at now, so it is resolved here rather than assumed absent.
		descriptor, err := puller.Head(ctx, reference)
		if local && absent(err) {
			c.Log.Info("a declared tag is not in the store, so it protects nothing", "reference", reference.String())
			continue
		}
		if err != nil {
			return fmt.Errorf("resolving %s: %w", reference, err)
		}
		keep[descriptor.Digest.String()] = struct{}{}
	}

	return nil
}

// unplacedPackage tells the package tag of a module the cluster keeps whose release names a
// ModuleSource the cluster does not have — see fill.ModuleRef.Unplaced.
//
// Such a module is looked for under the platform's path, and what a fill copied while its source
// existed is under the source's own. Its package missing from the platform's path is then no module
// whose package never arrived, and a package found there may be the platform's module of the same name
// and version: either way its images may well be in the store, and taking them for garbage deletes what
// an air-gapped cluster gets back only with another `d8 mirror push`. The state is short-lived: the
// releases of a deleted source are its dependents, and go with it.
func (c *Collector) unplacedPackage(reference name.Reference) (fill.ModuleRef, bool) {
	tag, ok := reference.(name.Tag)
	if !ok {
		return fill.ModuleRef{}, false
	}
	for _, module := range c.Modules {
		if module.Unplaced() && tag.TagStr() == module.Version &&
			strings.HasSuffix("/"+tag.RepositoryStr(), "/"+module.StorePath()) {
			return module, true
		}
	}
	return fill.ModuleRef{}, false
}

// addUpstreamSet claims the set of a release read the way the fill reads it: the platform from the
// upstream, and the modules the cluster keeps — with the catalogues — from this store, as declaredDigests
// reads them. Anything that cannot be read fails the whole: an unreadable part is one whose images could
// not be told apart from garbage.
func (c *Collector) addUpstreamSet(
	ctx context.Context, version string, local fill.Registry, localPuller *remote.Puller, keep map[string]struct{},
) error {
	upstreamPuller, err := remote.NewPuller(c.Upstream.Options...)
	if err != nil {
		return fmt.Errorf("building the client: %w", err)
	}

	platform, err := fill.Release{Versions: []string{version}}.Discover(ctx, *c.Upstream, upstreamPuller)
	if err != nil {
		return err
	}
	if err := c.addDigests(ctx, upstreamPuller, platform, keep, false); err != nil {
		return err
	}

	var kept []fill.ModuleRef
	for _, module := range c.Modules {
		if !module.Pending {
			kept = append(kept, module)
		}
	}
	modules, err := fill.ModuleReferences(ctx, local, localPuller, kept)
	if err != nil {
		return fmt.Errorf("the modules the cluster keeps: %w", err)
	}
	catalogue, err := fill.ModuleCatalogue(ctx, local, nil)
	if err != nil {
		return fmt.Errorf("the module catalogue: %w", err)
	}
	catalogues, err := fill.SourceCatalogues(ctx, local, kept, nil)
	if err != nil {
		return fmt.Errorf("the catalogues of the module sources: %w", err)
	}
	return c.addDigests(ctx, localPuller, append(append(modules, catalogue...), catalogues...), keep, true)
}

// sweeps decides whether this run goes on to sweep: when it deleted something, when the store is to
// be swept anyway, or when blobs nothing references lie on disk.
//
// The last is what a failed sweep leaves: the manifests it deleted are gone, their blobs stay, and the
// next run, finding nothing left to delete, never swept them — on a live cluster, 805 manifests
// deleted between two replicas and the stores at their budget for good. Nor only after a failure: a
// fill cut off between its blobs and its manifest leaves the same. Counted with Reclaimable, which is
// distribution's mark read from the files, so a run sweeps exactly when the estimate says it frees
// something. Inside the collection, with this replica's writes paused, and after a push the collection
// waited out: what is unreferenced now is not on its way to being referenced.
func (c *Collector) sweeps(report *Report) bool {
	if len(report.Deleted) > 0 || c.SweepAnyway {
		return true
	}
	if c.DataDir == "" {
		return false
	}
	orphaned, err := Reclaimable(c.DataDir, nil)
	if err != nil {
		c.Log.Warn("cannot tell whether blobs nothing references lie on disk, so nothing is swept",
			"error", err.Error())
		return false
	}
	if orphaned == 0 {
		return false
	}
	c.Log.Info("nothing to delete, but blobs nothing references lie on disk, so the store is swept",
		"bytes", orphaned)
	report.Orphaned = orphaned
	return true
}

// deleteOptions are the store's options with the run's context, in a slice of their own so the
// collector's are never appended to.
func (c *Collector) deleteOptions(ctx context.Context) []remote.Option {
	return append(append(make([]remote.Option, 0, len(c.Registry.Options)+1), c.Registry.Options...), remote.WithContext(ctx))
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
func (b *Binary) Sweep(ctx context.Context) (int, error) {
	if b.Path == "" || b.ConfigPath == "" {
		return -1, errors.New("no registry binary to reclaim blobs with")
	}

	command := exec.CommandContext(ctx, b.Path, "garbage-collect", b.ConfigPath)
	// Without this process's REGISTRY_* variables: distribution reads every one of them as an
	// override of the configuration it was given, so a variable meant for the syncer that happened to
	// name a configuration key would change the store the sweep runs on. Today they only earn a
	// warning each, at the top of the output.
	command.Env = withoutRegistryVariables(os.Environ())
	output, err := command.CombinedOutput()
	if err != nil {
		return -1, fmt.Errorf("%s garbage-collect: %w: %s", b.Path, err, lastLine(output))
	}

	summary := sweepSummary(output)
	b.Log.Info("the registry reclaimed its unreachable blobs", "summary", summary)
	return sweptBlobs(summary), nil
}

// sweptBlobs reads how many blobs a sweep removed out of upstream's summary, "N blobs marked, N blobs
// and N manifests eligible for deletion" — what is eligible is what a sweep that did not fail deleted.
// -1 when the summary does not say.
func sweptBlobs(summary string) int {
	var marked, blobs, manifests int
	if _, err := fmt.Sscanf(summary, "%d blobs marked, %d blobs and %d manifests eligible for deletion",
		&marked, &blobs, &manifests); err != nil {
		return -1
	}
	return blobs
}

// lastLine is what a failed sweep says about why: the wrapper prints its error last, after a line per
// manifest and per blob it marked — thousands of them on a real store, which once went whole into the
// replica's status.
func lastLine(output []byte) string {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// sweepSummary is upstream's own account of a sweep, "N blobs marked, N blobs and N manifests eligible
// for deletion", or the last line when there is none.
func sweepSummary(output []byte) string {
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "blobs marked") {
			return strings.TrimSpace(line)
		}
	}
	return lastLine(output)
}

func withoutRegistryVariables(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, variable := range environment {
		if !strings.HasPrefix(variable, "REGISTRY_") {
			kept = append(kept, variable)
		}
	}
	return kept
}
