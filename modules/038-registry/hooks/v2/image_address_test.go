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

package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func TestImageAddressSwitched(t *testing.T) {
	cases := []struct {
		name  string
		state imageAddressState
		// switched is whether image references may name the in-cluster registry.
		switched bool
		// blocked is a fragment of the reason, checked when not switched.
		blocked string
	}{{
		// A cluster that has just started managing its pull path. The controller has
		// written nothing yet, so no node can resolve the in-cluster address.
		name:     "nothing has happened yet",
		state:    imageAddressState{},
		switched: false,
		blocked:  "no nodes are known yet",
	}, {
		// The race a count of reconciled layouts alone would miss. One node is configured
		// and reports so; the other two have no layout at all and would be left with
		// image references naming an address their runtime cannot resolve.
		name:     "one node of three configured, and that one reports success",
		state:    imageAddressState{Nodes: 3, Layouts: 1, Applied: 1},
		switched: false,
		blocked:  "written 1 layouts for 3 nodes",
	}, {
		name:     "every node has a layout, not all of them applied",
		state:    imageAddressState{Nodes: 3, Layouts: 3, Applied: 2},
		switched: false,
		blocked:  "2 of 3 nodes have applied",
	}, {
		name:     "every node is applying the layout it was given",
		state:    imageAddressState{Nodes: 3, Layouts: 3, Applied: 3},
		switched: true,
	}, {
		// The property the whole cluster's ability to pull rests on. Once image
		// references name the in-cluster registry, one node whose agent falls behind must
		// not take that address away: doing so re-renders every workload in the cluster
		// back onto the upstream, and an air-gapped cluster has no upstream to go back to.
		name: "a node falls behind after the address is already published",
		state: imageAddressState{
			AlreadyPublished: true,
			Nodes:            3,
			Layouts:          3,
			Applied:          1,
		},
		switched: true,
	}, {
		// The same property under the harshest reading of it: whatever the cluster looks
		// like now, the question has already been answered.
		name:     "the address is published and nothing else is known",
		state:    imageAddressState{AlreadyPublished: true},
		switched: true,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.switched, test.state.switched())
			if test.switched {
				return
			}
			assert.Contains(t, test.state.blockedReason(), test.blocked)
		})
	}
}

// TestRegistryNodeConvergenceFilter is about the difference between an agent that succeeded
// and an agent that applied what the cluster asked for.
//
// An agent that cannot use the layout the API offers — a credential it cannot resolve, for
// instance — treats that as an unreachable API and serves the layout it was installed with
// instead. It then reports success, because from its own point of view it did succeed. So a
// node reporting `reconciled` is not a node running the cluster's configuration, and using
// that alone here would publish the in-cluster address to a cluster where some nodes are
// still routing by whatever they were bootstrapped with.
func TestRegistryNodeConvergenceFilter(t *testing.T) {
	cases := []struct {
		name       string
		generation int64
		observed   int64
		reconciled bool
		applied    bool
	}{{
		name:       "applied the generation it was given",
		generation: 7,
		observed:   7,
		reconciled: true,
		applied:    true,
	}, {
		name:       "reports success on an older generation",
		generation: 8,
		observed:   7,
		reconciled: true,
		applied:    false,
	}, {
		// What a fallback looks like: no generation reported at all, because the layout
		// came from a file on disk rather than from the API.
		name:       "reports success on no generation at all",
		generation: 3,
		observed:   0,
		reconciled: true,
		applied:    false,
	}, {
		name:       "has not applied its layout",
		generation: 4,
		observed:   4,
		reconciled: false,
		applied:    false,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			node := &registryv1alpha1.RegistryNode{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Generation: test.generation},
				Status: registryv1alpha1.RegistryNodeStatus{
					ObservedGeneration: test.observed,
					Reconciled:         test.reconciled,
				},
			}

			raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(node)
			assert.NoError(t, err)

			result, err := filterRegistryNodeConvergence(&unstructured.Unstructured{Object: raw})
			assert.NoError(t, err)
			assert.Equal(t, nodeConvergence{Applied: test.applied}, result)
		})
	}
}
