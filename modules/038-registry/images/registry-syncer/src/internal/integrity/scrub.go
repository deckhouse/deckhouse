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
	"log/slog"
	"time"

	"golang.org/x/time/rate"
)

// Defaults of the scrub.
const (
	// DefaultScrubCycle is how often the scrub means to read the whole store. An hour, so that a blob
	// gone bad is found before many pulls have failed on it.
	DefaultScrubCycle = time.Hour

	// DefaultScrubMinRate is the slowest the scrub reads, so that a small store is read in minutes and
	// the rest of the hour is idle rather than spread thin.
	DefaultScrubMinRate = 1 << 20

	// DefaultScrubMaxRate is the fastest the scrub reads. A store larger than this reads in an hour is
	// read in longer: 16 MiB/s is a small share of what a master's disk gives, and the disk under the
	// store is often the one etcd writes to.
	DefaultScrubMaxRate = 16 << 20
)

// scrubRetryDelay is how long after finding a blob broken the scrub reads it again before it believes
// it: a Repair may have been replacing it at that moment.
const scrubRetryDelay = 5 * time.Second

// Scrub reads the whole store over and over in the background, at the pace that reads it once per
// Cycle, and records what does not match its digest.
//
// Only reads, and takes nothing from anyone: a pull, a fill and a collection go on beside it. A blob
// removed under it is skipped, and one replaced under it is read again before it is called broken.
type Scrub struct {
	Log *slog.Logger

	// Root is the store's directory.
	Root string

	// Ledger is where broken blobs are recorded, and forgotten when found whole or gone.
	Ledger *Ledger

	// Cycle, MinRate and MaxRate pace the reading — see the defaults. Zero means the default.
	Cycle   time.Duration
	MinRate int64
	MaxRate int64

	// Read is told how many bytes were read, for the metrics. Optional.
	Read func(bytes int64)

	// Finished is told when a pass over the whole store finishes, with what it read. Optional.
	Finished func(ctx context.Context, at time.Time, blobs int, bytes int64)

	// Now and Sleep are the clock, so a test does not wait.
	Now   func() time.Time
	Sleep func(ctx context.Context, d time.Duration) bool
}

func (s *Scrub) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

func (s *Scrub) sleep(ctx context.Context, d time.Duration) bool {
	if s.Sleep != nil {
		return s.Sleep(ctx, d)
	}
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func orDefault[T int64 | time.Duration](value, fallback T) T {
	if value <= 0 {
		return fallback
	}
	return value
}

// Pace is how fast a pass reads a store of total bytes: what reads it in cycle, kept between the two
// bounds.
func Pace(total int64, cycle time.Duration, minRate, maxRate int64) int64 {
	rate := int64(float64(total) / cycle.Seconds())
	return min(max(rate, minRate), maxRate)
}

// Run scrubs until the context ends.
func (s *Scrub) Run(ctx context.Context) {
	cycle := orDefault(s.Cycle, DefaultScrubCycle)
	for ctx.Err() == nil {
		started := s.now()
		if !s.pass(ctx, cycle) {
			// The store could not be listed: tried again after a while, not in a loop.
			if !s.sleep(ctx, time.Minute) {
				return
			}
			continue
		}
		// A pass that read faster than the cycle — a small store, at the lowest rate — waits out the
		// rest of it.
		if rest := cycle - s.now().Sub(started); rest > 0 && !s.sleep(ctx, rest) {
			return
		}
	}
}

// pass reads the whole store once. False when the store could not be listed.
func (s *Scrub) pass(ctx context.Context, cycle time.Duration) bool {
	blobs, err := WalkBlobs(s.Root)
	if err != nil {
		s.Log.Warn("the scrub cannot list the store, so it waits", "error", err.Error())
		return false
	}

	var total int64
	held := make(map[string]bool, len(blobs))
	for _, blob := range blobs {
		total += blob.Size
		held[blob.Digest] = true
	}
	pace := Pace(total, cycle, orDefault(s.MinRate, DefaultScrubMinRate), orDefault(s.MaxRate, DefaultScrubMaxRate))
	limiter := Limiter(pace)
	s.Log.Debug("the scrub starts a pass", "blobs", len(blobs), "bytes", total, "bytes_per_second", pace)

	var read int64
	for _, blob := range blobs {
		if ctx.Err() != nil {
			return true
		}
		outcome, n, err := Check(ctx, s.Root, blob.Digest, limiter)
		read += n
		if s.Read != nil {
			s.Read(n)
		}
		if err != nil {
			// The scrub's context has no deadline, so its pace never refuses a read: an error is the
			// disk's or the context's end.
			if ctx.Err() == nil {
				s.Log.Warn("the scrub cannot read a blob", "digest", blob.Digest, "error", err.Error())
			}
			continue
		}
		switch outcome {
		case Intact:
			if s.Ledger.Remove(blob.Digest) {
				s.Log.Info("a blob recorded as broken is whole again", "digest", blob.Digest)
			}
		case NotHeld:
			delete(held, blob.Digest)
			s.Ledger.Remove(blob.Digest)
		case Corrupted:
			s.confirm(ctx, blob.Digest, limiter)
		}
	}

	// What the store no longer holds is not broken any more: a collection took it.
	s.Ledger.Retain(func(digest string) bool { return held[digest] })

	if s.Finished != nil && ctx.Err() == nil {
		s.Finished(ctx, s.now(), len(blobs), read)
	}
	return true
}

// confirm reads a blob found broken once more after a pause, and records it if it is still broken.
func (s *Scrub) confirm(ctx context.Context, digest string, limiter *rate.Limiter) {
	if !s.sleep(ctx, scrubRetryDelay) {
		return
	}
	outcome, n, err := Check(ctx, s.Root, digest, limiter)
	if s.Read != nil {
		s.Read(n)
	}
	if err != nil || outcome != Corrupted {
		return
	}
	if s.Ledger.Add(digest) {
		s.Log.Warn("a blob of the store does not match its digest; a Repair will replace it", "digest", digest)
	}
}
