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

package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// crdPath points at the CRD this module ships with. It lives outside the Go module, so the path is
// relative to this package directory.
const crdPath = "../../../../../crds/node_group.yaml"

// allowedOpaquePaths are spec subtrees this module deliberately does not model. Each entry needs a
// reason: either the schema belongs to somebody else, or it has no fixed keys at all. A path listed
// here is skipped together with everything under it.
var allowedOpaquePaths = map[string]string{
	"nodeTemplate.labels":      "free-form map[string]string, no fixed keys",
	"nodeTemplate.annotations": "free-form map[string]string, no fixed keys",
}

// TestNodeGroupSpecMatchesCRD fails when the shipped CRD holds a spec field the Go type cannot.
// Such a field decodes to nothing, disappears from the bashible context, shifts the node
// configuration checksum and re-runs bashible on every node — silently.
//
// Note: the CRD lives outside the node_controller paths-filter, so a PR that only touches
// crds/node_group.yaml does not run this test. The guard still fires on the next PR that touches
// this module — which is the one that would ship the mismatch.
func TestNodeGroupSpecMatchesCRD(t *testing.T) {
	crdSpec := loadStoredVersionSpecSchema(t)
	goPaths := jsonPaths(reflect.TypeOf(v1.NodeGroupSpec{}), "")

	var missing []string
	walkSchema(crdSpec, "", func(path string) {
		if isAllowed(path) {
			return
		}
		if _, ok := goPaths[path]; !ok {
			missing = append(missing, path)
		}
	})

	sort.Strings(missing)
	require.Empty(t, missing,
		"CRD spec paths absent from v1.NodeGroupSpec — add the field, or record it in allowedOpaquePaths with a reason:\n%s",
		strings.Join(missing, "\n"))
}

func isAllowed(path string) bool {
	for allowed := range allowedOpaquePaths {
		if path == allowed || strings.HasPrefix(path, allowed+".") {
			return true
		}
	}
	return false
}

// loadStoredVersionSpecSchema returns the spec schema of the version marked storage: true — the one
// objects are persisted as, and the only one this module decodes.
func loadStoredVersionSpecSchema(t *testing.T) *apiextensionsv1.JSONSchemaProps {
	t.Helper()

	raw, err := os.ReadFile(filepath.Clean(crdPath))
	require.NoError(t, err, "read NodeGroup CRD")

	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, sigsyaml.Unmarshal(raw, crd), "decode NodeGroup CRD")

	for i := range crd.Spec.Versions {
		ver := crd.Spec.Versions[i]
		if !ver.Storage {
			continue
		}
		require.NotNil(t, ver.Schema, "storage version %s has no schema", ver.Name)
		spec, ok := ver.Schema.OpenAPIV3Schema.Properties["spec"]
		require.True(t, ok, "storage version %s has no spec in schema", ver.Name)
		return &spec
	}

	t.Fatal("NodeGroup CRD has no storage version")
	return nil
}

// walkSchema visits every property path of an object schema, skipping subtrees the CRD itself
// declares open.
func walkSchema(s *apiextensionsv1.JSONSchemaProps, prefix string, visit func(path string)) {
	if s == nil {
		return
	}
	if s.XPreserveUnknownFields != nil && *s.XPreserveUnknownFields {
		return
	}
	for name := range s.Properties {
		prop := s.Properties[name]
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		visit(path)

		switch prop.Type {
		case "object":
			walkSchema(&prop, path, visit)
		case "array":
			if prop.Items != nil && prop.Items.Schema != nil {
				walkSchema(prop.Items.Schema, path, visit)
			}
		}
	}
}

// jsonPaths returns every json-tagged path of a Go struct, following pointers, slices and nested
// structs, so the set is comparable with the schema walk above.
func jsonPaths(t reflect.Type, prefix string) map[string]struct{} {
	out := map[string]struct{}{}
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return out
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		out[path] = struct{}{}
		for nested := range jsonPaths(field.Type, path) {
			out[nested] = struct{}{}
		}
	}
	return out
}
