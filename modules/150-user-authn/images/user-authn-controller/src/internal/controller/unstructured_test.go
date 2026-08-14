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

package controller

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestObjectAndListSetGVK(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "dex.coreos.com", Version: "v1", Kind: "Password"}
	obj := Object(gvk)
	if obj.GroupVersionKind() != gvk {
		t.Errorf("Object GVK = %s, want %s", obj.GroupVersionKind(), gvk)
	}
	list := List(gvk)
	wantList := schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: "PasswordList"}
	if list.GroupVersionKind() != wantList {
		t.Errorf("List GVK = %s, want %s", list.GroupVersionKind(), wantList)
	}
}

func TestDecodeIntoNil(t *testing.T) {
	t.Parallel()

	var dest map[string]any
	if err := DecodeInto(nil, &dest); err == nil {
		t.Fatal("DecodeInto(nil) want error")
	}
}

func TestDecodeIntoRoundtripAndTypeError(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{Object: map[string]any{
		"email": "jane@example.com",
	}}
	var dest struct {
		Email string `json:"email"`
	}
	if err := DecodeInto(obj, &dest); err != nil {
		t.Fatalf("DecodeInto: %v", err)
	}
	if dest.Email != "jane@example.com" {
		t.Errorf("email = %q", dest.Email)
	}

	var bad int
	if err := DecodeInto(obj, &bad); err == nil {
		t.Fatal("DecodeInto into int: want error")
	}
}

func TestAsInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   any
		want    int64
		wantErr bool
	}{
		{name: "int", value: 4, want: 4},
		{name: "int32", value: int32(5), want: 5},
		{name: "int64", value: int64(3), want: 3},
		{name: "uint", value: uint(6), want: 6},
		{name: "uint32", value: uint32(8), want: 8},
		{name: "uint64", value: uint64(11), want: 11},
		{name: "float64 integer", value: float64(7), want: 7},
		{name: "nil", value: nil, want: 0},
		{name: "json number", value: json.Number("9"), want: 9},
		{name: "uint64 overflow", value: uint64(math.MaxUint64), wantErr: true},
		{name: "json number overflow", value: json.Number("18446744073709551615"), wantErr: true},
		{name: "float overflow", value: float64(math.MaxFloat64), wantErr: true},
		{name: "non-integer float", value: 1.5, wantErr: true},
		{name: "unexpected type", value: "nope", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := AsInt64(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("AsInt64(%v) = %d, want error", tt.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("AsInt64(%v): %v", tt.value, err)
			}
			if got != tt.want {
				t.Errorf("AsInt64(%v) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseRFC3339(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantNil bool
		wantErr bool
	}{
		{name: "empty", value: "", wantNil: true},
		{name: "rfc3339", value: "2026-08-13T12:00:00Z"},
		{name: "rfc3339 nano", value: "2026-08-13T12:00:00.123456789Z"},
		{name: "invalid", value: "not-a-time", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRFC3339(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRFC3339: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil")
			}
			if got.Location() != time.UTC {
				t.Errorf("location = %v, want UTC", got.Location())
			}
		})
	}
}

func TestAsUnstructured(t *testing.T) {
	t.Parallel()

	u := &unstructured.Unstructured{}
	got, ok := AsUnstructured(u)
	if !ok || got != u {
		t.Fatalf("AsUnstructured pointer: got %v ok=%v", got, ok)
	}
	if _, ok := AsUnstructured(nil); ok {
		t.Fatal("AsUnstructured(nil) want false")
	}
}

func TestGVKKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		gvk  schema.GroupVersionKind
		kind string
	}{
		{gvk: PasswordGVK, kind: "Password"},
		{gvk: OfflineSessionsGVK, kind: "OfflineSessions"},
		{gvk: RefreshTokenGVK, kind: "RefreshToken"},
		{gvk: UserGVK, kind: "User"},
		{gvk: GroupGVK, kind: "Group"},
		{gvk: DexProviderGVK, kind: "DexProvider"},
		{gvk: UserOperationGVK, kind: "UserOperation"},
		{gvk: NamespaceGVK, kind: "Namespace"},
		{gvk: SecretGVK, kind: "Secret"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			t.Parallel()
			if tt.gvk.Kind != tt.kind {
				t.Errorf("Kind = %q, want %q", tt.gvk.Kind, tt.kind)
			}
			obj := Object(tt.gvk)
			if obj.GroupVersionKind() != tt.gvk {
				t.Errorf("Object GVK = %s, want %s", obj.GroupVersionKind(), tt.gvk)
			}
		})
	}
}
