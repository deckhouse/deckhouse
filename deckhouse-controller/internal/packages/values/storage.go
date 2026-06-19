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
	"strings"
	"sync"

	addonutils "github.com/flant/addon-operator/pkg/utils"

	sdkutils "github.com/deckhouse/module-sdk/pkg/utils"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema"
)

// Storage manages package values with layering, patching, and schema validation.
// It maintains both the user config (before merging) and the final result after all layers.
//
// Thread Safety: Protected by mutex for concurrent access.
type Storage struct {
	name string

	valuesPatches []addonutils.ValuesPatch

	schemaStorage *schema.Storage

	mu sync.Mutex

	// staticValues from:
	//   /packages/values.yaml
	//   /packages/001-package/values.yaml
	staticValues addonutils.Values

	// settings are user-defined values from package settings
	// These are stored separately before merging with static and openapi values
	settings addonutils.Values

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
//   - settingsBytes: OpenAPI config schema (YAML bytes)
//   - valuesBytes: OpenAPI values schema (YAML bytes)
//
// Returns error if schema initialization or initial value calculation fails.
func NewStorage(name string, staticValues addonutils.Values, settingsBytes, valuesBytes []byte) (*Storage, error) {
	schemaStorage, err := schema.NewStorage(settingsBytes, valuesBytes)
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

// GetValuesChecksum returns a checksum of the final merged values.
// Used to detect when values have changed (e.g., for triggering hook reruns).
func (s *Storage) GetValuesChecksum() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.resultValues.Checksum()
}

// GetSettingsChecksum returns a checksum of only the user-defined config values.
// Unlike GetValuesChecksum, this excludes static values, schema defaults, and patches.
func (s *Storage) GetSettingsChecksum() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.settings.Checksum()
}

// GetValues returns the final merged values that hooks and templates see.
// This includes all layers: static values, schema defaults, user config, and patches.
func (s *Storage) GetValues() addonutils.Values {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.resultValues
}

// GetSettings returns config values with config-schema defaults applied.
// Available in templates as .Application.Settings or .Module.Settings.
func (s *Storage) GetSettings() addonutils.Values {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings := s.settings
	if len(settings) == 0 {
		settings = addonutils.Values{}
	}

	return s.openapiDefaultsTransformer(schema.TypeSettings).Transform(settings)
}

// ApplySettingsDefaults returns a copy of the provided values with defaults
// from the config OpenAPI schema applied. Does not modify stored values.
func (s *Storage) ApplySettingsDefaults(settings addonutils.Values) addonutils.Values {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(settings) == 0 {
		settings = addonutils.Values{}
	}

	return s.openapiDefaultsTransformer(schema.TypeSettings).Transform(settings)
}

// ValidateSettings validates values against the config OpenAPI schema.
// Does not modify the stored values - use ApplySettings to persist.
func (s *Storage) ValidateSettings(settings addonutils.Values) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(settings) == 0 {
		settings = addonutils.Values{}
	}

	if err := s.validateSettings(settings); err != nil {
		return fmt.Errorf("validate config values: %w", err)
	}

	return nil
}

// ApplySettings validates and saves user-defined config values.
// After saving, recalculates the result values with all layers merged.
func (s *Storage) ApplySettings(settings addonutils.Values) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(settings) == 0 {
		settings = addonutils.Values{}
	}

	if err := s.validateSettings(settings); err != nil {
		return fmt.Errorf("validate config values: %w", err)
	}

	s.settings = settings

	return s.calculateResultValues()
}

// ApplyValuesPatch applies a JSON patch to the result values.
// Patches are accumulated and reapplied on each recalculation.
// Used by hooks to dynamically modify values at runtime.
func (s *Storage) ApplyValuesPatch(patch addonutils.ValuesPatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Apply new patches in Strict mode. Hook should not return 'remove' with nonexistent path.
	patched, changed, err := addonutils.ApplyValuesPatch(s.resultValues, patch, addonutils.Strict)
	if err != nil {
		return fmt.Errorf("try apply values patch: %w", err)
	}

	if !changed {
		return nil
	}

	// Validate updated values against schema
	if err = s.validateValues(patched); err != nil {
		return fmt.Errorf("validate values patch: %w", err)
	}

	s.valuesPatches = addonutils.AppendValuesPatch(s.valuesPatches, patch)
	return s.calculateResultValues()
}

