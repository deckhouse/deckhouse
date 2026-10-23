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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var bumpTime = time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)

func TestIdentity_Generation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		annotation *string
		expected   uint64
		wantErr    bool
	}{
		{name: "no annotation is generation 0", expected: 0},
		{name: "zero", annotation: new("0"), expected: 0},
		{name: "decimal", annotation: new("42"), expected: 42},
		{name: "max uint64", annotation: new("18446744073709551615"), expected: 18446744073709551615},
		{name: "empty", annotation: new(""), wantErr: true},
		{name: "leading zero", annotation: new("042"), wantErr: true},
		{name: "sign", annotation: new("+1"), wantErr: true},
		{name: "negative", annotation: new("-1"), wantErr: true},
		{name: "overflow", annotation: new("18446744073709551616"), wantErr: true},
		{name: "not a number", annotation: new("three"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := New("jane", "ldap")
			if tt.annotation != nil {
				id.setAnnotation(GenerationAnnotation, *tt.annotation)
			}
			got, err := id.Generation()
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidGeneration) {
					t.Fatalf("Generation = %d, %v; want ErrInvalidGeneration", got, err)
				}
				return
			}
			if err != nil || got != tt.expected {
				t.Errorf("Generation = %d, %v; want %d", got, err, tt.expected)
			}
		})
	}
}

func TestIdentity_SetGeneration(t *testing.T) {
	t.Parallel()

	id := New("jane", "ldap")
	id.SetGeneration(7)
	if value := id.Object().GetAnnotations()[GenerationAnnotation]; value != "7" {
		t.Errorf("annotation = %q, want 7", value)
	}
	if got, err := id.Generation(); err != nil || got != 7 {
		t.Errorf("Generation = %d, %v; want 7", got, err)
	}
}

func TestBumpGeneration_CreatesAMissingObjectAtOne(t *testing.T) {
	t.Parallel()

	c := newFakeClient(t)
	pair := Pair{UserID: "jane", ConnectorID: "ldap"}
	claims := Claims{UserID: "jane", Email: "jane@example.com"}

	generation, err := BumpGeneration(t.Context(), c, c, pair, claims, bumpTime)
	if err != nil {
		t.Fatalf("BumpGeneration: %v", err)
	}
	if generation != 1 {
		t.Errorf("generation = %d, want 1", generation)
	}
	id := getIdentity(t, c, pair.Name())
	if got, _ := id.Generation(); got != 1 {
		t.Errorf("stored generation = %d, want 1", got)
	}
	if got, _ := id.Claims(); !got.Equal(claims) {
		t.Errorf("claims = %+v, want %+v", got, claims)
	}
	if created, _ := id.CreatedAt(); !created.Equal(bumpTime) {
		t.Errorf("createdAt = %v, want %v", created, bumpTime)
	}
	// The transitional Dex reads the old places of an object without the record, and the copy
	// brings them over: a revocation must not cut the pair off its TOTP secret and lock.
	if id.Migrated() {
		t.Error("the created object carries the record of the copy")
	}
}

func TestBumpGeneration_RaisesByOneAndChangesNothingElse(t *testing.T) {
	t.Parallel()

	existing := New("jane", "ldap")
	existing.SetClaims(Claims{UserID: "jane", Email: "jane@example.com"})
	if err := existing.SetBlockedUntil(lockTime); err != nil {
		t.Fatal(err)
	}
	existing.SetGeneration(3)
	before := existing.Object().DeepCopy()
	c := newFakeClient(t, existing.Object())

	generation, err := BumpGeneration(t.Context(), c, c, Pair{UserID: "jane", ConnectorID: "ldap"}, Claims{UserID: "other"}, bumpTime)
	if err != nil {
		t.Fatalf("BumpGeneration: %v", err)
	}
	if generation != 4 {
		t.Errorf("generation = %d, want 4", generation)
	}
	after := getIdentity(t, c, existing.Object().GetName()).Object()
	if after.GetAnnotations()[GenerationAnnotation] != "4" {
		t.Errorf("annotation = %q, want 4", after.GetAnnotations()[GenerationAnnotation])
	}
	for _, field := range []string{fieldUserID, fieldConnectorID, fieldClaims, fieldBlockedUntil} {
		if !equalJSON(before.Object[field], after.Object[field]) {
			t.Errorf("%s changed: %v -> %v", field, before.Object[field], after.Object[field])
		}
	}
}

func TestBumpGeneration_RetriesAfterAConflict(t *testing.T) {
	t.Parallel()

	existing := New("jane", "ldap")
	existing.SetGeneration(1)
	var updates atomic.Int32
	c := newFakeClient(t, existing.Object(), interceptor.Funcs{
		Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if updates.Add(1) == 1 {
				// Another worker raised the generation between the read and the write.
				concurrent := getIdentity(t, cl, obj.GetName())
				concurrent.SetGeneration(2)
				if err := cl.Update(ctx, concurrent.Object()); err != nil {
					return err
				}
				return apierrors.NewConflict(schema.GroupResource{Group: "dex.coreos.com", Resource: "useridentities"}, obj.GetName(), nil)
			}
			return cl.Update(ctx, obj, opts...)
		},
	})

	generation, err := BumpGeneration(t.Context(), c, c, Pair{UserID: "jane", ConnectorID: "ldap"}, Claims{}, bumpTime)
	if err != nil {
		t.Fatalf("BumpGeneration: %v", err)
	}
	if generation != 3 {
		t.Errorf("generation = %d, want 3: the retry must read the concurrent write and add one to it", generation)
	}
}

func TestBumpGeneration_FailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build func() *Identity
	}{
		{
			name: "unparsable generation",
			build: func() *Identity {
				id := New("jane", "ldap")
				id.setAnnotation(GenerationAnnotation, "x")
				return id
			},
		},
		{
			name: "object of another pair under the name",
			build: func() *Identity {
				id := New("jane", "ldap")
				id.Object().Object[fieldUserID] = "john"
				return id
			},
		},
		{
			name: "generation at the maximum",
			build: func() *Identity {
				id := New("jane", "ldap")
				id.setAnnotation(GenerationAnnotation, "18446744073709551615")
				return id
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newFakeClient(t, tt.build().Object())
			if _, err := BumpGeneration(t.Context(), c, c, Pair{UserID: "jane", ConnectorID: "ldap"}, Claims{}, bumpTime); err == nil {
				t.Fatal("BumpGeneration succeeded, want an error")
			}
		})
	}
}

func equalJSON(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(left, right)
}
