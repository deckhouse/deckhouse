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

package integrity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/time/rate"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/report"
)

// Source is somewhere a whole copy of a blob can be asked for.
type Source struct {
	// Name says which, in a message: "replica master-1", "the upstream".
	Name string

	// Registry is the serving listener asked, with its credentials. Its Repository is not used: a
	// blob is asked for in the repository of this store that links it, and every replica lays out its
	// store the same way.
	Registry fill.Registry

	// Quarantine says the copy here has to be moved aside first. It is this replica's own serving
	// listener: a pull-through cache, which answers from its store while the store holds the blob and
	// fetches it from the upstream only once the store does not.
	Quarantine bool
}

// Repairer runs a Repair on this replica's store.
//
// Nothing is deleted to repair it. A broken blob is replaced by a whole one: downloaded beside the
// store's blobs, checked against its digest, and renamed over the broken one, so that no moment
// passes in which the store holds neither. Removing the image instead would repair nothing: the
// broken blob is usually a layer other images share, it stays, and a fill finds it present and
// copies nothing.
type Repairer struct {
	Log *slog.Logger

	// Root is the store's directory.
	Root string

	// Ledger forgets what is repaired or found whole, and records what is found broken and cannot be.
	Ledger *Ledger

	// Sources are where whole copies are asked for, in order.
	Sources func(ctx context.Context) ([]Source, error)

	// Rate is how fast the store is read to check it. Zero means DefaultVerifyRate.
	Rate int64

	// Read is told how many bytes were read, for the metrics. Optional.
	Read func(bytes int64)
}

// Repair checks the blobs of the given images or blobs, and replaces those that are broken or missing.
// Fails only when the store cannot be read or the context ends; a digest it could not repair is
// reported as Failed in the result.
func (r *Repairer) Repair(ctx context.Context, digests []string) (registryv1alpha1.RepairResult, error) {
	var result registryv1alpha1.RepairResult

	targets, err := Resolve(r.Root, digests)
	if err != nil {
		return result, fmt.Errorf("finding the digests in the store: %w", err)
	}

	var sources []Source
	sourcesKnown := false
	sourcesOf := func() ([]Source, error) {
		if sourcesKnown {
			return sources, nil
		}
		found, err := r.Sources(ctx)
		if err != nil {
			return nil, err
		}
		sources, sourcesKnown = found, true
		return sources, nil
	}

	rateLimit := r.Rate
	if rateLimit <= 0 {
		rateLimit = DefaultVerifyRate
	}
	limiter := Limiter(rateLimit)

	for _, target := range targets {
		entry, err := r.repair(ctx, target, sourcesOf, limiter)
		if err != nil {
			return result, err
		}
		entry.Message = report.Clip(entry.Message)
		result.Digests = append(result.Digests, entry)
	}
	return result, nil
}

// repairRounds bounds how often an image is checked again after a manifest of it was replaced: a
// manifest fetched where there was none names blobs nobody had looked at.
const repairRounds = 3

