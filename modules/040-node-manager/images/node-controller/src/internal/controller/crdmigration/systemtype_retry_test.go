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

package crdmigration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
)

type patchFunc func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error

var webhookDenial = apierrors.NewForbidden(schema.GroupResource{Group: "deckhouse.io", Resource: "nodegroups"},
	"worker", errors.New("admission webhook denied the request"))

func newFakeClient(t *testing.T, patch patchFunc, objects ...client.Object) client.Client {
	t.Helper()

	scheme := k8sruntime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithInterceptorFuncs(interceptor.Funcs{Patch: patch}).
		Build()
}

func legacyGroup(name string, systemType deckhousev1.SystemType) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic, SystemType: systemType},
	}
}

func storedSystemType(t *testing.T, c client.Client, name string) deckhousev1.SystemType {
	t.Helper()

	group := &deckhousev1.NodeGroup{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: name}, group))
	return group.Spec.SystemType
}

func TestRetrySystemTypeMigration(t *testing.T) {
	t.Run("retries until the webhook lets the NodeGroup through", func(t *testing.T) {
		denials := 2
		c := newFakeClient(t, func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, isGroup := obj.(*deckhousev1.NodeGroup); isGroup && denials > 0 {
				denials--
				return webhookDenial
			}
			return c.Patch(ctx, obj, patch, opts...)
		}, legacyGroup("worker", deckhousev1.SystemTypeLegacyImmutable))

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		RetrySystemTypeMigration(ctx, c, logr.Discard(), time.Millisecond)
		require.Zero(t, denials)
		require.Equal(t, deckhousev1.SystemTypeDeckhouseEngine, storedSystemType(t, c, "worker"))
	})

	t.Run("returns once the context is done before a success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		c := newFakeClient(t, func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
			cancel()
			return webhookDenial
		}, legacyGroup("worker", deckhousev1.SystemTypeLegacyImmutable))

		RetrySystemTypeMigration(ctx, c, logr.Discard(), time.Millisecond)
		require.Equal(t, deckhousev1.SystemTypeLegacyImmutable, storedSystemType(t, c, "worker"))
	})
}

func TestMigrateSystemTypesKeepsGoingPastARefusal(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   "engine",
		Labels: map[string]string{common.NodeSystemTypeLabel: string(deckhousev1.SystemTypeLegacyImmutable)},
	}}
	c := newFakeClient(t, func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
		if obj.GetName() == "a-refused" {
			return webhookDenial
		}
		return c.Patch(ctx, obj, patch, opts...)
	},
		legacyGroup("a-refused", deckhousev1.SystemTypeLegacyImmutable),
		legacyGroup("b-immutable", deckhousev1.SystemTypeLegacyImmutable),
		legacyGroup("c-mutable", deckhousev1.SystemTypeLegacyMutable),
		node,
	)

	err := MigrateSystemTypes(t.Context(), c)
	require.ErrorContains(t, err, "NodeGroup a-refused")
	require.True(t, apierrors.IsForbidden(err), err)

	require.Equal(t, deckhousev1.SystemTypeLegacyImmutable, storedSystemType(t, c, "a-refused"))
	require.Equal(t, deckhousev1.SystemTypeDeckhouseEngine, storedSystemType(t, c, "b-immutable"))
	require.Equal(t, deckhousev1.SystemTypeGenericOS, storedSystemType(t, c, "c-mutable"))
	stored := &corev1.Node{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: "engine"}, stored))
	require.Equal(t, string(deckhousev1.SystemTypeDeckhouseEngine), stored.Labels[common.NodeSystemTypeLabel])
}

func TestMigrateSystemTypesRefusesAStaleResourceVersion(t *testing.T) {
	changed := false
	c := newFakeClient(t, func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
		if !changed {
			changed = true
			group := &deckhousev1.NodeGroup{}
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(obj), group))
			group.Labels = map[string]string{"changed": "after-list"}
			require.NoError(t, c.Update(ctx, group))
		}
		return c.Patch(ctx, obj, patch, opts...)
	}, legacyGroup("worker", deckhousev1.SystemTypeLegacyImmutable))

	err := MigrateSystemTypes(t.Context(), c)
	require.True(t, apierrors.IsConflict(err), err)
	require.Equal(t, deckhousev1.SystemTypeLegacyImmutable, storedSystemType(t, c, "worker"))

	require.NoError(t, MigrateSystemTypes(t.Context(), c))
	group := &deckhousev1.NodeGroup{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: "worker"}, group))
	require.Equal(t, deckhousev1.SystemTypeDeckhouseEngine, group.Spec.SystemType)
	require.Equal(t, "after-list", group.Labels["changed"])
}
