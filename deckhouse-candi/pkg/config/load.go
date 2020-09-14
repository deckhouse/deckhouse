package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/go-openapi/validate/post"
	"github.com/hashicorp/go-multierror"
	"sigs.k8s.io/yaml"
)

type SchemaStore struct {
	cache map[SchemaIndex]*spec.Schema
}

var once sync.Once

var store *SchemaStore

func NewSchemaStore() *SchemaStore {
	once.Do(func() {
		store = &SchemaStore{make(map[SchemaIndex]*spec.Schema)}
		err := filepath.Walk(candiDir, func(path string, info os.FileInfo, err error) error {
			if strings.HasSuffix(path, providerSchemaFilenameSuffix) || info.Name() == "cloud_discovery_data.yaml" {
				uploadError := store.UploadByPath(path)
				if uploadError != nil {
					return uploadError
				}
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
	})
	return store
}

func (s *SchemaStore) Get(index *SchemaIndex) *spec.Schema {
	return s.cache[*index]
}

func (s *SchemaStore) Validate(doc *[]byte) (*SchemaIndex, error) {
	var index SchemaIndex

	err := yaml.Unmarshal(*doc, &index)
	if err != nil {
		return nil, fmt.Errorf("json unmarshal: %v", err)
	}

	err = s.ValidateWithIndex(&index, doc)
	return &index, err
}

func (s *SchemaStore) ValidateWithIndex(index *SchemaIndex, doc *[]byte) error {
	if !index.IsValid() {
		return fmt.Errorf("invalid index: %v", index)
	}

	isValid, err := openAPIValidate(doc, s.Get(index))
	if !isValid {
		return fmt.Errorf("document validation failed:\n\n%s\n\n%w", string(*doc), err)
	}
	return nil
}

func (s *SchemaStore) UploadByPath(path string) error {
	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("loading schema file: %v", err)
	}

	return s.upload(fileContent)
}

func (s *SchemaStore) upload(fileContent []byte) error {
	openAPISchema := new(OpenAPISchema)
	if err := yaml.UnmarshalStrict(fileContent, openAPISchema); err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	for _, parsedSchema := range openAPISchema.Versions {
		schema := new(spec.Schema)

		d, err := json.Marshal(parsedSchema.Schema)
		if err != nil {
			return fmt.Errorf("expand the schema: %v", err)
		}

		if err := json.Unmarshal(d, schema); err != nil {
			return fmt.Errorf("json marshal: %v", err)
		}

		err = spec.ExpandSchema(schema, schema, nil)
		if err != nil {
			return fmt.Errorf("expand the schema: %v", err)
		}

		s.cache[SchemaIndex{Kind: openAPISchema.Kind, Version: parsedSchema.Version}] = schema
	}

	return nil
}

func openAPIValidate(dataObj *[]byte, schema *spec.Schema) (isValid bool, multiErr error) {
	validator := validate.NewSchemaValidator(schema, nil, "", strfmt.Default)

	var blank map[string]interface{}

	err := yaml.Unmarshal(*dataObj, &blank)
	if err != nil {
		return false, fmt.Errorf("openAPIValidate json unmarshal: %v", err)
	}

	result := validator.Validate(blank)
	if result.IsValid() {
		// Add default values from openAPISpec
		post.ApplyDefaults(result)
		*dataObj, _ = json.Marshal(result.Data())

		return true, nil
	}

	var allErrs *multierror.Error
	allErrs = multierror.Append(allErrs, result.Errors...)

	return false, allErrs.ErrorOrNil()
}
