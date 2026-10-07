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
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/go-jose/go-jose/v4/json"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
)

func ProjectTemplate(template *v1alpha2.ProjectTemplate) error {
	if _, err := LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema); err != nil {
		return fmt.Errorf("load OpenAPI schema from the '%s' project template spec: %w", template.Name, err)
	}

	return nil
}

// Project validates the project parameters against the parametersSchema of its template.
func Project(project *v1alpha3.Project, template *v1alpha2.ProjectTemplate) error {
	templateOpenAPI, err := LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema)
	if err != nil {
		return fmt.Errorf("load open api schema from the '%s' project template spec: %w", template.Name, err)
	}

	parameters := parametersToValidate(project, templateOpenAPI)
	if err = validate.AgainstSchema(transform(templateOpenAPI), parameters, strfmt.Default); err != nil {
		return fmt.Errorf("the '%s' project is not met the OpenAPI schema for the '%s' project template: %w", project.Name, template.Name, err)
	}

	return nil
}

// The parameters under which a parametersSchema written for the v1alpha2 Project layout declares
// what are now the spec.administrators and spec.quota standard fields.
const (
	administratorsParameter = "administrators"
	resourceQuotaParameter  = "resourceQuota"
)

// parametersToValidate returns the project parameters as the template schema sees them. The
// v1alpha2 -> v1alpha3 conversion moves administrators and resourceQuota out of spec.parameters of
// every project, while a schema copied from the default template of that layout still declares and
// requires both. When the schema declares one of them, it is put back from its standard field the
// way the v1alpha3 -> v1alpha2 conversion does, so the project validates as it did in that layout;
// a schema that does not declare it never gets it. A standard field that is empty is gone after the
// first write of the Project (the fields are omitempty), so a required one the parameters do not
// hold either is put back in its empty form. The project is left as it is: the renderer reads its
// parameters without the two.
func parametersToValidate(project *v1alpha3.Project, schema *spec.Schema) map[string]any {
	_, declaresAdministrators := schema.Properties[administratorsParameter]
	quotaSchema, declaresQuota := schema.Properties[resourceQuotaParameter]
	_, givenAdministrators := project.Spec.Parameters[administratorsParameter]
	_, givenQuota := project.Spec.Parameters[resourceQuotaParameter]

	takesAdministrators := declaresAdministrators &&
		(project.Spec.Administrators != nil || !givenAdministrators && slices.Contains(schema.Required, administratorsParameter))
	takesQuota := declaresQuota &&
		(project.Spec.Quota != nil || !givenQuota && slices.Contains(schema.Required, resourceQuotaParameter))
	if !takesAdministrators && !takesQuota {
		return project.Spec.Parameters
	}

	parameters := make(map[string]any, len(project.Spec.Parameters)+2)
	maps.Copy(parameters, project.Spec.Parameters)
	if takesAdministrators {
		parameters[administratorsParameter] = legacyAdministrators(project.Spec.Administrators)
	}
	if takesQuota {
		parameters[resourceQuotaParameter] = legacyResourceQuota(project.Spec.Quota, &quotaSchema)
	}

	return parameters
}

// legacyAdministrators lays the administrators out as the administrators parameter held them.
func legacyAdministrators(administrators []v1alpha3.Administrator) []any {
	result := make([]any, 0, len(administrators))
	for _, administrator := range administrators {
		result = append(result, map[string]any{"subject": administrator.Kind, "name": administrator.Name})
	}
	return result
}

// legacyResourceQuota lays a quota out as the resourceQuota parameter held it. Only a "requests." or
// "limits." prefix is nesting, and the rest of the key is kept whole ("requests.nvidia.com/gpu" is
// requests["nvidia.com/gpu"]); every other key stays flat. A flat "requests" or "limits" gives way
// to the nested form whatever the order of the keys, as in the conversion. schema is the
// resourceQuota schema and decides how each value is written (see legacyQuotaValue).
func legacyResourceQuota(quota corev1.ResourceList, schema *spec.Schema) map[string]any {
	result := make(map[string]any, len(quota))
	for name, quantity := range quota {
		group, resourceName, isNested := strings.Cut(string(name), ".")
		if isNested && (group == "requests" || group == "limits") {
			values, isObject := result[group].(map[string]any)
			if !isObject {
				values = map[string]any{}
				result[group] = values
			}
			values[resourceName] = legacyQuotaValue(quantity, propertySchema(propertySchema(schema, group), resourceName))
			continue
		}

		if _, isObject := result[string(name)].(map[string]any); !isObject {
			result[string(name)] = legacyQuotaValue(quantity, propertySchema(schema, string(name)))
		}
	}
	return result
}

