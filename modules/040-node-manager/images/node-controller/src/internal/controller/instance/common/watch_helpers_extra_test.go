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

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func TestMapObjectNameToInstance(t *testing.T) {
	t.Parallel()

	requests := MapObjectNameToInstance(context.Background(), &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-x"},
	})
	require.Len(t, requests, 1)
	require.Equal(t, "node-x", requests[0].Name)
	require.Empty(t, requests[0].Namespace)
}

func TestStaticNodeEventPredicateGenericAndNonNode(t *testing.T) {
	t.Parallel()

	pred, ok := StaticNodeEventPredicate().(predicate.Funcs)
	require.True(t, ok)

	require.True(t, pred.GenericFunc(event.GenericEvent{Object: staticNode("static-generic", nil)}))
	require.False(t, pred.GenericFunc(event.GenericEvent{Object: makeNode("cloud-generic", testCloudEphemeralNodeTypeValue, nil, false)}))

	// Non-Node objects must never match.
	nonNode := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod"}}
	require.False(t, pred.CreateFunc(event.CreateEvent{Object: nonNode}))
	require.False(t, pred.DeleteFunc(event.DeleteEvent{Object: nonNode}))
	require.False(t, pred.GenericFunc(event.GenericEvent{Object: nonNode}))
	require.False(t, pred.UpdateFunc(event.UpdateEvent{ObjectOld: nonNode, ObjectNew: staticNode("static", nil)}))
	require.False(t, pred.UpdateFunc(event.UpdateEvent{ObjectOld: staticNode("static", nil), ObjectNew: nonNode}))
}
