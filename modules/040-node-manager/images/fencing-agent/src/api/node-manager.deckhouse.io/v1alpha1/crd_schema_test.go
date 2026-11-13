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

package v1alpha1

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const crdDir = "../../../../../../crds"

var timeType = reflect.TypeFor[metav1.Time]()

// leafTypes are serialized as scalars or come from upstream, so the schema is
// not walked below them.
var leafTypes = map[reflect.Type]bool{
	timeType:                             true,
	reflect.TypeFor[metav1.MicroTime]():  true,
	reflect.TypeFor[metav1.Duration]():   true,
	reflect.TypeFor[metav1.ObjectMeta](): true,
	reflect.TypeFor[metav1.Condition]():  true,
}

// microsecondsPattern is how the CRDs demand a timestamp with microseconds.
const microsecondsPattern = `\.[0-9]{6}`

type crdVersion struct {
	Name   string `json:"name"`
	Schema struct {
		OpenAPIV3Schema map[string]any `json:"openAPIV3Schema"`
	} `json:"schema"`
}

// TestTypesMatchTheCRDSchemas keeps these hand-written types and the CRDs the
// module installs in step. Every field of a type must be in the schema, or the
// API server prunes it and this binary reads it empty forever; every required
// property must be in the type, or a write through it is rejected. A property
// the binary does not need may stay out of its type.
func TestTypesMatchTheCRDSchemas(t *testing.T) {
	for file, typ := range map[string]reflect.Type{
		"fencingfailednodestates.yaml": reflect.TypeFor[FencingFailedNodeState](),
		"fencingslaprofiles.yaml":      reflect.TypeFor[FencingSLAProfile](),
	} {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(crdDir, file))
			if err != nil {
				// The file exists in a repo checkout; a build sandbox may cut the tree at src/.
				t.Skipf("CRD is not reachable: %v", err)
			}

			var crd struct {
				Spec struct {
					Versions []crdVersion `json:"versions"`
				} `json:"spec"`
			}

			if err := yaml.Unmarshal(raw, &crd); err != nil {
				t.Fatalf("unmarshal %s: %v", file, err)
			}

			index := slices.IndexFunc(crd.Spec.Versions, func(v crdVersion) bool { return v.Name == GroupVersion.Version })
			if index < 0 {
				t.Fatalf("%s has no version %s", file, GroupVersion.Version)
			}

			compareSchema(t, typ.Name(), crd.Spec.Versions[index].Schema.OpenAPIV3Schema, typ)
		})
	}
}

func compareSchema(t *testing.T, path string, schema map[string]any, typ reflect.Type) {
	t.Helper()

	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if leafTypes[typ] {
		// metav1.Time serializes whole seconds, which a property demanding
		// microseconds rejects on write.
		if pattern, _ := schema["pattern"].(string); typ == timeType && strings.Contains(pattern, microsecondsPattern) {
			t.Errorf("%s is metav1.Time, but the CRD schema demands microseconds: use metav1.MicroTime", path)
		}

		return
	}

	switch typ.Kind() {
	case reflect.Slice:
		items, _ := schema["items"].(map[string]any)
		compareSchema(t, path+"[]", items, typ.Elem())
	case reflect.Struct:
		properties, _ := schema["properties"].(map[string]any)
		fields := jsonFields(typ)

		for name, field := range fields {
			property, ok := properties[name].(map[string]any)
			if !ok {
				t.Errorf("%s.%s is in the Go type but not in the CRD schema", path, name)

				continue
			}

			compareSchema(t, path+"."+name, property, field)
		}

		required, _ := schema["required"].([]any)
		for _, name := range required {
			if _, ok := fields[name.(string)]; !ok {
				t.Errorf("%s.%v is required by the CRD schema but not in the Go type", path, name)
			}
		}
	}
}

// jsonFields maps the JSON names of a struct to their types, flattening inline
// embedded structs the way encoding/json does.
func jsonFields(typ reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)

	for i := range typ.NumField() {
		field := typ.Field(i)

		name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}

		if field.Anonymous && strings.Contains(options, "inline") {
			for inlined, inlinedType := range jsonFields(field.Type) {
				fields[inlined] = inlinedType
			}

			continue
		}

		if name == "" {
			name = field.Name
		}

		fields[name] = field.Type
	}

	return fields
}
