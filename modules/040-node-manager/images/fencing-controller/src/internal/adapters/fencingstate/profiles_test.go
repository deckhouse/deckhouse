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
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

const profileName = "critical"

func TestGetSLAProfileReturnsTheProfile(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(profile(profileName)).Build()

	got, err := NewProfiles(c).GetSLAProfile(context.Background(), profileName)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	if want := time.Second; got.Spec.Fallback.TTL.Duration != want {
		t.Errorf("read fallback.ttl %s, want %s", got.Spec.Fallback.TTL.Duration, want)
	}
}

// TestGetSLAProfileKeepsNotFoundRecognizable is the invariant the resolver
// depends on: a profile that is not in the cluster is a configuration error the
// operator has to fix, and the resolver tells it apart from a transient failure
// with apierrors.IsNotFound. Wrapping the error with %v instead of %w would make
// a missing profile look transient, the condition would never be set, and the
// incident would be retried in silence.
func TestGetSLAProfileKeepsNotFoundRecognizable(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()

	_, err := NewProfiles(c).GetSLAProfile(context.Background(), profileName)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("get of a missing profile returned %v, want an error IsNotFound recognizes", err)
	}

	if !strings.Contains(err.Error(), profileName) {
		t.Errorf("error %q does not name the profile that was read", err)
	}
}

func TestGetSLAProfileWrapsOtherFailures(t *testing.T) {
	apiDown := apierrors.NewServiceUnavailable("etcd leader changed")

	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return apiDown
			},
		}).
		Build()

	_, err := NewProfiles(c).GetSLAProfile(context.Background(), profileName)
	if !errors.Is(err, apiDown) {
		t.Fatalf("get returned %v, want the API error so the caller retries with backoff", err)
	}
}

func profile(name string) *v1alpha1.FencingSLAProfile {
	return &v1alpha1.FencingSLAProfile{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.FencingSLAProfileSpec{
			Fallback:   v1alpha1.FencingSLAProfileFallback{TTL: metav1.Duration{Duration: time.Second}},
			Evacuation: v1alpha1.FencingSLAProfileEvacuation{Delay: metav1.Duration{Duration: 1200 * time.Millisecond}},
		},
	}
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add fencing API to scheme: %v", err)
	}

	return s
}
