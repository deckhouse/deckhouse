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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/report"
)

// ClusterOperations reads the RegistryOperation objects and writes this replica's entry in them.
//
// Read straight from the API server rather than from an informer: the syncer keeps no cache, and what
// is read is a few small objects every quarter of a minute.
type ClusterOperations struct {
	Client client.Client

	// Node this replica runs on, and the key of its entry.
	Node string
}

var _ Operations = (*ClusterOperations)(nil)

// Next returns the oldest operation this replica still owes.
func (c *ClusterOperations) Next(ctx context.Context) (*Operation, error) {
	list := &registryv1alpha1.RegistryOperationList{}
	if err := c.Client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("listing the registry operations: %w", err)
	}

	var oldest *registryv1alpha1.RegistryOperation
	for i := range list.Items {
		item := &list.Items[i]
		if !c.owes(item) {
			continue
		}
		if oldest == nil || item.CreationTimestamp.Before(&oldest.CreationTimestamp) ||
			(item.CreationTimestamp.Equal(&oldest.CreationTimestamp) && item.Name < oldest.Name) {
			oldest = item
		}
	}
	if oldest == nil {
		return nil, nil
	}
	return toOperation(oldest, c.Node), nil
}

// Get reads an operation again.
func (c *ClusterOperations) Get(ctx context.Context, name string) (*Operation, error) {
	item := &registryv1alpha1.RegistryOperation{}
	if err := c.Client.Get(ctx, types.NamespacedName{Name: name}, item); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading the registry operation %s: %w", name, err)
	}
	if !c.owes(item) {
		return nil, nil
	}
	return toOperation(item, c.Node), nil
}

// Report writes this replica's entry, under an optimistic lock: the other replicas and the controller
// write the same list, and a merge patch replaces it whole.
func (c *ClusterOperations) Report(ctx context.Context, name string, progress Progress) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		item := &registryv1alpha1.RegistryOperation{}
		if err := c.Client.Get(ctx, types.NamespacedName{Name: name}, item); err != nil {
			if apierrors.IsNotFound(err) {
				return ErrNotOwed
			}
			return err
		}
		// An entry the controller has settled — the replica was given up on, or the operation
		// finished — is not reopened by a replica that reports late.
		if !c.owes(item) {
			return ErrNotOwed
		}

		patch := client.MergeFromWithOptions(item.DeepCopy(), client.MergeFromWithOptimisticLock{})
		entry := item.Status.Replica(c.Node)
		before := entry.DeepCopy()
		apply(entry, progress)
		if equality.Semantic.DeepEqual(before, entry) {
			return nil
		}
		return c.Client.Status().Patch(ctx, item, patch)
	})
}

// owes reports whether this replica still has to do something about an operation.
func (c *ClusterOperations) owes(item *registryv1alpha1.RegistryOperation) bool {
	switch item.Spec.Type {
	case registryv1alpha1.OperationGarbageCollection, registryv1alpha1.OperationVerify, registryv1alpha1.OperationRepair:
	default:
		return false
	}
	switch item.Status.Phase {
	case registryv1alpha1.OperationPending, registryv1alpha1.OperationRunning:
	default:
		// Not accepted yet, or finished.
		return false
	}
	entry := item.Status.Replica(c.Node)
	return entry != nil && !entry.Phase.Finished()
}

func toOperation(item *registryv1alpha1.RegistryOperation, node string) *Operation {
	operation := &Operation{
		Name:     item.Name,
		Type:     item.Spec.Type,
		Digests:  append([]string(nil), item.Spec.Digests()...),
		Canceled: item.Canceled(),
	}
	if entry := item.Status.Replica(node); entry != nil {
		operation.PassedGate = entry.Phase == registryv1alpha1.ReplicaOperationRunning ||
			(entry.Phase == registryv1alpha1.ReplicaOperationWaiting && entry.Reason == registryv1alpha1.ReasonOperationTurn)
	}
	if item.Status.Deadline != nil {
		operation.Deadline = item.Status.Deadline.Time
	}
	return operation
}

// apply writes progress into an entry. The times are written as given, so a replica that waits again
// after a restart does not keep the start of the collection it never finished.
func apply(entry *registryv1alpha1.ReplicaOperationStatus, progress Progress) {
	entry.Phase = progress.Phase
	entry.Reason = progress.Reason
	entry.Message = report.Clip(progress.Message)
	entry.StartedAt = stamp(progress.StartedAt)
	entry.FinishedAt = stamp(progress.FinishedAt)

	entry.GarbageCollection = nil
	if report := progress.Report; report != nil {
		result := &registryv1alpha1.GarbageCollectionResult{
			RemovedManifests: int32(report.RemovedManifests),
			RemovedVersions:  append([]string(nil), report.RemovedVersions...),
		}
		if report.RemovedBlobs >= 0 {
			blobs := int32(report.RemovedBlobs)
			result.RemovedBlobs = &blobs
		}
		if report.ReclaimedBytes != nil {
			reclaimed := *report.ReclaimedBytes
			result.ReclaimedBytes = &reclaimed
		}
		entry.GarbageCollection = result
	}
	entry.Verify = progress.Verify.DeepCopy()
	if entry.Verify != nil {
		// The status is read by every replica and the controller; the list is bounded, and so is
		// what a missing digest can add.
		if len(entry.Verify.MissingDigests) > registryv1alpha1.MaxOperationDigests {
			entry.Verify.MissingDigests = entry.Verify.MissingDigests[:registryv1alpha1.MaxOperationDigests]
		}
	}
	entry.Repair = progress.Repair.DeepCopy()
}

// stamp converts a time to what the API server stores: a time it rounds down otherwise compares
// unequal to itself when it is read back, and every write after it looks like a change.
func stamp(t *time.Time) *metav1.Time {
	if t == nil {
		return nil
	}
	converted := metav1.NewTime(*t).Rfc3339Copy()
	return &converted
}
