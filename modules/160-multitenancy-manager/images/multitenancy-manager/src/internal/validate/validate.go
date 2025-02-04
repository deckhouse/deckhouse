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

	"github.com/go-jose/go-jose/v4/json"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
)

func ProjectTemplate(template *v1alpha1.ProjectTemplate) error {
	if _, err := LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema); err != nil {
		return fmt.Errorf("load OpenAPI schema from the '%s' project template spec: %w", template.Name, err)
	}

	return nil
}

func Project(project *v1alpha2.Project, template *v1alpha1.ProjectTemplate) error {
	templateOpenAPI, err := LoadSchema(template.Spec.ParametersSchema.OpenAPIV3Schema)
	if err != nil {
		return fmt.Errorf("load open api schema from the '%s' project template spec: %w", template.Name, err)
	}

	if err = validate.AgainstSchema(transform(templateOpenAPI), project.Spec.Parameters, strfmt.Default); err != nil {
		return fmt.Errorf("the '%s' project is not met the OpenAPI schema for the '%s' project template: %w", project.Name, template.Name, err)
	}

	return nil
}

func LoadSchema(properties map[string]interface{}) (*spec.Schema, error) {
	marshaled, err := json.Marshal(properties)
	if err != nil {
		var jsonErr *json.SyntaxError
		if errors.As(err, &jsonErr) {
			problemPart := marshaled[jsonErr.Offset-10 : jsonErr.Offset+10]
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
