// Copyright 2021 Flant JSC
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

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/go-openapi/validate/post"
	"github.com/hashicorp/go-multierror"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type SchemaStore struct {
	cache              map[SchemaIndex]*spec.Schema
	moduleConfigsCache map[string]*spec.Schema
	modulesCache       map[string]struct{}
}

var once sync.Once

var store *SchemaStore

type validateOptions struct {
	commanderMode      bool
	strictUnmarshal    bool
	validateExtensions bool
	requiredSSHHost    bool
}

type ValidateOption func(o *validateOptions)

func ValidateOptionCommanderMode(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.commanderMode = v
	}
}

func ValidateOptionStrictUnmarshal(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.strictUnmarshal = v
	}
}

func ValidateOptionValidateExtensions(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.validateExtensions = v
	}
}

func ValidateOptionRequiredSSHHost(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.requiredSSHHost = v
	}
}

func NewSchemaStore(paths ...string) *SchemaStore {
	paths = append([]string{candiDir}, paths...)

	pathsStr := strings.TrimSpace(os.Getenv("DHCTL_CLI_ADDITIONAL_SCHEMAS_PATHS"))
	if pathsStr != "" {
		pathsNoTrimmed := strings.Split(pathsStr, ",")
		for _, p := range pathsNoTrimmed {
			paths = append(paths, strings.TrimSpace(p))
		}
	}

	return newOnceSchemaStore(paths)
}

func newSchemaStore(schemasDir []string) *SchemaStore {
	st := &SchemaStore{
		cache:              make(map[SchemaIndex]*spec.Schema),
		moduleConfigsCache: make(map[string]*spec.Schema),
		modulesCache:       make(map[string]struct{}),
	}

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}

		switch info.Name() {
		case "init_configuration.yaml",
			"cluster_configuration.yaml",
			"static_cluster_configuration.yaml",
			"cloud_discovery_data.yaml",
			"cloud_provider_discovery_data.yaml",
			"ssh_configuration.yaml",
			"ssh_host_configuration.yaml":
			uploadError := st.UploadByPath(path)
			if uploadError != nil {
				return uploadError
			}
		}

		return nil
	}

	for _, d := range schemasDir {
		err := filepath.Walk(d, walkFunc)
		if err != nil {
			panic(err)
		}
	}

	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		// autoconverger and state exporter do not contains module dir
		log.WarnF("Modules dir not found\n")
		return st
	}

	loadConfigValuesSchema := func(path string, moduleName string) error {
		content, err := os.ReadFile(path)
		if err == nil {
			schema := new(spec.Schema)

			if err := yaml.Unmarshal(content, schema); err != nil {
				return err
			}

			err = spec.ExpandSchema(schema, schema, nil)
			if err != nil {
				return err
			}
			st.moduleConfigsCache[moduleName] = schema
		} else if errors.Is(err, os.ErrNotExist) {
			log.DebugF("openapi spec not found for module %s\n", moduleName)
		} else {
			return err
		}

		return nil
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		moduleName := strings.TrimLeft(name, "01234567890-")
		st.modulesCache[moduleName] = struct{}{}
		p := path.Join(modulesDir, name, "openapi", "config-values.yaml")
		if err := loadConfigValuesSchema(p, moduleName); err != nil {
			// We don't expect panic here our logger does not support log.Fatal
			panic(err)
		}
	}

	err = loadConfigValuesSchema(path.Join(globalHooksModule, "openapi", "config-values.yaml"), "global")
	if err != nil {
		// We don't expect panic here our logger does not support log.Fatal
		panic(err)
	}

	return st
}

func newOnceSchemaStore(schemasDir []string) *SchemaStore {
	once.Do(func() {
		store = newSchemaStore(schemasDir)
	})
	return store
}

func (s *SchemaStore) Get(index *SchemaIndex) *spec.Schema {
	return s.cache[*index]
}

func (s *SchemaStore) HasSchemaForModuleConfig(name string) bool {
	_, ok := s.moduleConfigsCache[name]
	return ok
}

func (s *SchemaStore) GetModuleConfigVersion(name string) int {
	schema, ok := s.moduleConfigsCache[name]
	if ok {
		if len(schema.VendorExtensible.Extensions) > 0 {
			v, ok := schema.VendorExtensible.Extensions["x-config-version"]
			if ok {
				return int(v.(float64))
			}
		}
		return 1
	}

	return 1
}

func (s *SchemaStore) Validate(doc *[]byte, opts ...ValidateOption) (*SchemaIndex, error) {
	var index SchemaIndex

	err := yaml.Unmarshal(*doc, &index)
	if err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	err = s.ValidateWithIndex(&index, doc, opts...)
	return &index, err
}

