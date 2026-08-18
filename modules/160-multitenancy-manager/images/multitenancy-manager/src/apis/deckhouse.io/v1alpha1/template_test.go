/*
Copyright 2024 Flant JSC

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

package v1alpha1

import (
	"sync"
	"testing"
)

// TestParametersSchemaDeepCopyInto_NilMap verifies that DeepCopyInto works
// correctly when OpenAPIV3Schema is nil.
func TestParametersSchemaDeepCopyInto_NilMap(t *testing.T) {
	src := &ParametersSchema{}
	dst := &ParametersSchema{}
	src.DeepCopyInto(dst)
	if dst.OpenAPIV3Schema != nil {
		t.Errorf("expected nil map, got %v", dst.OpenAPIV3Schema)
	}
}

// TestParametersSchemaDeepCopyInto_IndependentCopy verifies that the copy is
// independent: mutations to the destination do not affect the source.
func TestParametersSchemaDeepCopyInto_IndependentCopy(t *testing.T) {
	src := &ParametersSchema{
		OpenAPIV3Schema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"foo": "bar"},
		},
	}
	dst := &ParametersSchema{}
	src.DeepCopyInto(dst)

	// Mutate destination.
	dst.OpenAPIV3Schema["extra"] = "value"

	if _, ok := src.OpenAPIV3Schema["extra"]; ok {
		t.Error("mutation of destination affected source map")
	}
	if len(dst.OpenAPIV3Schema) != 3 {
		t.Errorf("expected 3 keys in destination, got %d", len(dst.OpenAPIV3Schema))
	}
}

// TestParametersSchemaDeepCopyInto_ConcurrentCopies is the regression test for
// the "concurrent map writes" panic.  It calls DeepCopyObject on a shared
// ProjectTemplate from many goroutines simultaneously, which is exactly what
// the webhook cache does under load.
func TestParametersSchemaDeepCopyInto_ConcurrentCopies(t *testing.T) {
	pt := &ProjectTemplate{
		Spec: ProjectTemplateSpec{
			ParametersSchema: ParametersSchema{
				OpenAPIV3Schema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = pt.DeepCopyObject()
		}()
	}
	wg.Wait()
}
