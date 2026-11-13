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

package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/distribution"
	"github.com/deckhouse/registry-syncer/internal/images"
	"github.com/deckhouse/registry-syncer/internal/integrity"
	"github.com/deckhouse/registry-syncer/internal/metrics"
	"github.com/deckhouse/registry-syncer/internal/report"
	"github.com/deckhouse/registry-syncer/internal/run"
)

// integrityParts are the pieces that check the store, index its images and say what they found.
type integrityParts struct {
	log       *slog.Logger
	node      string
	ledger    *integrity.Ledger
	indexer   *images.Indexer
	publisher *report.Publisher
	metrics   *metrics.Metrics

	// repairOwed says that an image of the set references a broken blob — see run.Loop.RepairOwed.
	repairOwed func() bool

	// What is published, and the signal that it changed: published from one goroutine, so that the
	// scrub finding a blob broken never waits on the API server.
	mu         sync.Mutex
	images     *registryv1alpha1.ReplicaImages
	scrubbedAt *time.Time
	changed    chan struct{}
}

func newIntegrityParts(
	log *slog.Logger, node string, loop *run.Loop, publisher *report.Publisher, collected *metrics.Metrics,
) *integrityParts {
	parts := &integrityParts{
		log:        log,
		node:       node,
		ledger:     &integrity.Ledger{},
		publisher:  publisher,
		metrics:    collected,
		changed:    make(chan struct{}, 1),
		repairOwed: loop.RepairOwed,
	}

	indexer := images.NewIndexer()
	indexer.Log = log
	indexer.Node = node
	indexer.Root = distribution.DataDir
	indexer.Corrupted = parts.ledger.Snapshot
	indexer.Set = func(ctx context.Context) (images.Set, error) {
		digests, owners, err := loop.DeclaredOwners(ctx)
		if err != nil {
			return images.Set{}, err
		}
		return images.Set{Known: true, Digests: digests, Owners: owners}, nil
	}
	indexer.Built = func(_ context.Context, took time.Duration, counts *registryv1alpha1.ReplicaImages) {
		collected.IndexDuration.Observe(took.Seconds())
		if counts == nil {
			return
		}
		collected.Images.WithLabelValues("present").Set(float64(counts.Present))
		collected.Images.WithLabelValues("missing").Set(float64(counts.Missing))
		collected.Images.WithLabelValues("incomplete").Set(float64(counts.Incomplete))
		collected.Images.WithLabelValues("corrupted").Set(float64(counts.Corrupted))
		collected.Images.WithLabelValues("out_of_set").Set(float64(counts.OutOfSet))
		parts.mu.Lock()
		parts.images = counts
		parts.mu.Unlock()
		parts.signal()
	}
	parts.indexer = indexer

	loop.Corrupted = parts.ledger.Snapshot
	parts.ledger.OnChange(func() {
		collected.CorruptedBlobs.Set(float64(parts.ledger.Len()))
		loop.CorruptionChanged()
		indexer.Kick()
		parts.signal()
	})
	return parts
}

func (p *integrityParts) signal() {
	select {
	case p.changed <- struct{}{}:
	default:
	}
}

// integrityPublishTimeout bounds one write of what the checks found.
const integrityPublishTimeout = 30 * time.Second

// publish writes what changed into this replica's entry of RegistryStorage, until the context ends.
func (p *integrityParts) publish(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.changed:
		}
		p.mu.Lock()
		state := report.Integrity{
			Node:           p.node,
			CorruptedBlobs: p.ledger.List(),
			Images:         p.images.DeepCopy(),
			ScrubbedAt:     p.scrubbedAt,
		}
		p.mu.Unlock()
		// Asked outside the lock: it may read manifests off the disk, and only when a blob is recorded.
		state.ShortOfSet = len(state.CorruptedBlobs) > 0 && p.repairOwed != nil && p.repairOwed()

		write, cancel := context.WithTimeout(ctx, integrityPublishTimeout)
		err := p.publisher.PublishIntegrity(write, state)
		cancel()
		if err != nil && ctx.Err() == nil {
			p.log.Warn("cannot record what the checks of the store found; trying again", "error", err.Error())
			time.AfterFunc(time.Minute, p.signal)
		}
	}
}

// restore takes back the broken blobs this replica recorded before it restarted: the scrub finds them
// again within its cycle, and until then the replica would count them as whole. Tried for a while, as
// the API server may not answer yet when the syncer starts.
func (p *integrityParts) restore(ctx context.Context, reader client.Reader) {
	for attempt := 0; attempt < 30 && ctx.Err() == nil; attempt++ {
		storage := &registryv1alpha1.RegistryStorage{}
		err := reader.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage)
		if err == nil {
			for _, replica := range storage.Status.Replicas {
				if replica.Node == p.node && len(replica.CorruptedBlobs) > 0 {
					p.ledger.Add(replica.CorruptedBlobs...)
					p.log.Info("blobs recorded as broken before the restart are taken as broken until checked",
						"count", len(replica.CorruptedBlobs))
				}
			}
			return
		}
		if client.IgnoreNotFound(err) == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

// start clears what an unfinished repair left, restores what was recorded, then runs the index, the scrub and the publishing of both.
func (p *integrityParts) start(ctx context.Context, reader client.Reader, opts options) {
	// Before any Repair runs: what one left half done goes back or away.
	if restored, err := integrity.Recover(distribution.DataDir); err != nil {
		p.log.Warn("cannot clear what an unfinished repair left", "error", err.Error())
	} else if restored > 0 {
		p.log.Info("broken copies an unfinished repair kept aside are back in the store", "count", restored)
	}
	go p.publish(ctx)
	go func() {
		if reader != nil {
			p.restore(ctx, reader)
		}
		go p.indexer.Run(ctx)
		scrub := &integrity.Scrub{
			Log:     p.log,
			Root:    distribution.DataDir,
			Ledger:  p.ledger,
			Cycle:   opts.scrubCycle,
			MaxRate: int64(opts.scrubMaxRateMiB) << 20,
			Read: func(bytes int64) {
				p.metrics.ScrubbedBytes.WithLabelValues(metrics.ReaderScrub).Add(float64(bytes))
			},
			Finished: func(_ context.Context, at time.Time, blobs int, bytes int64) {
				p.metrics.LastScrub.Set(float64(at.Unix()))
				p.log.Info("the scrub read the whole store", "blobs", blobs, "bytes", bytes)
				p.mu.Lock()
				p.scrubbedAt = &at
				p.mu.Unlock()
				p.signal()
			},
		}
		if opts.scrubCycle <= 0 {
			p.log.Info("the scrub is turned off")
			return
		}
		scrub.Run(ctx)
	}()
}

// verifier and repairer run the operations of those types on this replica.
func (p *integrityParts) verifier(opts options) *integrity.Verifier {
	return &integrity.Verifier{
		Log:    p.log,
		Root:   distribution.DataDir,
		Ledger: p.ledger,
		Rate:   int64(opts.verifyRateMiB) << 20,
		Read: func(bytes int64) {
			p.metrics.ScrubbedBytes.WithLabelValues(metrics.ReaderVerify).Add(float64(bytes))
		},
	}
}

func (p *integrityParts) repairer(opts options, loop *run.Loop) *integrity.Repairer {
	return &integrity.Repairer{
		Log:     p.log,
		Root:    distribution.DataDir,
		Ledger:  p.ledger,
		Sources: loop.RepairSources,
		Rate:    int64(opts.verifyRateMiB) << 20,
		Read: func(bytes int64) {
			p.metrics.ScrubbedBytes.WithLabelValues(metrics.ReaderRepair).Add(float64(bytes))
		},
	}
}
