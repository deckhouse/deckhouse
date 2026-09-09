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

package helm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestStampHoldoff(t *testing.T) {
	cases := []struct {
		name string
		ns   *corev1.Namespace
		want time.Duration
	}{
		{name: "nil", ns: nil, want: 0},
		{name: "no creation timestamp", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}, want: 0},
		{
			name: "old enough",
			ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name:              "foo",
				CreationTimestamp: metav1.NewTime(time.Now().Add(-AdoptGracePeriod - time.Second)),
			}},
			want: 0,
		},
		{
			name: "young with a release name is free to inspect",
			ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name:              "foo",
				CreationTimestamp: metav1.Now(),
				Annotations:       map[string]string{ResourceAnnotationReleaseName: "foo"},
			}},
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, StampHoldoff(tc.ns))
		})
	}

	t.Run("young empty namespace waits", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:              "foo",
			CreationTimestamp: metav1.Now(),
		}}
		got := StampHoldoff(ns)
		assert.Greater(t, got, time.Duration(0))
		assert.LessOrEqual(t, got, AdoptGracePeriod)
	})
}

func TestStampReleaseOwnership_YoungNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:              "foo",
		CreationTimestamp: metav1.Now(),
	}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ns).Build()

	err := StampReleaseOwnership(context.Background(), c, "foo")
	require.Error(t, err)
	var holdoff StampHoldoffError
	require.True(t, errors.As(err, &holdoff))
	assert.Greater(t, holdoff.Remaining, time.Duration(0))

	got := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.NotEqual(t, ManagedByHelm, got.Labels[ResourceLabelManagedBy])
	assert.Empty(t, got.Annotations[ResourceAnnotationReleaseName])
}

func TestStampReleaseOwnership(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ns).Build()

	require.NoError(t, StampReleaseOwnership(context.Background(), c, "foo"))

	got := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, ManagedByHelm, got.Labels[ResourceLabelManagedBy])
	assert.Equal(t, "foo", got.Annotations[ResourceAnnotationReleaseName])
	assert.Equal(t, "", got.Annotations[ResourceAnnotationReleaseNamespace])
}

func TestStampReleaseOwnership_MissingNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	require.NoError(t, StampReleaseOwnership(context.Background(), c, "missing"))
}

func TestApplyReleaseOwnership_Terminating(t *testing.T) {
	now := metav1.Now()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo", DeletionTimestamp: &now}}
	assert.False(t, ApplyReleaseOwnership(ns, "foo"))
	assert.NotContains(t, ns.Labels, ResourceLabelManagedBy)
}

func TestStampReleaseOwnership_Idempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ns).Build()

	require.NoError(t, StampReleaseOwnership(context.Background(), c, "foo"))
	first := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, first))

	require.NoError(t, StampReleaseOwnership(context.Background(), c, "foo"))
	second := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, second))
	assert.Equal(t, first.ResourceVersion, second.ResourceVersion)
}

func TestStampReleaseOwnership_LongProjectName(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	name := "t-mtm61-" + strings.Repeat("x", 53)
	require.Equal(t, 61, len(name))
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ns).Build()

	require.NoError(t, StampReleaseOwnership(context.Background(), c, name))

	got := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: name}, got))
	rel := ReleaseName(name)
	assert.LessOrEqual(t, len(rel), helmReleaseNameMaxLen)
	assert.NotEqual(t, name, rel)
	assert.Equal(t, rel, got.Annotations[ResourceAnnotationReleaseName])
	assert.Equal(t, ManagedByHelm, got.Labels[ResourceLabelManagedBy])
}

func TestApplyReleaseOwnership_ForeignReleaseUnchanged(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "foo",
		Labels: map[string]string{
			ResourceLabelManagedBy: ManagedByHelm,
		},
		Annotations: map[string]string{
			ResourceAnnotationReleaseName:      "user-chart",
			ResourceAnnotationReleaseNamespace: "foo",
		},
	}}

	assert.Equal(t, "user-chart/foo", ForeignRelease(ns, "foo"))
	assert.False(t, ApplyReleaseOwnership(ns, "foo"))
	assert.Equal(t, "user-chart", ns.Annotations[ResourceAnnotationReleaseName])
	assert.Equal(t, "foo", ns.Annotations[ResourceAnnotationReleaseNamespace])
}

func TestStampReleaseOwnership_ForeignRelease(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "foo",
		Annotations: map[string]string{
			ResourceAnnotationReleaseName: "user-chart",
		},
	}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ns).Build()

	err := StampReleaseOwnership(context.Background(), c, "foo")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrForeignRelease)

	got := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, "user-chart", got.Annotations[ResourceAnnotationReleaseName])
	assert.NotEqual(t, ManagedByHelm, got.Labels[ResourceLabelManagedBy])
}

func TestForeignRelease_SameNameInUserNamespaceIsForeign(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "foo",
		Annotations: map[string]string{
			ResourceAnnotationReleaseName:      "foo",
			ResourceAnnotationReleaseNamespace: "foo",
		},
	}}
	assert.Equal(t, "foo/foo", ForeignRelease(ns, "foo"))
	assert.False(t, ApplyReleaseOwnership(ns, "foo"))
	assert.Equal(t, "foo", ns.Annotations[ResourceAnnotationReleaseNamespace])
}

func TestForeignRelease_OurStorageNamespaceIsOurs(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "foo",
		Annotations: map[string]string{
			ResourceAnnotationReleaseName:      "foo",
			ResourceAnnotationReleaseNamespace: ReleaseStorageNamespace,
		},
	}}
	assert.Empty(t, ForeignRelease(ns, "foo"))
}

func TestForeignRelease_EmptyNamespaceStampIsOurs(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:        "foo",
		Annotations: map[string]string{ResourceAnnotationReleaseName: "foo"},
	}}
	assert.Empty(t, ForeignRelease(ns, "foo"))
}

func TestForeignRelease_LongNameOldStampIsOurs(t *testing.T) {
	name := "t-mtm61-" + strings.Repeat("x", 53)
	require.Equal(t, 61, len(name))
	rel := ReleaseName(name)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:        name,
		Annotations: map[string]string{ResourceAnnotationReleaseName: name},
	}}
	assert.Empty(t, ForeignRelease(ns, rel))
	assert.True(t, ApplyReleaseOwnership(ns, rel))
	assert.Equal(t, rel, ns.Annotations[ResourceAnnotationReleaseName])
	assert.Equal(t, "", ns.Annotations[ResourceAnnotationReleaseNamespace])
}