// ApplyHookValuesPatch rewrites a hook-produced values patch from the
// addon-operator hook layout into this storage's flat layout, then applies it.
//
// Hooks written for addon-operator receive values nested under a single
// top-level key (the module's camelCase values key, or "global") and emit patch
// operations with paths like "/<key>/internal/x". This storage keeps the
// package's own values flat, so the "/<key>" prefix is stripped before applying.
// Operations targeting a different subtree (e.g. a module hook patching
// "/global/...") are ignored because that subtree is owned by another storage.
func (s *Storage) ApplyHookValuesPatch(patch addonutils.ValuesPatch, key string) error {
	jsonPrefix := "/" + key

	rewritten := addonutils.ValuesPatch{
		Operations: make([]*sdkutils.ValuesPatchOperation, 0, len(patch.Operations)),
	}

	for _, op := range patch.Operations {
		if op == nil {
			continue
		}

		switch {
		case op.Path == jsonPrefix:
			// Operation targets the package root itself; map it to the document root.
			rewritten.Operations = append(rewritten.Operations, &sdkutils.ValuesPatchOperation{
				Op:    op.Op,
				Path:  "",
				Value: op.Value,
			})
		case strings.HasPrefix(op.Path, jsonPrefix+"/"):
			rewritten.Operations = append(rewritten.Operations, &sdkutils.ValuesPatchOperation{
				Op:    op.Op,
				Path:  strings.TrimPrefix(op.Path, jsonPrefix),
				Value: op.Value,
			})
		default:
			// Outside the package's subtree (e.g. global.* from a module hook).
			continue
		}
	}

	if len(rewritten.Operations) == 0 {
		return nil
	}

	return s.ApplyValuesPatch(rewritten)
}

// calculateResultValues merges all value layers and applies patches.
// Layer order: static -> config schema defaults -> user config -> values schema defaults -> patches
func (s *Storage) calculateResultValues() error {
	merged := mergeLayers(
		addonutils.Values{},
		// init static values (from packages/values.yaml and packages/XXX/values.yaml)
		s.staticValues,

		// from openapi config spec
		s.openapiDefaultsTransformer(schema.TypeSettings),

		// from package settings
		s.settings,

		// from openapi values spec
		s.openapiDefaultsTransformer(schema.TypeValues),
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

// openapiDefaultsTransformer creates a transformer that applies defaults from an OpenAPI schema.
func (s *Storage) openapiDefaultsTransformer(schemaType schema.Type) transformer {
	return &applyDefaults{
		schema: s.schemaStorage.GetSchema(schemaType),
	}
}

// validateValues validates values against the values OpenAPI schema.
// Previously merged result values are passed as the old state so that
// x-deckhouse-validations transition rules (rules referencing oldSelf) can
// catch attempts to mutate immutable fields via values patches.
func (s *Storage) validateValues(values addonutils.Values) error {
	validatableValues := addonutils.Values{s.name: values}

	var oldValidatable addonutils.Values
	if s.resultValues != nil {
		oldValidatable = addonutils.Values{s.name: s.resultValues}
	}

	return s.schemaStorage.ValidateTransition(schema.TypeValues, s.name, validatableValues, oldValidatable)
}

// validateConfigValues validates values against the config OpenAPI schema.
// Returns error if values are provided but no config schema is defined.
// Previously stored user settings are passed as the old state so that
// x-deckhouse-validations transition rules (rules referencing oldSelf) can
// implement immutability and other update-time invariants on ModuleConfig.
func (s *Storage) validateSettings(values addonutils.Values) error {
	validatableValues := addonutils.Values{s.name: values}

	if s.schemaStorage.GetSchema(schema.TypeSettings) == nil && len(values) > 0 {
		return errors.New("config schema is not defined but config values were provided")
	}

	var oldValidatable addonutils.Values
	if s.settings != nil {
		oldValidatable = addonutils.Values{s.name: s.settings}
	}

	return s.schemaStorage.ValidateTransition(schema.TypeSettings, s.name, validatableValues, oldValidatable)
}
