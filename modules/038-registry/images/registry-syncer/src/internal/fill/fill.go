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

// Package fill copies images into the local storage and accounts for what ended
// up there.
//
// The accounting is the reason this exists as its own component. Completeness
// cannot be established by asking the registry whether it serves an image,
// because a pass-through cache answers yes by fetching it from the upstream on
// the spot: the probe would report a full cache on an empty store, and the
// controller would then drop the upstream out from under the cluster. So
// completeness is derived from what was actually WRITTEN, and in an air-gapped
// cluster — where self-filling is impossible — from an honest read of the
// catalogue.
package fill

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// Registry is one end of a copy.
type Registry struct {
	// Address is "host[:port]" of the registry.
	Address string

	// Repository is the prefix images live under, without a leading slash.
	Repository string

	// Insecure allows plain HTTP.
	Insecure bool

	// Options are the transport and authentication options to talk to it.
	Options []remote.Option
}

// RegistryOptions builds the options for a registry, given its certificate
// authority and credentials.
func RegistryOptions(certificateAuthority, username, password string) ([]remote.Option, error) {
	roundTripper, err := roundTripper(certificateAuthority)
	if err != nil {
		return nil, err
	}

	options := []remote.Option{remote.WithTransport(roundTripper)}
	if username != "" || password != "" {
		options = append(options, remote.WithAuth(&authn.Basic{Username: username, Password: password}))
	}
	return options, nil
}

// Report is the outcome of a fill, and the input to the replica status.
type Report struct {
	// Written is how many distinct digests this run put into the local storage,
	// including the ones it confirmed were already there.
	//
	// This is the number the air-gap gate is derived from, and it comes from the
	// copy itself rather than from asking the registry what it can serve.
	Written int32

	// Skipped is how many were already present with the same digest.
	Skipped int32

	// Failed lists the references that could not be copied, so a partial fill says
	// which part is missing rather than only that it is incomplete.
	Failed []string

	// Pending lists the references the SOURCE does not have yet.
	//
	// Distinct from Failed, and the distinction is what makes replication from a leader that is
	// still filling possible. A follower enumerates the same set the leader is filling towards, so
	// while the leader is part-way through, some of that set is simply not there yet — which is not
	// a failure of anything, it is the reason to come back. Counted as failure, as it was, it made
	// every early replication look broken, which is why followers were kept from starting at all
	// until the leader was complete — and that left them idle for as long as a fill takes, holding
	// nothing if the leader died meanwhile.
	Pending []string

	// Total is how many distinct references the run set out to copy — the size of the set the
	// kept releases declare, as enumerated from their installers.
	//
	// Recorded because it is the expectation. It used to be supplied by the operator as
	// `storage.source.expectedDigests` and by nothing else, so a cluster whose configuration
	// omitted it — every cache-with-an-upstream installation, since that field belongs to the
	// air-gap shape — could never be judged complete: no completeness means no
	// `safeToDropUpstream`, hence no air-gap, and it means no follower ever replicates, because
	// replication only happens from a leader that reads as full. The set is knowable without
	// being told, and here it is known.
	Total int32
}

// Complete reports whether the run put in place every image the cluster needs.
//
// The expectation is the size of the set this run enumerated, and NOTHING ELSE — by the owner's rule:
// completeness is how full the store is of the images the cluster needs, and the total number of images
// must not enter the decision. So `storage.source.expectedDigests` is not consulted here even as a
// fallback: it is what an operator measured in a bundle, and a bundle holds more than any cluster needs
// — other modules, other editions, attestations. Measured on an air-gapped cluster installed from one:
// 333 declared digests held against a stated 556, so the store stayed `Filling` while serving
// everything the cluster ran, and the installation failed waiting for a completeness unreachable by
// construction.
//
// A run that enumerated nothing is therefore never complete, whatever an operator stated. That is the
// safe direction: an empty set is not a full cache, and calling it one would authorize dropping the
// upstream on no evidence at all.
//
// The same rule holds in applyCatalogue, and the two must not drift apart: a copying pass and a
// counting pass that disagree about what "full" means is how a lease came to travel between three
// replicas every twenty seconds.
func (r *Report) Complete() bool {
	if r.Total <= 0 {
		return false
	}
	return len(r.Failed) == 0 && r.Written >= r.Total
}

