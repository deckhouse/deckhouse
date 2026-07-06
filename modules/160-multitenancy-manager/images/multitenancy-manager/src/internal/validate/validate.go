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

package validate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-jose/go-jose/v4/json"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
)

func ProjectTemplate(template *v1alpha1.ProjectTemplate) error {
	if _, err := LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema); err != nil {
		return fmt.Errorf("load OpenAPI schema from the '%s' project template spec: %w", template.Name, err)
	}

	return nil
}

func Project(project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) error {
	templateOpenAPI, err := LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema)
	if err != nil {
		return fmt.Errorf("load open api schema from the '%s' project template spec: %w", template.Name, err)
	}

	if err = validate.AgainstSchema(transform(templateOpenAPI), project.Spec.Parameters, strfmt.Default); err != nil {
		return fmt.Errorf("the '%s' project is not met the OpenAPI schema for the '%s' project template: %w", project.Name, template.Name, err)
	}

	return nil
}

func LoadSchema(properties map[string]any) (*spec.Schema, error) {
	marshaled, err := json.Marshal(properties)
	if err != nil {
		var jsonErr *json.SyntaxError
		if errors.As(err, &jsonErr) {
			start := max(int(jsonErr.Offset)-10, 0)
			end := min(int(jsonErr.Offset)+10, len(marshaled))
			problemPart := marshaled[start:end]
			err = fmt.Errorf("%w ~ error near '%s' (offset %d)", err, problemPart, jsonErr.Offset)
		}
		return nil, fmt.Errorf("json marshal spec.openAPI: %w", err)
	}

	schema := new(spec.Schema)
	if err = json.Unmarshal(marshaled, schema); err != nil {
		return nil, fmt.Errorf("unmarshal spec.openAPI to spec.Schema: %w", err)
	}

	if err = spec.ExpandSchema(schema, schema, nil); err != nil {
		return nil, fmt.Errorf("expand the schema in spec.openAPI: %w", err)
	}

	return schema, nil
}

// MergeDefaults overlays a parametersSchema's property defaults onto the project-supplied values,
// producing the effective parameters a template renders against. A project value always wins over a
// schema default; nested objects are merged recursively; an additionalProperties (free-form map)
// schema keeps the user's own keys. This is the single source of truth for parameter defaulting,
// shared by the helm (legacy resourcesTemplate) and the structured render paths.
func MergeDefaults(schema *spec.Schema, projectValues map[string]any) map[string]any {
	result := make(map[string]any)

	for property, propertySchema := range schema.Properties {
		if projectValue, exists := projectValues[property]; exists {
			result[property] = projectValue
			if propertySchema.Type.Contains("object") {
				if valueMap, ok := projectValue.(map[string]any); ok {
					result[property] = MergeDefaults(&propertySchema, valueMap)
				}
			}
		} else if propertySchema.Default != nil {
			result[property] = propertySchema.Default
		}

		if propertySchema.Type.Contains("object") {
			if _, ok := result[property]; !ok {
				result[property] = MergeDefaults(&propertySchema, nil)
			}
		}
	}

	// additionalProperties models a free-form map: the user's keys win and the named-property
	// defaults computed above are discarded (the two are not combined).
	if schema.AdditionalProperties != nil {
		mapResult := make(map[string]any)
		for key, value := range projectValues {
			if _, exists := schema.Properties[key]; exists {
				continue
			}
			mapResult[key] = value
		}
		result = mapResult
	}

	return result
}

// ParamPath verifies that a (optionally dotted) fromParam reference resolves to a parameter declared
// in the loaded parametersSchema, and that the parameter's declared type can satisfy the field it is
// bound to. It walks the schema's properties segment by segment; descent stops successfully as soon
// as it reaches a free-form node (additionalProperties or x-kubernetes-preserve-unknown-fields),
// since the remaining segments address user-defined keys the schema cannot enumerate (the type check
// is skipped there — the value shape is user-defined). A segment that is neither a declared property
// nor under a free-form node is reported as undefined.
//
// fieldType is the OpenAPI type the field renders the parameter into ("string", "boolean", "object",
// "array"); empty fieldType or a parameter without a declared type skips the compatibility check.
// Without this check a template binding e.g. a boolean field to a string parameter would be accepted
// at admission and fail only when every project on the template renders.
func ParamPath(schema *spec.Schema, path, fieldType string) error {
	if path == "" {
		return errors.New("empty fromParam reference")
	}

	node := schema
	walked := make([]string, 0, len(path))
	for _, segment := range strings.Split(path, ".") {
		if allowsUnknown(node) {
			return nil
		}
		child, ok := node.Properties[segment]
		if !ok {
			where := "spec.parametersSchema.properties"
			if len(walked) > 0 {
				where = "property '" + strings.Join(walked, ".") + "'"
			}
			return fmt.Errorf("references parameter '%s', but '%s' is not declared in %s", path, segment, where)
		}
		node = &child
		walked = append(walked, segment)
	}

	if fieldType != "" && len(node.Type) > 0 && !node.Type.Contains(fieldType) {
		// "integer" satisfies a "number" field; anything else must match exactly.
		if !(fieldType == "number" && node.Type.Contains("integer")) {
			return fmt.Errorf("references parameter '%s' of type '%s', but the field requires type '%s'",
				path, strings.Join(node.Type, ","), fieldType)
		}
	}
	return nil
}

// allowsUnknown reports whether a schema node accepts keys it does not enumerate, either via
// additionalProperties or the x-kubernetes-preserve-unknown-fields extension.
func allowsUnknown(s *spec.Schema) bool {
	if s == nil {
		return false
	}
	if ap := s.AdditionalProperties; ap != nil && (ap.Allows || ap.Schema != nil) {
		return true
	}
	if ext, ok := s.Extensions["x-kubernetes-preserve-unknown-fields"]; ok {
		if b, ok := ext.(bool); ok && b {
			return true
		}
	}
	return false
}

// transform sets undefined AdditionalProperties to false recursively.
func transform(s *spec.Schema) *spec.Schema {
	if s == nil {
		return nil
	}
	if s.AdditionalProperties == nil {
		s.AdditionalProperties = &spec.SchemaOrBool{
			Allows: false,
		}
	}
	for k, prop := range s.Properties {
		if prop.AdditionalProperties == nil {
			prop.AdditionalProperties = &spec.SchemaOrBool{
				Allows: false,
			}
			ts := prop
			s.Properties[k] = *transform(&ts)
		}
	}
	if s.Items != nil {
		if s.Items.Schema != nil {
			s.Items.Schema = transform(s.Items.Schema)
		}
		for i, item := range s.Items.Schemas {
			ts := item
			s.Items.Schemas[i] = *transform(&ts)
		}
	}
	return s
}
