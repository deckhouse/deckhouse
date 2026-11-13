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
	"fmt"
	"log/slog"
	"slices"
	"sort"

	"golang.org/x/time/rate"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// DefaultVerifyRate is how fast a Verify reads: a store of 15 GB in about eight minutes per replica.
// Faster than the scrub, because somebody asked and is waiting; still a fraction of what a master's
// disk gives, because the disk is often etcd's too.
const DefaultVerifyRate = 32 << 20

// Verifier runs a Verify on this replica's store.
type Verifier struct {
	Log *slog.Logger

	// Root is the store's directory.
	Root string

	// Ledger records what is found broken, and forgets what is found whole.
	Ledger *Ledger

	// Rate is how fast the store is read, in bytes per second. Zero means DefaultVerifyRate.
	Rate int64

	// Read is told how many bytes were read, for the metrics. Optional.
	Read func(bytes int64)
}

// Verify reads the blobs of the given images or blobs — every blob of the store when none is given —
// and compares each with its digest. Fails only when the store cannot be read, or the context ends.
func (v *Verifier) Verify(ctx context.Context, digests []string) (registryv1alpha1.VerifyResult, error) {
	var result registryv1alpha1.VerifyResult

	var blobs []string
	if len(digests) == 0 {
		all, err := WalkBlobs(v.Root)
		if err != nil {
			return result, err
		}
		for _, blob := range all {
			blobs = append(blobs, blob.Digest)
		}
	} else {
		targets, err := Resolve(v.Root, digests)
		if err != nil {
			return result, fmt.Errorf("finding the digests in the store: %w", err)
		}
		seen := map[string]bool{}
		for _, target := range targets {
			if len(target.Blobs) == 0 {
				result.MissingDigests = append(result.MissingDigests, target.Digest)
				continue
			}
			for _, blob := range target.Blobs {
				if !seen[blob] {
					seen[blob] = true
					blobs = append(blobs, blob)
				}
			}
		}
	}

	rateLimit := v.Rate
	if rateLimit <= 0 {
		rateLimit = DefaultVerifyRate
	}
	limiter := Limiter(rateLimit)

	var corrupted []string
	for _, blob := range blobs {
		outcome, read, err := v.check(ctx, blob, limiter)
		result.CheckedBytes += read
		if err != nil {
			// What was found before the window closed is found: recorded, and in the result, so that
			// a Verify cut short still asks for the Repair of what it did find.
			v.record(&result, corrupted)
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			return result, err
		}
		switch outcome {
		case Intact:
			result.CheckedBlobs++
			v.Ledger.Remove(blob)
		case Corrupted:
			result.CheckedBlobs++
			corrupted = append(corrupted, blob)
		case NotHeld:
			// A blob an image names and the store does not hold: the image is incomplete, which the
			// listing of the images says. Nothing here is broken.
			v.Ledger.Remove(blob)
		}
	}

	v.record(&result, corrupted)
	return result, nil
}

// record writes the broken blobs found into the ledger and the result.
func (v *Verifier) record(result *registryv1alpha1.VerifyResult, corrupted []string) {
	if len(corrupted) > 0 && v.Ledger.Add(corrupted...) {
		v.Log.Warn("the Verify found blobs that do not match their digests; a Repair will replace them",
			"count", len(corrupted))
	}
	corrupted = slices.Clone(corrupted)
	sort.Strings(corrupted)
	result.CorruptedCount = int32(len(corrupted))
	if len(corrupted) > registryv1alpha1.MaxOperationDigests {
		corrupted = corrupted[:registryv1alpha1.MaxOperationDigests]
	}
	result.CorruptedBlobs = corrupted
}

func (v *Verifier) check(ctx context.Context, blob string, limiter *rate.Limiter) (Outcome, int64, error) {
	outcome, read, err := Check(ctx, v.Root, blob, limiter)
	if v.Read != nil {
		v.Read(read)
	}
	return outcome, read, err
}
