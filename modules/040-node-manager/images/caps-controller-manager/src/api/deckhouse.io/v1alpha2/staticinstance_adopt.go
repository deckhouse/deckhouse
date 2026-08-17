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

// ShouldAdopt reports whether the node behind spec.address has to be adopted as-is instead of
// being bootstrapped.
//
// Adoption is requested either imperatively, with SkipBootstrapPhaseAnnotation (adopt
// unconditionally), or declaratively, with AdoptIfNodeExistsAnnotation (adopt only if a Node
// with the same address is already part of the cluster). Without either annotation the
// instance is bootstrapped as usual.
//
// The declarative form deliberately re-checks cluster state on every call instead of relying
// on a stored decision: skipping the bootstrap of a node that is not actually configured
// would leave that node unusable.
func (r *StaticInstance) ShouldAdopt(ctx context.Context, cli client.Reader) (bool, error) {
	if _, ok := r.Annotations[SkipBootstrapPhaseAnnotation]; ok {
		return true, nil
	}

	if _, ok := r.Annotations[AdoptIfNodeExistsAnnotation]; !ok {
		return false, nil
	}

	nodeName, err := r.FindNodeWithSameAddress(ctx, cli)
	if err != nil {
		return false, err
	}

	return nodeName != "", nil
}

// FindNodeWithSameAddress returns the name of a Node that is already part of the cluster and
// shares an address with spec.address, or an empty string if there is no such Node.
func (r *StaticInstance) FindNodeWithSameAddress(ctx context.Context, cli client.Reader) (string, error) {
	if r.Spec.Address == "" {
		return "", nil
	}

	nodes := &corev1.NodeList{}
	if err := cli.List(ctx, nodes); err != nil {
		return "", fmt.Errorf("failed to list cluster nodes: %w", err)
	}

	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Address == r.Spec.Address {
				return node.Name, nil
			}
		}
	}

	return "", nil
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
