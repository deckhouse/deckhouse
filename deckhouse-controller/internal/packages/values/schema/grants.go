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

package schema

import (
	"encoding/json"
	"fmt"

	"github.com/go-openapi/spec"
)

// XGrant is the OpenAPI extension key that binds a settings field to a grantable
// cluster resource (see multitenancy-manager AvailableClusterResource). A field
// carrying this extension is defaulted to the project's default granted name when
// left empty, and its value is checked against the project's available names.
const XGrant = "x-deckhouse-grant"

// GrantExtension is the parsed value of an x-deckhouse-grant extension block.
type GrantExtension struct {
	// Resource is the name of the AvailableClusterResource (equals the
	// GrantableClusterResourceDefinition name, e.g. "storageclasses") whose
	// per-project catalog backs this field. The granted resource GVK is owned by
	// the definition and is intentionally not referenced here.
	Resource string `json:"resource"`
}

// GrantRef is a single field in a schema that references a grantable cluster
// resource via the x-deckhouse-grant extension.
type GrantRef struct {
	// Path is the property path to the field, from the schema root, e.g.
	// ["storageClass"] or ["postgres", "storageClass"].
	Path []string
	// Resource is the grantable resource name from the extension.
	Resource string
}

// GrantRefs returns all x-deckhouse-grant references declared in the settings
// schema. It returns nil when no settings schema is registered.
func (s *Storage) GrantRefs() ([]GrantRef, error) {
	scheme := s.schemas[TypeSettings]
	if scheme == nil {
		return nil, nil
	}

	return CollectGrantRefs(scheme)
}

// CollectGrantRefs walks the schema properties recursively and returns every
// field that carries the x-deckhouse-grant extension. It validates that each such
// field is of type string, returning an error otherwise.
func CollectGrantRefs(s *spec.Schema) ([]GrantRef, error) {
	if s == nil {
		return nil, nil
	}

	var refs []GrantRef
	if err := collectGrantRefs(s, nil, &refs); err != nil {
		return nil, err
	}

	return refs, nil
}

func collectGrantRefs(s *spec.Schema, path []string, refs *[]GrantRef) error {
	for name, prop := range s.Properties {
		propPath := append(append([]string(nil), path...), name)

		if ext, ok := extractGrantExtension(&prop); ok {
			if ext.Resource == "" {
				return fmt.Errorf("field %q: %s requires a non-empty 'resource'", joinPath(propPath), XGrant)
			}
			if !prop.Type.Contains("string") {
				return fmt.Errorf("field %q: %s is only supported on 'type: string' fields", joinPath(propPath), XGrant)
			}
			*refs = append(*refs, GrantRef{Path: propPath, Resource: ext.Resource})
		}

		if err := collectGrantRefs(&prop, propPath, refs); err != nil {
			return err
		}
	}

	return nil
}

// extractGrantExtension parses the x-deckhouse-grant extension value from s.
// Returns ok=false when the extension is absent.
func extractGrantExtension(s *spec.Schema) (GrantExtension, bool) {
	if s == nil {
		return GrantExtension{}, false
	}

	raw, ok := s.Extensions[XGrant]
	if !ok || raw == nil {
		return GrantExtension{}, false
	}

	tmpBytes, _ := json.Marshal(raw)

	var ext GrantExtension
	_ = json.Unmarshal(tmpBytes, &ext)

	return ext, true
}

func joinPath(path []string) string {
	res := ""
	for i, p := range path {
		if i > 0 {
			res += "."
		}
		res += p
	}

	return res
}