func (r *Repairer) repair(
	ctx context.Context, target Target, sourcesOf func() ([]Source, error), limiter *rate.Limiter,
) (registryv1alpha1.DigestRepair, error) {
	entry := registryv1alpha1.DigestRepair{Digest: target.Digest}
	if len(target.Blobs) == 0 {
		entry.Outcome = registryv1alpha1.DigestNotHeld
		entry.Message = "This replica holds neither an image nor a blob by this digest."
		return entry, nil
	}

	var (
		repaired int32
		from     = map[string]bool{}
		failures []string
	)
	for round := 0; round < repairRounds; round++ {
		var broken []string
		for _, blob := range target.Blobs {
			outcome, read, err := Check(ctx, r.Root, blob, limiter)
			if r.Read != nil {
				r.Read(read)
			}
			if err != nil {
				return entry, err
			}
			switch outcome {
			case Intact:
				r.Ledger.Remove(blob)
			case Corrupted, NotHeld:
				broken = append(broken, blob)
			}
		}
		if len(broken) == 0 {
			break
		}
		if target.Repository == "" {
			r.Ledger.Add(onlyHeld(r.Root, broken)...)
			failures = append(failures, "no image of this store names it, so there is no repository to ask for a whole copy in")
			break
		}
		sources, err := sourcesOf()
		if err != nil {
			failures = append(failures, fmt.Sprintf("cannot tell where to take whole copies from: %v", err))
			break
		}

		manifestReplaced := false
		for _, blob := range broken {
			source, err := r.replace(ctx, sources, target.Repository, blob, target.Manifests[blob])
			if err != nil {
				if ctx.Err() != nil {
					return entry, ctx.Err()
				}
				r.Ledger.Add(onlyHeld(r.Root, []string{blob})...)
				failures = append(failures, fmt.Sprintf("%s: %v", blob, err))
				continue
			}
			r.Log.Info("replaced a blob with a whole copy", "digest", blob, "image", target.Digest, "from", source)
			repaired++
			from[source] = true
			manifestReplaced = manifestReplaced || target.Manifests[blob]
		}
		if len(failures) > 0 || !manifestReplaced || !target.Manifests[target.Digest] {
			break
		}
		// A manifest came back: what it names is checked in the next round.
		blobs, manifests, err := Closure(r.Root, target.Digest)
		if err != nil {
			return entry, err
		}
		target.Blobs, target.Manifests = blobs, manifests
	}

	entry.RepairedBlobs = repaired
	switch {
	case len(failures) > 0:
		entry.Outcome = registryv1alpha1.DigestFailed
		entry.Message = strings.Join(failures, "; ")
	case repaired > 0:
		entry.Outcome = registryv1alpha1.DigestRepaired
		names := make([]string, 0, len(from))
		for source := range from {
			names = append(names, source)
		}
		entry.Message = "Whole copies were taken from " + strings.Join(names, ", ") + "."
	default:
		entry.Outcome = registryv1alpha1.DigestIntact
		entry.Message = "Every blob matches its digest."
	}
	return entry, nil
}

// onlyHeld keeps the blobs the store holds content for: one that is missing is not broken, the
// ledger is for what serves wrong content.
func onlyHeld(root string, blobs []string) []string {
	var held []string
	for _, blob := range blobs {
		if path, ok := BlobPath(root, blob); ok {
			if _, err := os.Stat(path); err == nil {
				held = append(held, blob)
			}
		}
	}
	return held
}

// replace puts a whole copy of the blob where the store keeps it, from the first source that has one.
// Returns the name of that source.
func (r *Repairer) replace(
	ctx context.Context, sources []Source, repository, digest string, manifest bool,
) (string, error) {
	if len(sources) == 0 {
		return "", errors.New("there is no other replica and no upstream to take a whole copy from")
	}
	var failures []string
	for _, source := range sources {
		restore, discard := func() {}, func() {}
		if source.Quarantine {
			var err error
			if restore, discard, err = r.quarantine(digest); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", source.Name, err))
				continue
			}
		}
		if err := r.download(ctx, source, repository, digest, manifest); err != nil {
			// The broken copy goes back: a blob that is gone reads as missing, not broken, and the
			// finding, the alert and the next Repair would go with it.
			restore()
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			failures = append(failures, fmt.Sprintf("%s: %v", source.Name, err))
			continue
		}
		discard()
		r.Ledger.Remove(digest)
		return source.Name, nil
	}
	return "", errors.New(strings.Join(failures, "; "))
}

// repairDir is where whole copies are written before they replace the broken ones: inside the store,
// so that the rename is one filesystem's, and outside blobs and repositories, which the registry and
// its collection walk.
func repairDir(root string) string {
	return filepath.Join(root, "docker", "registry", "v2", "repair")
}

// quarantine moves the store's copy of a blob out of the way, so that this replica's own listener
// fetches the blob from the upstream instead of serving it.
//
// Kept aside until the whole copy is in place: restore puts it back when no whole copy came, discard
// removes it once one did. Removed at once, a Repair whose upstream did not answer left the store without
// the blob at all — no longer broken, so no longer recorded, alerted on or repaired.
func (r *Repairer) quarantine(digest string) (func(), func(), error) {
	path, ok := BlobPath(r.Root, digest)
	if !ok {
		return nil, nil, fmt.Errorf("not a digest: %q", digest)
	}
	if err := os.MkdirAll(repairDir(r.Root), 0o755); err != nil {
		return nil, nil, fmt.Errorf("making room for the copy: %w", err)
	}
	aside := filepath.Join(repairDir(r.Root), brokenPrefix+strings.ReplaceAll(digest, ":", "-"))
	if err := os.Rename(path, aside); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return func() {}, func() {}, nil
		}
		return nil, nil, fmt.Errorf("moving the broken copy aside: %w", err)
	}
	restore := func() { putBack(aside, path) }
	discard := func() { _ = os.Remove(aside) }
	return restore, discard, nil
}

