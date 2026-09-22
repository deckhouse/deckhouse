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

package status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func newPublisher(t *testing.T, objects ...client.Object) (*Publisher, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&registryv1alpha1.RegistryNode{}).
		Build()

	return &Publisher{Client: fakeClient, Node: "worker-1"}, fakeClient
}

func nodeLayout() *registryv1alpha1.RegistryNode {
	return &registryv1alpha1.RegistryNode{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Generation: 7},
	}
}

func getLayout(t *testing.T, c client.Client) *registryv1alpha1.RegistryNode {
	t.Helper()

	object := &registryv1alpha1.RegistryNode{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, object))
	return object
}

func TestPublish(t *testing.T) {
	publisher, c := newPublisher(t, nodeLayout())

	require.NoError(t, publisher.Publish(context.Background(), State{
		ObservedGeneration: 7,
		Reconciled:         true,
		ProxyListening:     true,
		ActiveBackends:     []string{"storage", "upstream"},
	}))

	got := getLayout(t, c)
	assert.EqualValues(t, 7, got.Status.ObservedGeneration)
	assert.True(t, got.Status.Reconciled)
	assert.True(t, got.Status.ProxyListening)
	assert.Equal(t, []string{"storage", "upstream"}, got.Status.ActiveBackends)

	condition := apimeta.FindStatusCondition(got.Status.Conditions, registryv1alpha1.ConditionReconciled)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, registryv1alpha1.ReasonResolved, condition.Reason)
}

// TestPublishFromCacheStaysTrue is the distinction an operator needs: the node IS
// configured and pulling, and what the reason says is that its configuration may be
// behind the cluster's.
func TestPublishFromCacheStaysTrue(t *testing.T) {
	publisher, c := newPublisher(t, nodeLayout())

	require.NoError(t, publisher.Publish(context.Background(), State{
		ObservedGeneration: 7, Reconciled: true, ProxyListening: true, FromCache: true,
	}))

	condition := apimeta.FindStatusCondition(
		getLayout(t, c).Status.Conditions, registryv1alpha1.ConditionReconciled)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, registryv1alpha1.ReasonAppliedFromCache, condition.Reason)
	assert.Contains(t, condition.Message, "copy on the node")
}

func TestPublishFailure(t *testing.T) {
	publisher, c := newPublisher(t, nodeLayout())

	require.NoError(t, publisher.Publish(context.Background(), State{
		ObservedGeneration: 7, Error: "the drop-in directory is not writable",
	}))

	got := getLayout(t, c)
	assert.False(t, got.Status.Reconciled)

	condition := apimeta.FindStatusCondition(got.Status.Conditions, registryv1alpha1.ConditionReconciled)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Contains(t, condition.Message, "not writable")
}

// TestPublishIsIdempotent matters because every node writes its own object and the
// controller watches them all: a needless write multiplies into a reconciliation of the
// whole layout.
func TestPublishIsIdempotent(t *testing.T) {
	publisher, c := newPublisher(t, nodeLayout())
	state := State{ObservedGeneration: 7, Reconciled: true, ProxyListening: true}
	ctx := context.Background()

	require.NoError(t, publisher.Publish(ctx, state))
	version := getLayout(t, c).ResourceVersion

	require.NoError(t, publisher.Publish(ctx, state))
	assert.Equal(t, version, getLayout(t, c).ResourceVersion)

	// And a real change is still written, so the assertion above is not vacuous.
	state.ProxyListening = false
	require.NoError(t, publisher.Publish(ctx, state))
	assert.NotEqual(t, version, getLayout(t, c).ResourceVersion)
}

// TestPublishToleratesAMissingLayout covers an unmanaged node, or one whose layout has
// not been compiled yet. An agent logging a failure every pass over that would drown out
// the ones that matter.
func TestPublishToleratesAMissingLayout(t *testing.T) {
	publisher, _ := newPublisher(t)
	assert.NoError(t, publisher.Publish(context.Background(), State{Reconciled: true}))
}

func TestPublishRequiresANodeName(t *testing.T) {
	publisher, _ := newPublisher(t, nodeLayout())
	publisher.Node = ""

	assert.Error(t, publisher.Publish(context.Background(), State{}))
}
