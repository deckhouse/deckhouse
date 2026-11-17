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

package informer

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func managedFields() []metav1.ManagedFieldsEntry {
	return []metav1.ManagedFieldsEntry{{
		Manager:    "kubectl",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		APIVersion: "rbac.authorization.k8s.io/v1",
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:roleRef":{},"f:subjects":{}}`)},
	}}
}

func roleBinding(fields []metav1.ManagedFieldsEntry) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "editors",
			Namespace:     "app",
			Labels:        map[string]string{"team": "a"},
			Annotations:   map[string]string{"note": "kept"},
			ManagedFields: fields,
		},
		Subjects: []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "jane@example.com"}},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "edit"},
	}
}

// annotated is a ClusterRole with the annotations permission-browser reads, one of another tool and,
// when lastApplied is set, the copy kubectl apply leaves.
func annotated(lastApplied bool) *rbacv1.ClusterRole {
	annotations := map[string]string{
		"user-authz.deckhouse.io/access-level":     "User",
		"rbac.deckhouse.io/deprecated-replaced-by": "d8:use:role:viewer",
		"en.meta.deckhouse.io/title":               "Viewer",
		"en.meta.deckhouse.io/description":         "Reads the objects of a namespace",
		"argocd.argoproj.io/tracking-id":           "rbac:rbac.authorization.k8s.io/ClusterRole:viewer",
	}
	if lastApplied {
		annotations[corev1.LastAppliedConfigAnnotation] = `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"ClusterRole"}`
	}
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "viewer", Annotations: annotations}}
}

func TestStripUnread(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "object with managed fields loses them and keeps the rest",
			input:    roleBinding(managedFields()),
			expected: roleBinding(nil),
		},
		{
			name:     "object without managed fields is unchanged",
			input:    roleBinding(nil),
			expected: roleBinding(nil),
		},
		{
			name:     "the last applied configuration goes, the other annotations stay",
			input:    annotated(true),
			expected: annotated(false),
		},
		{
			name: "an annotation map left empty stays a map",
			input: &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
				Name: "admin", Annotations: map[string]string{corev1.LastAppliedConfigAnnotation: "{}"},
			}},
			expected: &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "admin", Annotations: map[string]string{}}},
		},
		{
			name:     "object without annotations is unchanged",
			input:    &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app"}},
			expected: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app"}},
		},
		{
			name:     "value that is not an object passes through",
			input:    "not an object",
			expected: "not an object",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := StripUnread(tt.input)
			if err != nil {
				t.Fatalf("StripUnread() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("StripUnread() = %#v, want %#v", got, tt.expected)
			}
			// nil, not an empty list: an object written back with an empty list resets its
			// managed fields on the API server, one with none keeps them.
			if rb, ok := got.(*rbacv1.RoleBinding); ok && rb.ManagedFields != nil {
				t.Errorf("managedFields = %#v, want nil", rb.ManagedFields)
			}
		})
	}
}

func TestNewFactory(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	client := fake.NewClientset(roleBinding(managedFields()))
	factory := NewFactory(client, 0)
	lister := factory.Rbac().V1().RoleBindings().Lister()
	factory.Start(ctx.Done())
	// Shutdown waits for the informers, which stop only once the context is cancelled.
	defer func() {
		cancel()
		factory.Shutdown()
	}()
	for typ, synced := range factory.WaitForCacheSync(ctx.Done()) {
		if !synced {
			t.Fatalf("cache of %v is not synced", typ)
		}
	}

	got, err := lister.RoleBindings("app").Get("editors")
	if err != nil {
		t.Fatalf("get from the lister: %v", err)
	}
	if got.ManagedFields != nil {
		t.Errorf("cached managedFields = %#v, want nil", got.ManagedFields)
	}
	if !reflect.DeepEqual(got.Subjects, roleBinding(nil).Subjects) || got.RoleRef != roleBinding(nil).RoleRef {
		t.Errorf("cached binding = %#v, want subjects and roleRef kept", got)
	}

	// A binding added after the start goes through the same transform.
	_, err = client.RbacV1().RoleBindings("app").Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "viewers", Namespace: "app", ManagedFields: managedFields()},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !cache.WaitForCacheSync(ctx.Done(), func() bool {
		_, err := lister.RoleBindings("app").Get("viewers")
		return err == nil
	}) {
		t.Fatal("added binding never reached the cache")
	}
	added, err := lister.RoleBindings("app").Get("viewers")
	if err != nil {
		t.Fatalf("get added binding: %v", err)
	}
	if added.ManagedFields != nil {
		t.Errorf("added binding managedFields = %#v, want nil", added.ManagedFields)
	}
}
