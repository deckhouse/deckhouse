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

package fencingstate

import (
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}

	return scheme
}

func TestGetSLAProfileReturnsObject(t *testing.T) {
	stored := &v1alpha1.FencingSLAProfile{ObjectMeta: metav1.ObjectMeta{Name: "medium"}}
	c := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(stored).Build()

	got, err := NewProfiles(c).GetSLAProfile(t.Context(), "medium")
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	if got.Name != "medium" {
		t.Errorf("unexpected profile %q", got.Name)
	}
}

// Load distinguishes NotFound from transient errors, so the wrapper must keep
// the apierrors chain intact.
func TestGetSLAProfilePreservesNotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()

	_, err := NewProfiles(c).GetSLAProfile(t.Context(), "missing")
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected a NotFound error, got %v", err)
	}
}