// v1alpha1 was changed to v1 in-place. To keep the backward compatibility we check old and new schemas
func (s *SchemaStore) getV1alpha1CompatibilitySchema(index *SchemaIndex) *spec.Schema {
	schema := s.Get(index)
	if schema == nil && index.Version == "deckhouse.io/v1alpha1" {
		index.Version = "deckhouse.io/v1"
		return s.Get(index)
	}

	return schema
}

// ValidateWithIndex
// validate one document with schema
// two separated kinds will validate: ModuleConfig and another kinds with schema eg InitConfiguration
// if schema not fount then return ErrSchemaNotFound
// if schema not found for ModuleConfig then return ErrSchemaNotFound also
func (s *SchemaStore) ValidateWithIndex(index *SchemaIndex, doc *[]byte, opts ...ValidateOption) error {
	options := applyOptions(opts...)
	if !index.IsValid() {
		return fmt.Errorf(
			"document must contain \"kind\" and \"apiVersion\" fields:\n\tapiVersion: %s\n\tkind: %s\n\n%s",
			index.Version, index.Kind, string(*doc),
		)
	}

	docForValidate := *doc

	var schema *spec.Schema

	if index.Kind == "ModuleConfig" {
		mc := ModuleConfig{}
		if err := yaml.Unmarshal(*doc, &mc); err != nil {
			return err
		}
		mcName := mc.GetName()
		if mc.Spec.Enabled == nil && mcName != "global" {
			// we need return error because on top level we want filter module configs from modulesources and move into resources
			// global is special mc without module
			return fmt.Errorf("Enabled field for module config %s shoud set to true or false", mcName)
		}

		if _, ok := s.modulesCache[mcName]; !ok && mcName != "global" {
			log.DebugF("Module %s wasn't found. Probably it is module from modulesources. Skip it\n", mc.GetName())
			return ErrSchemaNotFound
		}

		if len(mc.Spec.Settings) == 0 {
			return nil
		}

		var ok bool
		schema, ok = s.moduleConfigsCache[mcName]
		if !ok {
			log.DebugF("Schema for module config %s wasn't found. Probably it is module from modulesources. Skip it\n", mc.GetName())
			return fmt.Errorf("Schema for module config %s not found", mcName)
		}

		if mc.Spec.Version == 0 {
			return fmt.Errorf("Version field for module config %s shoud set", mcName)
		}

		var err error
		docForValidate, err = yaml.Marshal(mc.Spec.Settings)
		if err != nil {
			return fmt.Errorf("Setting for validation module config failed: %v", err)
		}
	} else {
		schema = s.getV1alpha1CompatibilitySchema(index)
	}

	if schema == nil {
		log.DebugF("No schema for index %s. Skip it\n", index.String())
		// we need return error because on top level we want filter documents without index and move into resources
		return ErrSchemaNotFound
	}

	isValid, err := openAPIValidate(&docForValidate, schema, options)
	if !isValid {
		if options.commanderMode {
			return fmt.Errorf("%q document validation failed: %w", index.String(), err)
		}
		return fmt.Errorf("Document validation failed:\n---\n%s\n\n%w", string(*doc), err)
	}

	*doc = docForValidate

	return nil
}

func (s *SchemaStore) UploadByPath(path string) error {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Loading schema file: %v", err)
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

func openAPIValidate(dataObj *[]byte, schema *spec.Schema, options validateOptions) (isValid bool, multiErr error) {
	validator := validate.NewSchemaValidator(schema, nil, "", strfmt.Default)

	var blank map[string]interface{}

	if options.strictUnmarshal {
		err := yaml.UnmarshalStrict(*dataObj, &blank)
		if err != nil {
			return false, fmt.Errorf("openAPIValidate json unmarshal strict: %v", err)
		}
	} else {
		err := yaml.Unmarshal(*dataObj, &blank)
		if err != nil {
			return false, fmt.Errorf("openAPIValidate json unmarshal: %v", err)
		}
	}

	result := validator.Validate(blank)
	if !result.IsValid() {
		var allErrs *multierror.Error
		allErrs = multierror.Append(allErrs, result.Errors...)

		return false, allErrs.ErrorOrNil()
	}

	if options.validateExtensions {
		if err := validateExtensions(*dataObj, *schema); err != nil {
			return false, err
		}
	}

	// Add default values from openAPISpec
	post.ApplyDefaults(result)
	*dataObj, _ = json.Marshal(result.Data())

	return true, nil
}

func ValidateDiscoveryData(config *[]byte, paths []string, opts ...ValidateOption) (bool, error) {
	schemaStore := NewSchemaStore(paths...)

	_, err := schemaStore.Validate(config, opts...)
	if err != nil {
		return false, fmt.Errorf("Loading schema file: %v", err)
	}

	return true, nil
}

func applyOptions(opts ...ValidateOption) validateOptions {
	options := validateOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}
