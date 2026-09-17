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

package fencingstate

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

const (
	nodeAPIVersion = "v1"
	nodeKind       = "Node"
)

// States is the agent's half of the FencingFailedNodeState contract: it creates
// the object, records the failed section and removes the object once the peer is
// back. The phase and the conditions belong to fencing-controller, and the
// fallback section to the affected node itself, so nothing here ever writes them.
type States struct {
	api       client.Client
	reader    client.Reader
	nodeGroup string
	profile   v1alpha1.ProfileName
	// apiTimeout bounds a single request. Without it one hung call would stall
	// the reconcile loop and with it every other peer waiting to be reported.
	apiTimeout time.Duration
}

func NewStates(
	api client.Client,
	reader client.Reader,
	nodeGroup string,
	profile v1alpha1.ProfileName,
	apiTimeout time.Duration,
) *States {
	return &States{
		api:        api,
		reader:     reader,
		nodeGroup:  nodeGroup,
		profile:    profile,
		apiTimeout: apiTimeout,
	}
}

// List returns the incident objects of this NodeGroup from the informer cache.
//
// It is bounded like the writes: a controller-runtime cache creates its informer
// on the first read and blocks that read until the informer has synced, so an
// informer that never syncs would otherwise hang the reconcile loop with nothing
// to show for it.
func (s *States) List(ctx context.Context) ([]v1alpha1.FencingFailedNodeState, error) {
	var list v1alpha1.FencingFailedNodeStateList

	ctx, cancel := s.bounded(ctx)
	defer cancel()

	if err := s.reader.List(ctx, &list, client.MatchingLabels{domain.NodeGroupLabel: s.nodeGroup}); err != nil {
		return nil, fmt.Errorf("list fencingfailednodestates: %w", err)
	}

	return list.Items, nil
}

// Create records the object itself: the Node name as its own name, one owner
// reference to that Node for garbage collection and identity, and the immutable
// spec. The status is a separate request, because the API server drops .status
// from a create.
//
// The bool says whether this call is what brought the object into existence, so
// a caller does not announce an incident another agent had already recorded.
func (s *States) Create(ctx context.Context, peer domain.Peer) (bool, error) {
	if peer.UID == "" {
		return false, fmt.Errorf("node %q has no UID, the owner reference would be unusable", peer.Name)
	}

	state := &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name: peer.Name,
			// The label scopes the agent's informer to its own NodeGroup and
			// lets an operator select incidents the same way as Nodes.
			Labels: map[string]string{domain.NodeGroupLabel: s.nodeGroup},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: nodeAPIVersion,
				Kind:       nodeKind,
				Name:       peer.Name,
				UID:        types.UID(peer.UID),
			}},
		},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			NodeGroup:  s.nodeGroup,
			ProfileRef: v1alpha1.ProfileRef{Name: s.profile},
		},
	}

	ctx, cancel := s.bounded(ctx)
	defer cancel()

	if err := s.api.Create(ctx, state); err != nil {
		// Another agent got there first while the two views still differed. Its
		// object is as valid as this one would have been.
		if apierrors.IsAlreadyExists(err) {
			return false, nil
		}

		return false, fmt.Errorf("create fencingfailednodestate %q: %w", peer.Name, err)
	}

	return true, nil
}

// MarkFailed writes the failed section through the status subresource, and only
// if it is still empty: the first detector owns detectedAt, and moving it forward
// would push the evacuation deadline along with it.
//
// The conflict retry is the point of using an update here. It re-reads the
// object, so a phase the controller wrote in the meantime is never rolled back.
//
// The bool says whether this call is what put the section there.
func (s *States) MarkFailed(ctx context.Context, name string, failed v1alpha1.FencingFailedNodeStateFailed) (bool, error) {
	recorded := false

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		recorded = false

		// Every attempt gets its own budget: a conflict means the object moved
		// on, not that the API is slow.
		attemptCtx, cancel := s.bounded(ctx)
		defer cancel()

		var state v1alpha1.FencingFailedNodeState

		if err := s.api.Get(attemptCtx, types.NamespacedName{Name: name}, &state); err != nil {
			return err
		}

		if state.Status.Failed != nil {
			return nil
		}

		state.Status.Failed = failed.DeepCopy()

		if err := s.api.Status().Update(attemptCtx, &state); err != nil {
			return err
		}

		recorded = true

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("record failed state of %q: %w", name, err)
	}

	return recorded, nil
}

// Delete removes the object of a peer that came back. The UID precondition keeps
// a delete that raced a recreated Node from taking the fresh object with it.
func (s *States) Delete(ctx context.Context, name string, uid types.UID) error {
	state := &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: name}}

	ctx, cancel := s.bounded(ctx)
	defer cancel()

	if err := s.api.Delete(ctx, state, client.Preconditions{UID: &uid}); err != nil {
		// Gone is the outcome this asked for; the garbage collector or another
		// agent may have won the race.
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("delete fencingfailednodestate %q: %w", name, err)
	}

	return nil
}

// bounded caps one request. A zero timeout leaves the context alone, so a test
// or a caller that manages its own deadline is not overridden.
func (s *States) bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.apiTimeout <= 0 {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, s.apiTimeout)
}
