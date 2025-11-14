// Copyright 2025 Flant JSC
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

package values

import (
	"errors"
	"fmt"
	"sync"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
)

// Storage manages package values with layering, patching, and schema validation.
// It maintains both the user config (before merging) and the final result after all layers.
//
// Thread Safety: Protected by mutex for concurrent access.
type Storage struct {
	name string

	valuesPatches []addonutils.ValuesPatch

	schemaStorage *validation.SchemaStorage

	mu sync.Mutex

	// staticValues from:
	//   /packages/values.yaml
	//   /packages/001-package/values.yaml
	staticValues addonutils.Values

	// configValues are user-defined values from package settings
	// These are stored separately before merging with static and openapi values
	configValues addonutils.Values

	// resultValues is the final merged result of all value sources
	// This is what hooks and templates see
	resultValues addonutils.Values
}

// NewStorage creates a new values storage with the specified schemas and static values.
// It initializes the schema storage for validation and calculates initial result values.
//
// Parameters:
//   - name: Package name (will be converted to values key format)
//   - staticValues: Pre-loaded static values from values.yaml
//   - configBytes: OpenAPI config schema (YAML bytes)
//   - valuesBytes: OpenAPI values schema (YAML bytes)
//
// Returns error if schema initialization or initial value calculation fails.
func NewStorage(name string, staticValues addonutils.Values, configBytes, valuesBytes []byte) (*Storage, error) {
	schemaStorage, err := validation.NewSchemaStorage(configBytes, valuesBytes)
	if err != nil {
		return nil, fmt.Errorf("new schema storage: %w", err)
	}

	s := &Storage{
		name:          addonutils.ModuleNameToValuesKey(name),
		staticValues:  staticValues,
		schemaStorage: schemaStorage,
	}

	if err = s.calculateResultValues(); err != nil {
		return nil, fmt.Errorf("calculate values: %w", err)
	}

	return s, nil
}

func (s *Storage) GetValuesChecksum() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.resultValues.Checksum()
}

func (s *Storage) GetConfigChecksum() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.configValues.Checksum()
}

func (s *Storage) GetValues() addonutils.Values {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.resultValues
}

// GetConfigValues returns only user defined values
func (s *Storage) GetConfigValues() addonutils.Values {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.configValues
}

// ApplyConfigValues validates and saves config values
func (s *Storage) ApplyConfigValues(settings addonutils.Values) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if settings == nil {
		settings = addonutils.Values{}
	}

	if err := s.validateConfigValues(settings); err != nil {
		return fmt.Errorf("validate config values: %w", err)
	}

	s.configValues = settings

	return s.calculateResultValues()
}

func (s *Storage) ApplyPatch(patch addonutils.ValuesPatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := addonutils.ValidateHookValuesPatch(patch, s.name); err != nil {
		return fmt.Errorf("validate values patch: %w", err)
	}

	// Apply new patches in Strict mode. Hook should not return 'remove' with nonexistent path.
	patched, changed, err := addonutils.ApplyValuesPatch(s.resultValues, patch, addonutils.Strict)
	if err != nil {
		return fmt.Errorf("try apply values patch: %w", err)
	}

	if !changed {
		return nil
	}

	// Validate updated values against schema
	if validationErr := s.validateValues(patched); validationErr != nil {
		return fmt.Errorf("validate values patch: %w", validationErr)
	}

	s.valuesPatches = addonutils.AppendValuesPatch(s.valuesPatches, patch)
	return s.calculateResultValues()
}

func (s *Storage) calculateResultValues() error {
	merged := mergeLayers(
		addonutils.Values{},
		// init static values (from packages/values.yaml and packages/XXX/values.yaml)
		s.staticValues,

		// from openapi config spec
		s.openapiDefaultsTransformer(validation.ConfigValuesSchema),

		// from package settings
		s.configValues,

		// from openapi values spec
		s.openapiDefaultsTransformer(validation.ValuesSchema),
	)

	// from patches
	// Compact patches so we could execute all at once.
	// Each ApplyValuesPatch execution invokes json.Marshal for values.
	ops := *addonutils.NewValuesPatch()

	for _, patch := range s.valuesPatches {
		ops.Operations = append(ops.Operations, patch.Operations...)
	}

	merged, _, err := addonutils.ApplyValuesPatch(merged, ops, addonutils.IgnoreNonExistentPaths)
	if err != nil {
		return err
	}

	s.resultValues = merged

	return nil
}

func (s *Storage) openapiDefaultsTransformer(schemaType validation.SchemaType) transformer {
	return &applyDefaults{
		SchemaType: schemaType,
		Schemas:    s.schemaStorage.Schemas,
	}
}

func (s *Storage) validateValues(values addonutils.Values) error {
	validatableValues := addonutils.Values{s.name: values}

	return s.schemaStorage.ValidateValues(s.name, validatableValues)
}

func (s *Storage) validateConfigValues(values addonutils.Values) error {
	validatableValues := addonutils.Values{s.name: values}

	if s.schemaStorage.Schemas[validation.ConfigValuesSchema] == nil && len(values) > 0 {
		return errors.New("config schema is not defined but config values were provided")
	}

	return s.schemaStorage.ValidateConfigValues(s.name, validatableValues)
}
