/*
Copyright 2025 Flant JSC

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

package update

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/deckhouse/node-controller/internal/drain"
)

// DrainOrchestrator encapsulates the drain workflow for a node during update.
//
// The drain process follows this sequence:
//  1. Cordon the node (set spec.unschedulable = true).
//  2. Evict all non-DaemonSet, non-completed pods from the node.
//  3. Wait for eviction to complete (with timeout from NodeGroup spec).
//  4. Mark the node as drained by swapping draining -> drained annotation.
type DrainOrchestrator struct {
	client  client.Client
	drainer *drain.Drainer
}

// NewDrainOrchestrator creates a new DrainOrchestrator.
func NewDrainOrchestrator(c client.Client) *DrainOrchestrator {
	return &DrainOrchestrator{
		client: c,
		drainer: &drain.Drainer{
			Client: c,
		},
	}
}

// DrainAndMark performs the full drain workflow:
// cordon -> evict -> wait for eviction -> mark drained.
func (o *DrainOrchestrator) DrainAndMark(ctx context.Context, node *corev1.Node, drainTimeout time.Duration) error {
	log := logf.FromContext(ctx)

	// Step 1: Cordon the node and evict pods.
	log.Info("starting drain", "node", node.Name)
	if err := o.drainer.DrainNode(ctx, node); err != nil {
		return fmt.Errorf("drain node %s: %w", node.Name, err)
	}

	// Step 2: Wait for pod eviction to complete.
	log.Info("waiting for eviction to complete", "node", node.Name, "timeout", drainTimeout)
	if err := o.drainer.WaitForEviction(ctx, node, drainTimeout); err != nil {
		// Timeout is not fatal — proceed with marking drained, matching original hook behavior.
		log.Error(err, "eviction wait timed out, proceeding anyway", "node", node.Name)
	}

	// Step 3: Mark node as drained.
	return o.markDrained(ctx, node)
}

// markDrained swaps the draining annotation to drained on the node.
func (o *DrainOrchestrator) markDrained(ctx context.Context, node *corev1.Node) error {
	// Re-read the node to avoid conflicts.
	fresh := &corev1.Node{}
	if err := o.client.Get(ctx, client.ObjectKeyFromObject(node), fresh); err != nil {
		return fmt.Errorf("get fresh node %s: %w", node.Name, err)
	}

	patch := client.MergeFrom(fresh.DeepCopy())

	source := fresh.Annotations[annotationDraining]
	delete(fresh.Annotations, annotationDraining)
	if fresh.Annotations == nil {
		fresh.Annotations = make(map[string]string)
	}
	fresh.Annotations[annotationDrained] = source

	if err := o.client.Patch(ctx, fresh, patch); err != nil {
		return fmt.Errorf("patch node %s drained annotation: %w", node.Name, err)
	}

	return nil
}

// RemoveObsoleteDrainedAnnotation removes the drained annotation from a node
// that has become schedulable again with a user-initiated drained annotation.
// This mirrors the cleanup logic from the original handle_draining.go hook.
func (o *DrainOrchestrator) RemoveObsoleteDrainedAnnotation(ctx context.Context, node *corev1.Node) error {
	log := logf.FromContext(ctx)

	if node.Spec.Unschedulable {
		return nil
	}

	drainedSource, ok := node.Annotations[annotationDrained]
	if !ok || drainedSource != "user" {
		return nil
	}

	log.Info("removing obsolete drained annotation from schedulable node", "node", node.Name)

	patch := client.MergeFrom(node.DeepCopy())
	delete(node.Annotations, annotationDrained)

	return o.client.Patch(ctx, node, patch)
}

// isNodeBeingDrained returns true if the node has an active draining annotation.
func isNodeBeingDrained(node *corev1.Node) bool {
	if node.Annotations == nil {
		return false
	}
	v, ok := node.Annotations[annotationDraining]
	return ok && v != ""
}

// isNodeDrained returns true if the node has a drained annotation.
func isNodeDrained(node *corev1.Node) bool {
	if node.Annotations == nil {
		return false
	}
	_, ok := node.Annotations[annotationDrained]
	return ok
}

// isDrainingByBashible returns true if the node is being drained by bashible specifically.
func isDrainingByBashible(node *corev1.Node) bool {
	if node.Annotations == nil {
		return false
	}
	return node.Annotations[annotationDraining] == drainingSourceBashible
}
