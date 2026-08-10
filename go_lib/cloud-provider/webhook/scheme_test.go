// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
)

func TestRegisterUnstructuredGVKsAllowsConversionCheck(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
	if err := RegisterUnstructuredGVKs(scheme, gvk); err != nil {
		t.Fatalf("RegisterUnstructuredGVKs() error = %v", err)
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	ok, err := conversion.IsConvertible(scheme, obj)
	if err != nil {
		t.Fatalf("IsConvertible() error = %v", err)
	}
	if ok {
		t.Fatal("IsConvertible() = true, want false for unstructured placeholder")
	}
}
