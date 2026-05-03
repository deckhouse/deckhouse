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
	"errors"
	"fmt"
	"reflect"

	"github.com/flant/addon-operator/pkg/utils"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/swag/loading"
	"github.com/go-openapi/validate"
	"github.com/hashicorp/go-multierror"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema/cel"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema/transformers"
)

// Type identifies which schema variant is used for a given validation context.
type Type string

const (
	// TypeSettings is the schema for user-supplied configuration values (config.yaml).
	TypeSettings Type = "config"
	// TypeValues is the schema for the full set of internal module values.
	TypeValues Type = "values"
	// TypeHelm is derived from TypeValues with x-required-for-helm fields promoted to
	// required, making those fields mandatory during Helm chart rendering.
	TypeHelm Type = "helm"
)

func init() {
	// Add loader to override swag.BytesToYAML marshaling into yaml.MapSlice.
	// This type doesn't support map merging feature of YAML anchors. So additional
	// loader is required to unmarshal into ordinary interface{} before converting to JSON.
	loads.AddLoader(swag.YAMLMatcher, YAMLDocLoader)
}

// YAMLDocLoader loads a yaml document from either http or a file and converts it to json.
func YAMLDocLoader(path string, opts ...loading.Option) (json.RawMessage, error) {
	data, err := loading.LoadFromFileOrHTTP(path, opts...)
	if err != nil {
		return nil, err
	}

	return yamlBytesToJSONDoc(data)
}

// yamlBytesToJSONDoc is a replacement of swag.YAMLData and YAMLDoc to Unmarshal into interface{}.
// swag.BytesToYAML uses yaml.MapSlice to unmarshal YAML. This type doesn't support map merge of YAML anchors.
func yamlBytesToJSONDoc(data []byte) (json.RawMessage, error) {
	var yamlObj interface{}
	if err := yaml.Unmarshal(data, &yamlObj); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %v", err)
	}

	doc, err := swag.YAMLToJSON(yamlObj)
	if err != nil {
		return nil, fmt.Errorf("yaml to json: %v", err)
	}

	return doc, nil
}

// Storage holds compiled OpenAPI schemas for a package, keyed by Type.
// All schemas are pre-processed and ready for repeated validation calls.
type Storage struct {
	schemas map[Type]*spec.Schema
}

// NewStorage parses settings and values YAML schema documents, applies the
// required transformations, and returns a Storage ready for use.
func NewStorage(settings, values []byte) (*Storage, error) {
	schemas, err := prepareSchemas(settings, values)
	if err != nil {
		return nil, fmt.Errorf("prepare schemas: %w", err)
	}

	return &Storage{schemas: schemas}, err
}

// GetSchema returns schema by Type
func (s *Storage) GetSchema(schemaType Type) *spec.Schema {
	return s.schemas[schemaType]
}

// prepareSchemas loads schemas for config values, values and helm values.
func prepareSchemas(settings, values []byte) (map[Type]*spec.Schema, error) {
	res := make(map[Type]*spec.Schema)
	if len(settings) > 0 {
		schemaObj, err := loadSchemaFromBytes(settings)
		if err != nil {
			return nil, fmt.Errorf("load '%s' schema: %w", TypeSettings, err)
		}

		res[TypeSettings] = transformers.Transform(
			schemaObj,
			&transformers.AdditionalProperties{},
		)
	}

	if len(values) > 0 {
		schemaObj, err := loadSchemaFromBytes(values)
		if err != nil {
			return nil, fmt.Errorf("load '%s' schema: %w", TypeValues, err)
		}

		res[TypeValues] = transformers.Transform(
			schemaObj,
			&transformers.Extend{Parent: res[TypeValues]},
			&transformers.AdditionalProperties{},
		)

		res[TypeHelm] = transformers.Transform(
			schemaObj,
			// Copy schema object.
			&transformers.Copy{},
			// Transform x-required-for-helm
			&transformers.RequiredForHelm{},
		)
	}

	return res, nil
}

// loadSchemaFromBytes returns spec.Schema object loaded from YAML bytes.
func loadSchemaFromBytes(openApiContent []byte) (*spec.Schema, error) {
	jsonDoc, err := yamlBytesToJSONDoc(openApiContent)
	if err != nil {
		return nil, err
	}

	s := new(spec.Schema)
	if err = json.Unmarshal(jsonDoc, s); err != nil {
		return nil, fmt.Errorf("json unmarshal: %v", err)
	}

	if err = spec.ExpandSchema(s, s, nil /*new(noopResCache)*/); err != nil {
		return nil, fmt.Errorf("expand schema: %v", err)
	}

	return s, nil
}

// Validate validates values against the schema registered for valuesType.
// It extracts the value under root and runs both CEL rule checks and JSON-Schema
// validation. Returns nil if no schema is registered for valuesType.
func (s *Storage) Validate(valuesType Type, root string, values utils.Values) error {
	scheme := s.schemas[valuesType]
	if scheme == nil {
		return nil
	}

	obj, ok := values[root]
	if !ok {
		return fmt.Errorf("root key '%s' not found in values", root)
	}

	return validateObject(obj, scheme, root)
}

// validateObject runs CEL rule checks and JSON-Schema validation on dataObj
// using schema s, attributing errors to rootName in messages.
// dataObj must be utils.Values or map[string]interface{}.
// See https://github.com/kubernetes/apiextensions-apiserver/blob/1bb376f70aa2c6f2dec9a8c7f05384adbfac7fbb/pkg/apiserver/validation/validation.go#L47
func validateObject(dataObj interface{}, s *spec.Schema, rootName string) error {
	validator := validate.NewSchemaValidator(s, nil, rootName, strfmt.Default) // , validate.DisableObjectArrayTypeCheck(true)

	switch v := dataObj.(type) {
	case utils.Values:
		dataObj = map[string]interface{}(v)

	case map[string]interface{}:
	// pass

	default:
		return fmt.Errorf("validated data object have to be utils.Values or map[string]interface{}, got %v instead", reflect.TypeOf(v))
	}

	// Validate values against x-deckhouse-validation rules.
	if values, ok := dataObj.(map[string]interface{}); ok {
		validationErrs, err := cel.Validate(s, values)
		if err != nil {
			return err
		}
		if len(validationErrs) > 0 {
			return errors.Join(validationErrs...)
		}
	}

	result := validator.Validate(dataObj)
	if result.IsValid() {
		return nil
	}

	var allErrs *multierror.Error
	for _, err := range result.Errors {
		allErrs = multierror.Append(allErrs, err)
	}
	// NOTE: no validation errors, but config is not valid!
	if allErrs == nil || allErrs.Len() == 0 {
		allErrs = multierror.Append(allErrs, fmt.Errorf("configuration is not valid"))
	}

	return allErrs.ErrorOrNil()
}
