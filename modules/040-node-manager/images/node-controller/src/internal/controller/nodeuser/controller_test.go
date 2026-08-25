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

package nodeuser

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/common"
)

// TestStaleErrorsPatch pins the patch shape: only the named keys are nulled. A patch that nulls
// status.errors as a whole — what a typed MergeFrom sends once omitempty drops the emptied map —
// would wipe the errors of nodes that are still alive.
func TestStaleErrorsPatch(t *testing.T) {
	patch := staleErrorsPatch([]string{"alpha", "zeta"})
	require.JSONEq(t, `{"status":{"errors":{"alpha":null,"zeta":null}}}`, string(patch))
}

func TestStaleNodeNames(t *testing.T) {
	managed := func(name string) *corev1.Node {
		return &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{common.NodeGroupLabel: "worker"},
		}}
	}

	tests := []struct {
		name     string
		errs     map[string]string
		nodes    []client.Object
		expStale []string
	}{
		{name: "no errors", errs: nil, nodes: []client.Object{managed("a")}, expStale: nil},
		{
			name:  "every error refers to a live node",
			errs:  map[string]string{"a": "boom", "b": "boom"},
			nodes: []client.Object{managed("a"), managed("b"), managed("c")},
		},
		{
			name:     "one vanished node among live ones",
			errs:     map[string]string{"a": "boom", "gone": "boom"},
			nodes:    []client.Object{managed("a")},
			expStale: []string{"gone"},
		},
		{
			// Sorted output keeps the emitted patch deterministic.
			name:     "several vanished nodes come back sorted",
			errs:     map[string]string{"zeta": "boom", "alpha": "boom", "a": "boom"},
			nodes:    []client.Object{managed("a")},
			expStale: []string{"alpha", "zeta"},
		},
		{
			name:     "no live nodes at all",
			errs:     map[string]string{"a": "boom"},
			expStale: []string{"a"},
		},
		{
			// The hook selected nodes by the node-group label: one without it is not ours.
			name:     "an unmanaged node counts as gone",
			errs:     map[string]string{"stripped": "boom"},
			nodes:    []client.Object{&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "stripped"}}},
			expStale: []string{"stripped"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			r := &Reconciler{}
			r.InjectClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.nodes...).Build())

			stale, err := r.staleNodeNames(t.Context(), tc.errs)
			require.NoError(t, err)
			require.Equal(t, tc.expStale, stale)
		})
	}
}
