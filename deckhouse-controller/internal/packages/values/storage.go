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

	"github.com/ettle/strcase"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag/conv"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
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

	// dynamicDefaults are runtime-resolved defaults (e.g. from cluster resource
	// grants) applied to empty settings fields before the user config layer, so
	// the user can still override them.
	dynamicDefaults []DynamicDefault

	// resultValues is the final merged result of all value sources
	// This is what hooks and templates see
	resultValues addonutils.Values
}

// DynamicDefault is a runtime-resolved default value for a settings field located
// at Path (a property path from the package values root). It is applied only when
// the field is empty, and always before the user config layer.
type DynamicDefault struct {
	Path  []string
	Value string
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
		name:          strcase.ToCamel(name),
		staticValues:  staticValues,
		schemaStorage: schemaStorage,
	}

	if err = s.calculateResultValues(); err != nil {
		return nil, fmt.Errorf("calculate values: %w", err)
	}

	return s, nil
}

// GrantRefs returns the x-deckhouse-grantable-resource references declared in the
// settings schema. Returns nil when no settings schema or no such references exist.
func (s *Storage) GrantRefs() ([]schema.GrantRef, error) {
	return s.schemaStorage.GrantRefs()
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

// SetDynamicDefaults stores runtime-resolved defaults and recalculates the
// result values. Defaults are applied to empty settings fields before the user
// config layer, so an explicit user value always wins.
func (s *Storage) SetDynamicDefaults(defaults []DynamicDefault) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dynamicDefaults = defaults

	return s.calculateResultValues()
}

// calculateResultValues merges all value layers and applies patches.
// Layer order: static -> config schema defaults -> dynamic defaults -> user config -> values schema defaults -> patches
func (s *Storage) calculateResultValues() error {
	merged := mergeLayers(
		addonutils.Values{},
		// init static values (from packages/values.yaml and packages/XXX/values.yaml)
		s.staticValues,

		// from openapi config spec
		s.openapiDefaultsTransformer(schema.TypeSettings),

		// runtime-resolved defaults (e.g. cluster resource grants) for empty fields
		valuesTransform(s.applyDynamicDefaults),

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

// applyDynamicDefaults returns the subset of dynamic defaults whose target field
// is currently empty in current. Only these are merged, so values already
// provided by static values or schema defaults are preserved.
func (s *Storage) applyDynamicDefaults(current addonutils.Values) addonutils.Values {
	out := addonutils.Values{}

	for _, d := range s.dynamicDefaults {
		if d.Value == "" || len(d.Path) == 0 {
			continue
		}
		if !isEmptyAtPath(current, d.Path) {
			continue
		}
		setAtPath(out, d.Path, d.Value)
	}

	return out
}

// isEmptyAtPath reports whether the value at path in values is missing or an
// empty string. Any intermediate non-map node is treated as non-empty (the
// default cannot be placed there).
func isEmptyAtPath(values map[string]interface{}, path []string) bool {
	cur := values
	for i, key := range path {
		v, ok := cur[key]
		if !ok {
			return true
		}
		if i == len(path)-1 {
			str, isStr := v.(string)
			return isStr && str == ""
		}
		next, isMap := v.(map[string]interface{})
		if !isMap {
			return false
		}
		cur = next
	}

	return true
}

// setAtPath sets value at path in values, creating intermediate maps as needed.
func setAtPath(values map[string]interface{}, path []string, value string) {
	cur := values
	for i, key := range path {
		if i == len(path)-1 {
			cur[key] = value
			return
		}
		next, ok := cur[key].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			cur[key] = next
		}
		cur = next
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

// InjectRegistryValue sets the registry value in the static values
// TODO(ipaqsa): get rid of it after migration to module v2
func (s *Storage) InjectRegistryValue(registry registry.Remote) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// inject spec to values schema
	s.injectRegistrySpec(schema.TypeSettings)
	// inject spec to helm schema
	s.injectRegistrySpec(schema.TypeHelm)

	if s.staticValues == nil {
		s.staticValues = addonutils.Values{}
	}

	s.staticValues["registry"] = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"base":      {Kind: &structpb.Value_StringValue{StringValue: registry.Repository}},
			"dockercfg": {Kind: &structpb.Value_StringValue{StringValue: registry.DockerConfig}},
			"scheme":    {Kind: &structpb.Value_StringValue{StringValue: registry.Scheme}},
			"ca":        {Kind: &structpb.Value_StringValue{StringValue: registry.CA}},
		},
	}

	_ = s.calculateResultValues()
}

// injectRegistrySpec mutates the module schema to add a strict-typed "registry" field
func (s *Storage) injectRegistrySpec(schemaType schema.Type) {
	scheme := s.schemaStorage.GetSchema(schemaType)
	if scheme == nil {
		return
	}

	if len(scheme.Properties) == 0 {
		scheme.Properties = make(map[string]spec.Schema)
	}

	scheme.Properties["registry"] = spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:                 spec.StringOrArray{"object"},
			AdditionalProperties: &spec.SchemaOrBool{Allows: false},
			Properties: map[string]spec.Schema{
				"base": {
					SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"string"}, MinLength: conv.Pointer[int64](1)},
				},
				"dockercfg": {
					SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"string"}},
				},
				"scheme": {
					SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"string"}},
				},
				"ca": {
					SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"string"}},
				},
			},
			Required: []string{"base", "scheme"},
		},
	}
}