// putBack returns a broken copy kept aside to where the store keeps the blob, only into an empty place:
// a copy that arrived meanwhile is whole, and wins. A link, not a rename, so that it never replaces one.
func putBack(aside, path string) bool {
	restored := os.Link(aside, path) == nil
	_ = os.Remove(aside)
	return restored
}

// brokenPrefix names a broken copy kept aside in the repair directory.
const brokenPrefix = "broken-"

// Recover clears the repair directory of what a Repair the syncer did not live to finish left there:
// a broken copy kept aside goes back where the store keeps the blob, unless a whole one is there, and
// a download that never finished is removed. Without it a restart between the two left the store
// without the blob — no longer broken, so no longer recorded, alerted on or repaired — and the files
// there for good. Runs before any Repair does. Returns how many copies went back.
func Recover(root string) (int, error) {
	dir := repairDir(root)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", dir, err)
	}
	restored := 0
	for _, entry := range entries {
		file := filepath.Join(dir, entry.Name())
		if encoded, found := strings.CutPrefix(entry.Name(), brokenPrefix); found {
			algorithm, hexDigest, _ := strings.Cut(encoded, "-")
			if path, ok := BlobPath(root, algorithm+":"+hexDigest); ok && putBack(file, path) {
				restored++
				continue
			}
		}
		_ = os.RemoveAll(file)
	}
	return restored, nil
}

// download asks the source for the blob, writes it beside the store, checks it against its digest
// and renames it into place.
func (r *Repairer) download(ctx context.Context, source Source, repository, digest string, manifest bool) error {
	var nameOptions []name.Option
	if source.Registry.Insecure {
		nameOptions = append(nameOptions, name.Insecure)
	}
	registry, err := name.NewRegistry(source.Registry.Address, nameOptions...)
	if err != nil {
		return fmt.Errorf("parsing the address %q: %w", source.Registry.Address, err)
	}
	reference := registry.Repo(repository).Digest(digest)
	options := append(append([]remote.Option{}, source.Registry.Options...), remote.WithContext(ctx))

	content, err := open(reference, manifest, options)
	if err != nil {
		return err
	}
	defer func() { _ = content.Close() }()

	if err := os.MkdirAll(repairDir(r.Root), 0o755); err != nil {
		return fmt.Errorf("making room for the copy: %w", err)
	}
	file, err := os.CreateTemp(repairDir(r.Root), strings.ReplaceAll(digest, ":", "-")+"-*")
	if err != nil {
		return fmt.Errorf("creating the copy: %w", err)
	}
	written := file.Name()
	defer func() { _ = os.Remove(written) }()

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, hash), content); err != nil {
		_ = file.Close()
		return fmt.Errorf("downloading: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("writing the copy: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("writing the copy: %w", err)
	}

	algorithm, expected, _ := strings.Cut(digest, ":")
	if algorithm != "sha256" || hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("the copy does not match the digest either")
	}

	path, ok := BlobPath(r.Root, digest)
	if !ok {
		return fmt.Errorf("not a digest: %q", digest)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("making the blob's directory: %w", err)
	}
	if err := os.Chmod(written, 0o644); err != nil {
		return fmt.Errorf("opening the copy to the registry: %w", err)
	}
	if err := os.Rename(written, path); err != nil {
		return fmt.Errorf("putting the copy in place: %w", err)
	}
	return nil
}

// open asks a registry for a blob, or for a manifest by its digest: a manifest is a blob of the store,
// and a registry serves it on the manifests endpoint. A blob the blobs endpoint does not know is asked
// for as a manifest too — the manifest of an image in an index that has no revision of its own.
func open(reference name.Digest, manifest bool, options []remote.Option) (io.ReadCloser, error) {
	if !manifest {
		layer, err := remote.Layer(reference, options...)
		if err == nil {
			content, err := layer.Compressed()
			if err == nil {
				return content, nil
			}
		}
	}
	descriptor, err := remote.Get(reference, options...)
	if err != nil {
		return nil, fmt.Errorf("asking for %s: %w", reference, err)
	}
	return io.NopCloser(strings.NewReader(string(descriptor.Manifest))), nil
}
