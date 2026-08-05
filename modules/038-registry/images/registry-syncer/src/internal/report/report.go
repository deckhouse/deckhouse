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

// Package report publishes what one storage replica actually holds.
//
// Each replica owns exactly one entry in RegistryStorage.status.replicas and
// touches no other, which is what lets several syncers write the same status
// without a coordinator. The controller derives the cluster-wide summary from
// these entries; no replica claims anything about the others, because no replica
// can know.
package report

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// State is what a replica has to say about itself.
type State struct {
	// Node this replica runs on, and the key of its entry.
	Node string

	// Role in replication.
	Role registryv1alpha1.ReplicaRole

	// Full reports that this replica holds the whole expected set.
	//
	// The single most consequential field in the whole status: the controller drops
	// the upstream once the leader reports it. It must therefore be derived from
	// what was written or read, never from what the registry claims it can serve.
	Full bool

	// VerifiedDigests is how many digests were confirmed present.
	VerifiedDigests int32

	// Address is where OTHER replicas reach this one, so a follower can replicate
	// from the leader without resolving a node name.
	Address string

	// Source is the replica this one is filled from. Empty for the leader.
	Source string

	// Error is why the last task failed, and empty when it succeeded. Cleared
	// explicitly on success so a fixed problem stops being reported.
	Error string

	// CollectedAt is when this replica last reclaimed its disk, if it has.
	CollectedAt *time.Time

	// CollectionError is why the last attempt to reclaim did not finish.
	//
	// Separate from Error, which is about filling: a store that cannot be reclaimed still
	// serves every image it holds, so the two say different things about how worried to be.
	CollectionError string
}

// Publisher writes a replica's own entry.
type Publisher struct {
	Client client.Client

	// Name of the RegistryStorage object.
	Name string
}

// Publish merges the replica's state into the status, leaving every other entry
// alone.
//
// A missing storage object is not an error: the controller may not have created it
// yet, or the cache may have just been turned off, and a syncer crash-looping over
// that would be noise rather than information.
func (p *Publisher) Publish(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("a replica report needs the node it came from")
	}

	name := p.Name
	if name == "" {
		name = registryv1alpha1.SingletonName
	}

	storage := &registryv1alpha1.RegistryStorage{}
	if err := p.Client.Get(ctx, types.NamespacedName{Name: name}, storage); err != nil {
		return client.IgnoreNotFound(err)
	}

	patch := client.MergeFrom(storage.DeepCopy())

	if !Merge(&storage.Status.Replicas, state) {
		// Nothing changed. Every replica writes to this object and the controller
		// watches it, so a needless write multiplies into a reconciliation of the
		// whole layout.
		return nil
	}

	return p.Client.Status().Patch(ctx, storage, patch)
}

// PublishCollection records the outcome of a garbage collection.
func (p *Publisher) PublishCollection(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("a collection report needs the node it came from")
	}

	name := p.Name
	if name == "" {
		name = registryv1alpha1.SingletonName
	}

	storage := &registryv1alpha1.RegistryStorage{}
	if err := p.Client.Get(ctx, types.NamespacedName{Name: name}, storage); err != nil {
		return client.IgnoreNotFound(err)
	}

	patch := client.MergeFrom(storage.DeepCopy())

	if !MergeCollection(&storage.Status.Replicas, state) {
		return nil
	}

	return p.Client.Status().Patch(ctx, storage, patch)
}

// Merge replaces this replica's entry in place, or appends it, and reports whether
// anything changed.
//
// Split out from the client work so the merge semantics — the part that must not
// disturb other replicas — are testable on their own.
func Merge(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	entry := registryv1alpha1.StorageReplicaStatus{
		Node:            state.Node,
		Role:            state.Role,
		Full:            state.Full,
		VerifiedDigests: state.VerifiedDigests,
		Address:         state.Address,
		Source:          state.Source,
		Error:           state.Error,
	}

	for i := range *replicas {
		if (*replicas)[i].Node != state.Node {
			continue
		}

		// The garbage collection reports on its own schedule and owns two fields of this
		// entry. Rebuilding the entry from a fill report would erase them, and the two
		// reporters would take turns wiping each other — which looks like a status that
		// flaps for no reason.
		entry.CollectedAt = (*replicas)[i].CollectedAt
		entry.CollectionError = (*replicas)[i].CollectionError

		if equality.Semantic.DeepEqual((*replicas)[i], entry) {
			return false
		}
		(*replicas)[i] = entry
		return true
	}

	*replicas = append(*replicas, entry)
	return true
}

// MergeCollection records the outcome of a garbage collection, and nothing else.
//
// The mirror of the rule above: this reporter owns two fields and must leave every other one
// alone. A replica that had just finished filling would otherwise be reported as empty by the
// next collection.
func MergeCollection(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	collectedAt := (*metav1.Time)(nil)
	if state.CollectedAt != nil {
		stamp := metav1.NewTime(*state.CollectedAt)
		collectedAt = &stamp
	}

	for i := range *replicas {
		if (*replicas)[i].Node != state.Node {
			continue
		}

		if equality.Semantic.DeepEqual((*replicas)[i].CollectedAt, collectedAt) &&
			(*replicas)[i].CollectionError == state.CollectionError {
			return false
		}
		(*replicas)[i].CollectedAt = collectedAt
		(*replicas)[i].CollectionError = state.CollectionError
		return true
	}

	// No entry yet: a replica that collected before it ever reported a fill. The entry is
	// created with what is known, which is the node and the collection.
	*replicas = append(*replicas, registryv1alpha1.StorageReplicaStatus{
		Node:            state.Node,
		Role:            state.Role,
		Address:         state.Address,
		CollectedAt:     collectedAt,
		CollectionError: state.CollectionError,
	})
	return true
}
