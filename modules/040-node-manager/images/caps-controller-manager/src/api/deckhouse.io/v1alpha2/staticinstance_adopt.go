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

package v1alpha2

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveAdoption reports what has to happen to the node behind spec.address: whether it is
// bootstrapped as usual, adopted as-is, or neither.
//
// Adoption is requested either imperatively, with SkipBootstrapPhaseAnnotation (adopt
// unconditionally), or declaratively, with AdoptIfNodeExistsAnnotation (adopt only if a Node
// with the same address is already part of the cluster and is fit to be adopted, see
// adoptionDecisionForNode). Without either annotation the instance is bootstrapped as usual.
//
// The declarative form deliberately re-checks cluster state on every call instead of relying
// on a stored decision: skipping the bootstrap of a node that is not actually configured
// would leave that node unusable.
func (r *StaticInstance) ResolveAdoption(ctx context.Context, cli client.Reader, nodeGroup string) (AdoptDecision, error) {
	if _, ok := r.Annotations[SkipBootstrapPhaseAnnotation]; ok {
		return AdoptDecision{
			Action:  AdoptActionAdopt,
			Reason:  AdoptReasonRequestedImperatively,
			Message: fmt.Sprintf("Adoption is requested with the %s annotation", SkipBootstrapPhaseAnnotation),
		}, nil
	}

	if _, ok := r.Annotations[AdoptIfNodeExistsAnnotation]; !ok {
		return AdoptDecision{
			Action:  AdoptActionBootstrap,
			Reason:  AdoptReasonNotRequested,
			Message: "Adoption is not requested",
		}, nil
	}

	node, err := r.FindNodeWithSameAddress(ctx, cli)
	if err != nil {
		return AdoptDecision{}, err
	}

	if node == nil {
		return AdoptDecision{
			Action:  AdoptActionBootstrap,
			Reason:  AdoptReasonNoNodeWithAddress,
			Message: fmt.Sprintf("No Node with address %q is part of the cluster", r.Spec.Address),
		}, nil
	}

	return adoptionDecisionForNode(node, nodeGroup), nil
}

// FindNodeWithSameAddress returns the Node that is already part of the cluster and shares an
// address with spec.address, or nil if there is no such Node.
func (r *StaticInstance) FindNodeWithSameAddress(ctx context.Context, cli client.Reader) (*corev1.Node, error) {
	if r.Spec.Address == "" {
		return nil, nil
	}

	nodes := &corev1.NodeList{}
	if err := cli.List(ctx, nodes); err != nil {
		return nil, fmt.Errorf("failed to list cluster nodes: %w", err)
	}

	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Address == r.Spec.Address {
				return &node, nil
			}
		}
	}

	return nil, nil
}

// adoptionRequested reports whether any of the adoption annotations is set. It only looks at
// the annotations, never at the cluster state.
func (r *StaticInstance) adoptionRequested() bool {
	if _, ok := r.Annotations[SkipBootstrapPhaseAnnotation]; ok {
		return true
	}

	_, ok := r.Annotations[AdoptIfNodeExistsAnnotation]

	return ok
}