// Copier moves images from one registry into another.
type Copier struct {
	Source      Registry
	Destination Registry

	// Discover decides what the copy consists of. Required, and required to be chosen at
	// the call site: see Discoverer.
	Discover Discoverer

	// OnProgress is called after each reference, so a long fill reports progress
	// rather than going silent for however long it takes.
	OnProgress func(done, total int32)

	// StoreDir is the destination's data directory, when the destination is this replica's own
	// store — which it always is here.
	//
	// Needed because "the destination already has this digest" is answered by asking the registry
	// for the manifest, and a registry answers yes for a manifest it holds without a single layer.
	// A pull-through cache writes exactly that: the manifest at the moment it SERVES it, the layers
	// only when somebody asks. So a copy into such a store skips every image it was meant to fix,
	// reports them all as already present, and the store stays unservable forever.
	//
	// Measured on `ly-mmc` after completeness started reading layers: the leader correctly said it
	// held none of the set, began a fill, and copied nothing at all — 333 MB before, 333 MB after,
	// zero writes to the registry — because every image "was already there".
	//
	// Empty disables the check, which is what a destination that is not a local directory gets.
	StoreDir string
}

// Run copies every tag of every repository of the source into the destination.
//
// A failure on one reference does not abort the run. A partially filled cache is
// useful — the nodes fall back to the upstream for the rest — while stopping at
// the first error would throw away the work already done and leave the report
// with no idea how much is actually there.
func (c *Copier) Run(ctx context.Context) (Report, error) {
	report := Report{}

	sourcePuller, err := remote.NewPuller(c.Source.Options...)
	if err != nil {
		return report, fmt.Errorf("building the source client: %w", err)
	}
	destinationPuller, err := remote.NewPuller(c.Destination.Options...)
	if err != nil {
		return report, fmt.Errorf("building the destination client: %w", err)
	}
	pusher, err := remote.NewPusher(c.Destination.Options...)
	if err != nil {
		return report, fmt.Errorf("building the destination pusher: %w", err)
	}

	if c.Discover == nil {
		return report, errors.New("no way to discover what to copy")
	}
	references, err := c.Discover.Discover(ctx, c.Source, sourcePuller)
	if err != nil {
		return report, fmt.Errorf("discovering what to copy: %w", err)
	}

	total := int32(len(references))
	report.Total = total
	for _, reference := range references {
		if err := ctx.Err(); err != nil {
			return report, err
		}

		written, err := c.copyOne(ctx, sourcePuller, destinationPuller, pusher, reference)
		switch {
		case err != nil && absentAtSource(err):
			// Not there yet, which is what a source still filling looks like.
			report.Pending = append(report.Pending, reference.String())
		case err != nil:
			report.Failed = append(report.Failed, reference.String())
		case written:
			report.Written++
		default:
			// Already there with the same digest. It counts towards completeness: the
			// point is what the storage HOLDS, not what this particular run moved.
			report.Written++
			report.Skipped++
		}

		if c.OnProgress != nil {
			c.OnProgress(report.Written, total)
		}
	}

	return report, nil
}

