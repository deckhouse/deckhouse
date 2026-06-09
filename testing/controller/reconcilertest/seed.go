// Copyright 2025 Flant JSC
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

package reconcilertest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// LoadFixture reads a fixture file from dir. It returns an empty slice (and no
// error) when name is empty, which lets callers seed a cluster with no objects.
func LoadFixture(dir, name string) ([]byte, error) {
	if name == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, fmt.Errorf("read fixture %s: %w", name, err)
	}

	return data, nil
}

// Decode turns a multi-document YAML blob into typed client objects using the
// supplied scheme. Unlike the hand-written `switch obj.Kind` blocks that this
// framework replaces, it relies on the scheme to map apiVersion/kind to a Go
// type, so any registered resource is supported automatically.
//
// It instantiates the typed object from the scheme by GVK and then unmarshals
// with sigs.k8s.io/yaml (which, like the legacy `assembleInitObject` helpers,
// matches struct json tags case-insensitively). This preserves the exact
// decoding semantics the existing golden files were generated with.
func Decode(scheme *runtime.Scheme, raw []byte) ([]client.Object, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	docs := SplitDocuments(raw)
	objects := make([]client.Object, 0, len(docs))
	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		var typeMeta metav1.TypeMeta
		if err := yaml.Unmarshal([]byte(doc), &typeMeta); err != nil {
			return nil, fmt.Errorf("read type meta: %w\n%s", err, doc)
		}

		gvk := typeMeta.GroupVersionKind()
		if gvk.Empty() {
			return nil, fmt.Errorf("manifest is missing apiVersion/kind\n%s", doc)
		}

		obj, err := scheme.New(gvk)
		if err != nil {
			return nil, fmt.Errorf("new object for %s: %w", gvk, err)
		}

		if err := yaml.Unmarshal([]byte(doc), obj); err != nil {
			return nil, fmt.Errorf("decode %s: %w\n%s", gvk, err, doc)
		}

		clientObj, ok := obj.(client.Object)
		if !ok {
			return nil, fmt.Errorf("decoded object %T does not implement client.Object", obj)
		}

		objects = append(objects, clientObj)
	}

	return objects, nil
}
