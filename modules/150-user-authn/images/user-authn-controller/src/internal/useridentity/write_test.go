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

package useridentity

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

var lockTime = time.Date(2077, 1, 1, 0, 0, 0, 0, time.UTC)

func TestMutate_CreatesMissingObject(t *testing.T) {
	t.Parallel()

	c := newFakeClient(t)
	name := Name("jane", "ldap")

	outcome, err := Mutate(t.Context(), c, c, name, func(current *Identity) (*Identity, error) {
		if current != nil {
			t.Fatalf("current = %v, want nil", current.Object())
		}
		id := New("jane", "ldap")
		if err := id.SetBlockedUntil(lockTime); err != nil {
			return nil, err
		}
		return id, nil
	})
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if outcome != OutcomeCreated {
		t.Errorf("outcome = %v, want created", outcome)
	}
	blocked, err := getIdentity(t, c, name).BlockedUntil()
	if err != nil || !blocked.Equal(lockTime) {
		t.Errorf("blockedUntil = %v (%v), want %v", blocked, err, lockTime)
	}
}

func TestMutate_UnchangedDoesNotWrite(t *testing.T) {
	t.Parallel()

	existing := New("jane", "ldap")
	if err := existing.SetBlockedUntil(lockTime); err != nil {
		t.Fatal(err)
	}
	var updates atomic.Int32
	c := newFakeClient(t, existing.Object(), interceptor.Funcs{
		Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			updates.Add(1)
			return cl.Update(ctx, obj, opts...)
		},
	})

	outcome, err := Mutate(t.Context(), c, c, existing.Object().GetName(), func(current *Identity) (*Identity, error) {
		// Writing the same instant again is not a change.
		if err := current.SetBlockedUntil(lockTime); err != nil {
			return nil, err
		}
		return current, nil
	})
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if outcome != OutcomeUnchanged || updates.Load() != 0 {
		t.Errorf("outcome = %v, updates = %d; want unchanged without a write", outcome, updates.Load())
	}
}

func TestMutate_RetriesTheDecisionAfterAConflict(t *testing.T) {
	t.Parallel()

	existing := New("jane", naming.LocalConnectorID)
	var conflicts atomic.Int32
	c := newFakeClient(t, existing.Object(), interceptor.Funcs{
		Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if conflicts.Add(1) == 1 {
				return apierrors.NewConflict(schema.GroupResource{Group: "dex.coreos.com", Resource: "useridentities"}, obj.GetName(), nil)
			}
			return cl.Update(ctx, obj, opts...)
		},
	})

	var decisions atomic.Int32
	outcome, err := Mutate(t.Context(), c, c, existing.Object().GetName(), func(current *Identity) (*Identity, error) {
		decisions.Add(1)
		current.SetClaims(Claims{UserID: "jane", Email: "jane@example.com"})
		return current, nil
	})
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if outcome != OutcomeUpdated {
		t.Errorf("outcome = %v, want updated", outcome)
	}
	if decisions.Load() != 2 {
		t.Errorf("decisions = %d, want 2: a conflict must re-read and decide again", decisions.Load())
	}
}

func TestMutate_RetriesCreateRace(t *testing.T) {
	t.Parallel()

	var creates atomic.Int32
	c := newFakeClient(t, nil, interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if creates.Add(1) == 1 {
				// Dex created the object between the read and the create.
				dexCopy := New("jane", "ldap")
				if err := cl.Create(ctx, dexCopy.Object()); err != nil {
					return err
				}
			}
			return cl.Create(ctx, obj, opts...)
		},
	})

	seenExisting := false
	outcome, err := Mutate(t.Context(), c, c, Name("jane", "ldap"), func(current *Identity) (*Identity, error) {
		if current == nil {
			return New("jane", "ldap"), nil
		}
		seenExisting = true
		if err := current.SetBlockedUntil(lockTime); err != nil {
			return nil, err
		}
		return current, nil
	})
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if !seenExisting || outcome != OutcomeUpdated {
		t.Errorf("outcome = %v, seenExisting = %v; want an update of the object Dex created", outcome, seenExisting)
	}
}

func TestMutate_RejectsAnotherName(t *testing.T) {
	t.Parallel()

	c := newFakeClient(t)
	_, err := Mutate(t.Context(), c, c, Name("jane", "ldap"), func(*Identity) (*Identity, error) {
		return New("john", "ldap"), nil
	})
	if err == nil {
		t.Fatal("Mutate wrote an object of another pair")
	}
}

func TestDelete_MissingIsNotAnError(t *testing.T) {
	t.Parallel()

	c := newFakeClient(t)
	if err := Delete(t.Context(), c, Name("jane", naming.LocalConnectorID)); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func newFakeClient(t *testing.T, objs ...any) client.WithWatch {
	t.Helper()
	builder := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithRESTMapper(newTestMapper())
	for _, o := range objs {
		switch v := o.(type) {
		case nil:
		case client.Object:
			builder = builder.WithObjects(v)
		case interceptor.Funcs:
			builder = builder.WithInterceptorFuncs(v)
		default:
			t.Fatalf("unexpected fake client option %T", o)
		}
	}
	return builder.Build()
}

func newTestMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "dex.coreos.com", Version: "v1"}})
	m.Add(controller.UserIdentityGVK, meta.RESTScopeNamespace)
	return m
}

func getIdentity(t *testing.T, c client.Client, name string) *Identity {
	t.Helper()
	obj := controller.Object(controller.UserIdentityGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj); err != nil {
		t.Fatalf("get useridentity %s: %v", name, err)
	}
	return FromUnstructured(obj)
}
