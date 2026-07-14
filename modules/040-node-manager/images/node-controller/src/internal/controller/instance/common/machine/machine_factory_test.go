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

package machine

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

func TestNewMachineUnsupportedType(t *testing.T) {
	t.Parallel()

	if _, err := NewMachineFactory().NewMachine(&corev1.Node{}); err == nil ||
		!strings.Contains(err.Error(), "unsupported machine type") {
		t.Fatalf("expected unsupported machine type error, got %v", err)
	}
}

func TestNewMachineFromRefValidation(t *testing.T) {
	t.Parallel()

	f := NewMachineFactory()
	ctx := context.Background()
	c := fake.NewClientBuilder().Build()

	tests := []struct {
		name    string
		ref     *deckhousev1alpha2.MachineRef
		wantErr string
	}{
		{"nil ref", nil, "machine ref is nil"},
		{"empty name", &deckhousev1alpha2.MachineRef{Name: ""}, "machine ref name is empty"},
		{"unsupported apiVersion", &deckhousev1alpha2.MachineRef{Name: "m", APIVersion: "bad.example.com/v1"}, "unsupported machine apiVersion"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.NewMachineFromRef(ctx, c, tt.ref)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewMachineFromRefDefaultsNamespace(t *testing.T) {
	t.Parallel()

	obj := &capi.Machine{}
	obj.Name = "defaulted"
	obj.Namespace = MachineNamespace
	scheme := runtime.NewScheme()
	if err := capi.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()

	// An empty namespace in the ref must default to the machine namespace.
	m, err := NewMachineFactory().NewMachineFromRef(context.Background(), c, &deckhousev1alpha2.MachineRef{
		Name:       "defaulted",
		APIVersion: capi.GroupVersion.String(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.GetName() != "defaulted" {
		t.Fatalf("name: got %q want %q", m.GetName(), "defaulted")
	}
}