// legacyQuotaValue writes a quantity in a form the schema of its parameter takes. The typed quota no
// longer says how the parameter held it: a whole number may have been a JSON number or digits in a
// string, and anything else a string such as 1500m or 2Gi. So the forms are tried in the order the
// conversion writes them, and the first one the schema takes is used; with none, the first form is
// what fails. The canonical string alone would not do: the canonical form of 1000 is 1k, which a
// schema with the pattern ^[0-9]+m?$ refuses. A property the schema does not describe gets the
// canonical string.
func legacyQuotaValue(quantity resource.Quantity, schema *spec.Schema) any {
	if schema == nil {
		return quantity.String()
	}
	forms := quotaValueForms(quantity)
	for _, form := range forms {
		if validate.AgainstSchema(schema, form, strfmt.Default) == nil {
			return form
		}
	}
	return forms[0]
}

// quotaValueForms lists the JSON forms the resourceQuota parameter may have held a quantity in: a
// whole number as a JSON number, as digits in a string and as its canonical string; anything else as
// its canonical string and as a JSON number.
func quotaValueForms(quantity resource.Quantity) []any {
	if value, ok := quantity.AsInt64(); ok {
		return []any{value, strconv.FormatInt(value, 10), quantity.String()}
	}
	return []any{quantity.String(), quantity.AsApproximateFloat64()}
}

// propertySchema returns the schema of a property: the declared one, or the additionalProperties
// schema of a map. It is nil when the parent is nil or says nothing about the property.
func propertySchema(parent *spec.Schema, name string) *spec.Schema {
	if parent == nil {
		return nil
	}
	if property, ok := parent.Properties[name]; ok {
		return &property
	}
	if parent.AdditionalProperties != nil {
		return parent.AdditionalProperties.Schema
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
// schema default; nested objects are merged recursively. The declared properties are merged the same
// way whatever additionalProperties says; a schema whose additionalProperties allows other keys (a
// free-form map) also keeps the project's undeclared keys as they are. This is the single source of
// truth for parameter defaulting.
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

	// additionalProperties models a free-form map: the project's undeclared keys are added next to the
	// declared properties merged above, which already hold the project's values for their own keys.
	if allowsAdditional(schema) {
		for key, value := range projectValues {
			if _, declared := schema.Properties[key]; !declared {
				result[key] = value
			}
		}
	}

	return result
}

// ParamPath verifies that a (optionally dotted) fromParam reference resolves to a parameter declared
// in the loaded parametersSchema, and that the parameter's declared type can satisfy the field it is
// bound to. It walks the schema's properties segment by segment and follows a declared property
// whatever additionalProperties says, so a declared parameter is type-checked in a free-form node
// too. Descent stops successfully only at a segment that a free-form node (additionalProperties or
// x-kubernetes-preserve-unknown-fields) does not declare, since that segment and the remaining ones
// address user-defined keys the schema cannot enumerate (the type check is skipped there — the value
// shape is user-defined). A segment that is neither a declared property nor under a free-form node is
// reported as undefined.
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
		child, declared := node.Properties[segment]
		if !declared {
			if allowsUnknown(node) {
				return nil
			}
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
		if fieldType != "number" || !node.Type.Contains("integer") {
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
	if allowsAdditional(s) {
		return true
	}
	if ext, ok := s.Extensions["x-kubernetes-preserve-unknown-fields"]; ok {
		if b, ok := ext.(bool); ok && b {
			return true
		}
	}
	return false
}

// allowsAdditional reports whether a schema node's additionalProperties accepts keys the node does
// not declare, that is additionalProperties is true or a schema rather than absent or false.
func allowsAdditional(s *spec.Schema) bool {
	ap := s.AdditionalProperties
	return ap != nil && (ap.Allows || ap.Schema != nil)
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