func (c *Copier) copyOne(
	ctx context.Context, sourcePuller, destinationPuller *remote.Puller, pusher *remote.Pusher,
	source name.Reference,
) (bool, error) {
	destinationRegistry, err := c.registry(c.Destination)
	if err != nil {
		return false, err
	}

	// By whatever the source names it: a release declares its images by digest, while a
	// replica's catalogue lists tags. Copying a digest under a tag would invent a tag the
	// upstream never published, and the cluster refers to those images by digest anyway.
	repository := destinationRegistry.Repo(c.rewriteRepository(source.Context().RepositoryStr()))
	destination, err := sameKind(repository, source)
	if err != nil {
		return false, err
	}

	sourceDescriptor, err := sourcePuller.Get(ctx, source)
	if err != nil {
		return false, fmt.Errorf("reading the source manifest of %s: %w", source, err)
	}

	present, err := manifestPresent(ctx, destinationPuller, destination, sourceDescriptor.Digest)
	if err != nil {
		return false, err
	}
	// Present is not the same as servable — see StoreDir. A manifest whose layers are missing is
	// re-copied, which is the only way a store that was filled by proxying ever becomes complete.
	if present && c.servable(sourceDescriptor.Digest.String()) {
		return false, nil
	}

	if err := pusher.Push(ctx, destination, sourceDescriptor); err != nil {
		return false, fmt.Errorf("writing %s: %w", destination, err)
	}
	return true, nil
}

// rewriteRepository maps a source repository onto the destination prefix, so the
// cluster refers to images under a fixed prefix no matter where they came from.
func (c *Copier) rewriteRepository(repository string) string {
	trimmed := repository
	if c.Source.Repository != "" {
		switch {
		case repository == c.Source.Repository:
			trimmed = ""
		case len(repository) > len(c.Source.Repository) &&
			repository[:len(c.Source.Repository)+1] == c.Source.Repository+"/":
			trimmed = repository[len(c.Source.Repository)+1:]
		}
	}

	switch {
	case c.Destination.Repository == "":
		return trimmed
	case trimmed == "":
		return c.Destination.Repository
	default:
		return c.Destination.Repository + "/" + trimmed
	}
}

func (c *Copier) registry(registry Registry) (name.Registry, error) {
	return parseRegistry(registry)
}

// servable reports whether the destination store can actually hand this image over.
//
// Read from the destination's own filesystem rather than asked of the registry, because the registry
// answers about the manifest and the question is about the blobs it names. Without a StoreDir there
// is nothing to read, and the answer is the old one — present means done.
//
// Not cached across a run on purpose: a fill writes as it goes, so an answer from the start of the
// pass would be stale by the end of it. Within one image the layer lookups share the blobs cache.
func (c *Copier) servable(digest string) bool {
	if c.StoreDir == "" {
		return true
	}

	complete, err := newBlobs(c.StoreDir).complete(digest, 0)
	if err != nil {
		// Unreadable is not "servable": copying again is cheap and wrong-but-quiet is not.
		return false
	}
	return complete
}

// sameKind names a reference in the destination the way the source names it.
func sameKind(repository name.Repository, source name.Reference) (name.Reference, error) {
	switch reference := source.(type) {
	case name.Digest:
		return repository.Digest(reference.DigestStr()), nil
	case name.Tag:
		return repository.Tag(reference.TagStr()), nil
	default:
		return nil, fmt.Errorf("%s is neither a tag nor a digest", source)
	}
}

// manifestPresent reports whether the destination already holds this exact digest.
// absentAtSource reports whether the source simply does not have this reference.
//
// Only 404: a refusal (401/403) is a real failure, and treating it as "not yet" would turn a
// credential problem into an endless quiet wait — which is exactly the shape of failure this store
// has produced before.
func absentAtSource(err error) bool {
	var transportError *transport.Error
	if !errors.As(err, &transportError) {
		return false
	}
	return transportError.StatusCode == http.StatusNotFound
}

func manifestPresent(
	ctx context.Context, puller *remote.Puller, reference name.Reference, digest v1.Hash,
) (bool, error) {
	descriptor, err := puller.Get(ctx, reference)
	if err == nil {
		return descriptor.Digest == digest, nil
	}

	var transportError *transport.Error
	if errors.As(err, &transportError) {
		switch transportError.StatusCode {
		case http.StatusNotFound, http.StatusForbidden:
			// Many registries create a repository on first push, so a missing one
			// cannot be told apart from a missing image, and both mean "copy it".
			return false, nil
		}
	}
	return false, fmt.Errorf("reading the destination manifest of %s: %w", reference, err)
}
