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

// Package status publishes what the agent is doing, for people rather than for
// machinery.
//
// Nothing in the pull path reads it, and that is deliberate: any node can write any
// node's status, because RBAC cannot scope a watch by field selector. So this is
// observability, and a failure to publish is never allowed to affect whether the node
// can pull.
package status

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// State is what the agent has to say about itself.
type State struct {
	// ObservedGeneration is the layout generation that was applied.
	ObservedGeneration int64

	// Reconciled reports that the runtime configuration matches the layout.
	Reconciled bool

	// ProxyListening reports that the agent is answering.
	ProxyListening bool

	// ActiveBackends are the backends the agent would currently use.
	ActiveBackends []string

	// FromCache reports that the layout did not come from the API server, but from a
	// copy on the node: the one stored on the last successful read, or the one the node
	// was installed with. Surfaced because "configured" and "running on what it has on
	// disk" look identical from the outside otherwise.
	FromCache bool

	// StaleStorageDataBytes is how much of the in-cluster cache's data is still on this
	// node while no cache is configured.
	StaleStorageDataBytes int64

	// Error is why reconciliation failed, empty when it succeeded.
	Error string
}

// Publisher writes the agent's own status.
type Publisher struct {
	Client client.Client

	// Node is the name of this node, and of its layout object.
	Node string
}

// Publish merges the state into the node's layout status.
//
// A missing object is not an error: the node may not be managed, or the controller may
// not have compiled its layout yet, and an agent that logged a failure every pass over
// that would drown out the ones that matter.
func (p *Publisher) Publish(ctx context.Context, state State) error {
	if p.Node == "" {
		return fmt.Errorf("no node name")
	}
	if p.Client == nil {
		// No way to reach the API, which is exactly when the agent matters most. There
		// is nobody to tell, and saying so on every pass would fill the log of a node
		// that is working.
		return nil
	}

	object := &registryv1alpha1.RegistryNode{}
	if err := p.Client.Get(ctx, types.NamespacedName{Name: p.Node}, object); err != nil {
		return client.IgnoreNotFound(err)
	}

	patch := client.MergeFrom(object.DeepCopy())
	previous := object.Status

	object.Status.ObservedGeneration = state.ObservedGeneration
	object.Status.Reconciled = state.Reconciled
	object.Status.ProxyListening = state.ProxyListening
	object.Status.ActiveBackends = state.ActiveBackends
	object.Status.StaleStorageDataBytes = state.StaleStorageDataBytes

	changed := apimeta.SetStatusCondition(&object.Status.Conditions, condition(state))
	if !changed && equality.Semantic.DeepEqual(previous, object.Status) {
		// Every node writes this object and the controller watches it, so a needless
		// write multiplies into a reconciliation of the whole layout.
		return nil
	}

	return p.Client.Status().Patch(ctx, object, patch)
}

// condition turns the state into the one condition the agent owns.
func condition(state State) metav1.Condition {
	result := metav1.Condition{
		Type:               registryv1alpha1.ConditionReconciled,
		ObservedGeneration: state.ObservedGeneration,
	}

	switch {
	case state.Error != "":
		result.Status = metav1.ConditionFalse
		result.Reason = registryv1alpha1.ReasonInvalidSpec
		result.Message = state.Error
	case state.FromCache:
		// True, because the node IS configured and pulling. The reason is what says the
		// configuration may be behind the cluster's.
		result.Status = metav1.ConditionTrue
		result.Reason = registryv1alpha1.ReasonAppliedFromCache
		result.Message = "The layout came from a copy on the node rather than from the API server; the runtime is configured and may be behind the cluster."
	default:
		result.Status = metav1.ConditionTrue
		result.Reason = registryv1alpha1.ReasonResolved
		result.Message = "The runtime is configured from the current layout."
	}

	return result
}
