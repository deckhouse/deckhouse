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

package images

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"
)

// DefaultEvery is how often the index is rebuilt when nothing asks for it sooner. What it reads is the
// manifests of the store and the size of each blob, which the loop's survey reads every minute too.
const DefaultEvery = 2 * time.Minute

// unknownSetRetry is how soon an index built without the set is built again: the set is read through
// this replica's own registry, which is not listening for a while after a start.
const unknownSetRetry = 30 * time.Second

// Indexer keeps the index of this replica's store and serves it.
type Indexer struct {
	Log *slog.Logger

	// Node this replica runs on.
	Node string

	// Root is the store's directory.
	Root string

	// Set reads what the cluster declares — the loop's own reading of it, so that the index and the
	// count of the set agree.
	Set func(ctx context.Context) (Set, error)

	// Corrupted is the blobs recorded as broken. Nil means none.
	Corrupted func() map[string]bool

	// Every is how often the index is rebuilt. Zero means DefaultEvery.
	Every time.Duration

	// Built is told how long a build took and how many images it found, for the metrics and the
	// status: nil counts when the set was not known, which are not worth publishing over a count that
	// knew it. Optional.
	Built func(ctx context.Context, took time.Duration, counts *registryv1alpha1.ReplicaImages)

	current atomic.Pointer[registryimagev1alpha1.ReplicaIndex]
	kick    chan struct{}
}

// Kick asks for a rebuild now: the store changed, or what is broken in it did.
func (i *Indexer) Kick() {
	select {
	case i.kick <- struct{}{}:
	default:
	}
}

// Run builds the index, and again whenever kicked or every Every, until the context ends.
func (i *Indexer) Run(ctx context.Context) {
	every := i.Every
	if every <= 0 {
		every = DefaultEvery
	}
	for ctx.Err() == nil {
		wait := every
		if index := i.build(ctx); index != nil && !index.SetKnown {
			wait = min(every, unknownSetRetry)
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-i.kick:
			timer.Stop()
		case <-timer.C:
		}
	}
}

// NewIndexer is an Indexer ready to Run. Fields are set on the result.
func NewIndexer() *Indexer {
	return &Indexer{kick: make(chan struct{}, 1)}
}

func (i *Indexer) build(ctx context.Context) *registryimagev1alpha1.ReplicaIndex {
	started := time.Now()

	set, err := i.Set(ctx)
	if err != nil {
		i.Log.Debug("cannot tell what the cluster declares, so the index lists every image outside the set",
			"error", err.Error())
		set = Set{}
	}
	var bad map[string]bool
	if i.Corrupted != nil {
		bad = i.Corrupted()
	}

	images, counts, err := Build(i.Root, set, bad)
	if err != nil {
		i.Log.Warn("cannot index the store; the last index stays", "error", err.Error())
		return i.current.Load()
	}

	now := metav1.NewTime(time.Now().Truncate(time.Second))
	index := &registryimagev1alpha1.ReplicaIndex{
		Node:      i.Node,
		Ready:     true,
		IndexedAt: &now,
		SetKnown:  set.Known,
		Images:    images,
	}
	i.current.Store(index)

	took := time.Since(started)
	i.Log.Debug("indexed the store", "images", len(images), "took", took.String())
	if i.Built != nil {
		if set.Known {
			i.Built(ctx, took, &counts)
		} else {
			i.Built(ctx, took, nil)
		}
	}
	return index
}

// Current is the last index, nil before the first.
func (i *Indexer) Current() *registryimagev1alpha1.ReplicaIndex {
	return i.current.Load()
}

// ServeHTTP answers with the last index, never reading the store.
func (i *Indexer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "only GET", http.StatusMethodNotAllowed)
		return
	}
	index := i.current.Load()
	if index == nil {
		index = &registryimagev1alpha1.ReplicaIndex{Node: i.Node}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(index)
}
