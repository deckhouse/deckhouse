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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	transformer "github.com/deckhouse/deckhouse/dhctl/pkg/config/schema"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// SchemaStore is safe for concurrent use: provider schemas can be loaded with
// LoadProviderDir while other goroutines validate. mu guards cache,
// moduleConfigsCache, modulesCache, providerDigests and providerIndexes.
type SchemaStore struct {
	mu                 sync.RWMutex
	cache              map[SchemaIndex]*spec.Schema
	moduleConfigsCache map[string]*spec.Schema
	modulesCache       map[string]struct{}
	conversionsStore   *conversion.ConversionsStore
	providerDigests    map[string]string
	providerIndexes    map[string][]SchemaIndex
}

var (
	once  sync.Once
	store *SchemaStore
)

type validateOptions struct {
	omitDocInError       bool
	commanderMode        bool
	strictUnmarshal      bool
	validateExtensions   bool
	requiredSSHHost      bool
	collectAllErrors     bool
	skipSchemaValidation bool
	operation            string
	downloadRootDir      string
}

type ValidateOption func(o *validateOptions)

// ValidateOptionOmitDocInError configures whether to exclude the original document
// from validation error messages. By default, the document is included.
// When this option is enabled, the document will be omitted from errors.
func ValidateOptionOmitDocInError(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.omitDocInError = v
	}
}

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

// ValidateOptionCollectAllErrors makes ParseConfigFromData accumulate per-doc
// errors into a *ValidationError instead of returning on the first one. Off
// by default — bootstrap CLI keeps its fail-fast semantics. Validators that
// want multi-error UX enable this.
func ValidateOptionCollectAllErrors(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.collectAllErrors = v
	}
}

// ValidateOptionSkipSchemaValidation makes parseDocument skip schemaStore
// OpenAPI checks and just categorize a document by its kind. Use it for
// "intent extraction" passes — domain analyzers (e.g. CNI mismatch) that
// must read the user's cluster intent without re-running schema checks the
// schema-validator pass has already done.
func ValidateOptionSkipSchemaValidation(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.skipSchemaValidation = v
	}
}

func ValidateOptionOperation(op string) ValidateOption {
	return func(o *validateOptions) {
		o.operation = op
	}
}

func ValidateOptionDownloadRootDir(dir string) ValidateOption {
	return func(o *validateOptions) {
		o.downloadRootDir = dir
	}
}

func NewSchemaStore(globalOptions *options.GlobalOptions, paths ...string) *SchemaStore {
	// fallback to default value
	candiDir := options.DefaultCandiDir
	if globalOptions != nil && globalOptions.CandiDir != "" {
		candiDir = globalOptions.CandiDir
	}
	paths = append([]string{candiDir}, paths...)

	// External provider images unpack into <DownloadDir>/<provider>@<digest>/
	// with a <DownloadDir>/<provider> symlink pointing at the current digest.
	// Scan only the real digest dirs: symlinks are skipped (their targets are
	// listed directly), as are the bundled "deckhouse" tree and the image cache.
	if globalOptions != nil && globalOptions.DownloadDir != "" {
		entries, err := os.ReadDir(globalOptions.DownloadDir)
		if err != nil && !os.IsNotExist(err) {
			log.WarnF("read download dir %s: %v\n", globalOptions.DownloadDir, err)
		}
		for _, e := range entries {
			if !e.IsDir() || e.Type()&os.ModeSymlink != 0 {
				continue
			}
			if e.Name() == "deckhouse" || e.Name() == "cache" {
				continue
			}
			paths = append(paths, filepath.Join(globalOptions.DownloadDir, e.Name()))
		}
	}

	pathsStr := strings.TrimSpace(os.Getenv("DHCTL_CLI_ADDITIONAL_SCHEMAS_PATHS"))
	if pathsStr != "" {
		pathsNoTrimmed := strings.Split(pathsStr, ",")
		for _, p := range pathsNoTrimmed {
			paths = append(paths, strings.TrimSpace(p))
		}
	}

	return newOnceSchemaStore(globalOptions, paths)
}

func newOnceSchemaStore(globalOptions *options.GlobalOptions, schemasDir []string) *SchemaStore {
	once.Do(func() {
		store = newSchemaStore(globalOptions, schemasDir)
	})
	return store
}

func newSchemaStore(globalOptions *options.GlobalOptions, schemasDir []string) *SchemaStore {
	st := &SchemaStore{
		cache:              make(map[SchemaIndex]*spec.Schema),
		moduleConfigsCache: make(map[string]*spec.Schema),
		modulesCache:       make(map[string]struct{}),
		providerDigests:    make(map[string]string),
		providerIndexes:    make(map[string][]SchemaIndex),
	}

	st.conversionsStore = conversion.NewConversionsStore()

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}

		if _, ok := schemaFileNames[info.Name()]; ok {
			if uploadError := st.UploadByPath(path); uploadError != nil {
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

	// fallback to default
	modulesDir := options.DefaultModulesDir
	globalHookModule := options.DefaultGlobalHooksModule
	if globalOptions != nil && globalOptions.ModulesDir != "" {
		modulesDir = globalOptions.ModulesDir
		globalHookModule = globalOptions.GlobalHooksModule
	}

	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		// autoconverger and state exporter do not contains module dir
		log.WarnF("Modules dir %s not found\n", modulesDir)
		return st
	}

	loadConversions := func(path, moduleName string) error {
		conversionPath := filepath.Join(filepath.Dir(path), "conversions")
		stat, err := os.Stat(conversionPath)
		if err == nil && stat.IsDir() {
			err := st.conversionsStore.Add(moduleName, conversionPath)
			log.DebugF("Found conversion for module %s. Latest version: %d\n", moduleName, st.conversionsStore.Get(moduleName).LatestVersion())

			return err
		}
		return nil
	}

	loadConfigValuesSchema := func(path, moduleName string) error {
		content, err := os.ReadFile(path)
		var schema *spec.Schema

		switch {
		case err == nil:
			schema = new(spec.Schema)
			if err := yaml.Unmarshal(content, schema); err != nil {
				return err
			}

			if err := spec.ExpandSchema(schema, schema, nil); err != nil {
				return err
			}

			schema = transformer.TransformSchema(
				schema,
				&transformer.AdditionalPropertiesTransformer{},
			)

			if err := loadConversions(path, moduleName); err != nil {
				return err
			}

			st.moduleConfigsCache[moduleName] = schema

		case errors.Is(err, os.ErrNotExist):
			log.DebugF("Openapi spec not found for module %s\n", moduleName)
		default:
			return err
		}

		return nil
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		moduleName := strings.TrimLeft(name, "0123456789-")
		st.modulesCache[moduleName] = struct{}{}
		p := path.Join(modulesDir, name, "openapi", "config-values.yaml")
		if err := loadConfigValuesSchema(p, moduleName); err != nil {
			// We don't expect panic here our logger does not support log.Fatal
			panic(err)
		}
		candiOpenAPIDir := filepath.Join(modulesDir, name, "candi", "openapi")
		if _, err := os.Stat(candiOpenAPIDir); err == nil {
			if err := filepath.Walk(candiOpenAPIDir, walkFunc); err != nil {
				panic(err)
			}
		}
	}

	err = loadConfigValuesSchema(path.Join(globalHookModule, "openapi", "config-values.yaml"), "global")
	if err != nil {
		// We don't expect panic here our logger does not support log.Fatal
		panic(err)
	}

	return st
}

func (s *SchemaStore) Get(index *SchemaIndex) *spec.Schema {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[*index]
}

func (s *SchemaStore) HasSchemaForModuleConfig(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.moduleConfigsCache[name]
	return ok
}

func (s *SchemaStore) GetModuleConfigSchema(name string) (*spec.Schema, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.moduleConfigsCache[name]
	if !ok {
		return nil, fmt.Errorf("schema for %s not found", name)
	}

	return res, nil
}

func (s *SchemaStore) GetModuleConfigVersion(name string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
		return nil, fmt.Errorf("Schema index unmarshal failed: %w", err)
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
		log.DebugF("Found module config to validate %s\n", mcName)
		if mc.Spec.Enabled == nil && mcName != "global" {
			// we need return error because on top level we want filter module configs from modulesources and move into resources
			// global is special mc without module
			return fmt.Errorf("Enabled field for module config %s should be set to true or false", mcName)
		}

		s.mu.RLock()
		_, moduleKnown := s.modulesCache[mcName]
		s.mu.RUnlock()
		if !moduleKnown && mcName != "global" {
			log.DebugF("Module %s wasn't found. It is probably a module from modulesources. Skipping it\n", mc.GetName())
			return ErrSchemaNotFound
		}

		if len(mc.Spec.Settings) == 0 {
			return nil
		}

		s.mu.RLock()
		var ok bool
		schema, ok = s.moduleConfigsCache[mcName]
		s.mu.RUnlock()
		if !ok {
			log.DebugF("Schema for module config %s wasn't found. It is probably a module from modulesources. Skipping it\n", mc.GetName())
			return fmt.Errorf("Schema for module config %s not found", mcName)
		}

		if mc.Spec.Version == 0 {
			return fmt.Errorf("Version field for module config %s should be set", mcName)
		}

		var err error
		docForValidate, err = s.applyConversions(mc)
		if err != nil {
			return fmt.Errorf("Setting up validation for module config failed: %v", err)
		}
	} else {
		schema = s.getV1alpha1CompatibilitySchema(index)
	}

	if schema == nil {
		log.DebugF("No schema for index %s. Skipping it\n", index.String())
		// we need return error because on top level we want filter documents without index and move into resources
		return ErrSchemaNotFound
	}

	schema = transformer.TransformSchema(schema, &transformer.AdditionalPropertiesTransformer{})

	isValid, err := openAPIValidate(&docForValidate, schema, options)
	if !isValid {
		if options.omitDocInError || options.commanderMode {
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
	parsed, err := parseOpenAPISchemas(fileContent)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for index, schema := range parsed {
		s.cache[index] = schema
	}

	return nil
}

func parseOpenAPISchemas(fileContent []byte) (map[SchemaIndex]*spec.Schema, error) {
	openAPISchema := new(OpenAPISchema)
	if err := yaml.UnmarshalStrict(fileContent, openAPISchema); err != nil {
		return nil, fmt.Errorf("json unmarshal: %v", err)
	}

	result := make(map[SchemaIndex]*spec.Schema, len(openAPISchema.Versions))
	for _, parsedSchema := range openAPISchema.Versions {
		schema := new(spec.Schema)

		d, err := json.Marshal(parsedSchema.Schema)
		if err != nil {
			return nil, fmt.Errorf("expand the schema: %v", err)
		}

		if err := json.Unmarshal(d, schema); err != nil {
			return nil, fmt.Errorf("json marshal: %v", err)
		}

		if err := spec.ExpandSchema(schema, schema, nil); err != nil {
			return nil, fmt.Errorf("expand the schema: %v", err)
		}

		schema = transformer.TransformSchema(
			schema,
			&transformer.AdditionalPropertiesTransformer{},
		)

		result[SchemaIndex{Kind: openAPISchema.Kind, Version: parsedSchema.Version}] = schema
	}

	return result, nil
}

// schemaFileNames lists the OpenAPI schema files dhctl loads from candi trees
// and unpacked provider bundles.
var schemaFileNames = map[string]struct{}{
	"init_configuration.yaml":            {},
	"cluster_configuration.yaml":         {},
	"static_cluster_configuration.yaml":  {},
	"cloud_discovery_data.yaml":          {},
	"cloud_provider_discovery_data.yaml": {},
	"ssh_configuration.yaml":             {},
	"ssh_host_configuration.yaml":        {},
}

// ProviderSchemasLoaded reports whether LoadProviderDir already loaded this
// provider at this digest.
func (s *SchemaStore) ProviderSchemasLoaded(provider, digest string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return digest != "" && s.providerDigests[provider] == digest
}

// HasProviderSchemas reports whether any schemas for the provider were loaded
// via LoadProviderDir into this store, regardless of digest.
func (s *SchemaStore) HasProviderSchemas(provider string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.providerIndexes[provider]) > 0
}

// LoadProviderDir loads provider schemas from dir (an unpacked bundle root)
// into the store, replacing schemas previously loaded for this provider. A
// repeated call with the same digest is a no-op.
func (s *SchemaStore) LoadProviderDir(provider, digest, dir string) error {
	if s.ProviderSchemasLoaded(provider, digest) {
		return nil
	}

	parsed := make(map[SchemaIndex]*spec.Schema)
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return err
		}
		if _, ok := schemaFileNames[info.Name()]; !ok {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read schema file: %w", err)
		}
		schemas, err := parseOpenAPISchemas(content)
		if err != nil {
			return fmt.Errorf("parse schema file %s: %w", path, err)
		}
		for index, schema := range schemas {
			parsed[index] = schema
		}
		return nil
	}
	if err := filepath.Walk(dir, walkFunc); err != nil {
		return fmt.Errorf("walk provider schemas dir %s: %w", dir, err)
	}

	indexes := make([]SchemaIndex, 0, len(parsed))
	for index := range parsed {
		indexes = append(indexes, index)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if digest != "" && s.providerDigests[provider] == digest {
		return nil
	}
	for _, index := range s.providerIndexes[provider] {
		delete(s.cache, index)
	}
	for index, schema := range parsed {
		s.cache[index] = schema
	}
	s.providerIndexes[provider] = indexes
	s.providerDigests[provider] = digest
	return nil
}

func openAPIValidate(dataObj *[]byte, schema *spec.Schema, options validateOptions) (bool, error) {
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

func ValidateDiscoveryData(config *[]byte, paths []string, globalOptions *options.GlobalOptions, opts ...ValidateOption) (bool, error) {
	schemaStore := NewSchemaStore(globalOptions, paths...)

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

func (s *SchemaStore) applyConversions(mc ModuleConfig) ([]byte, error) {
	conversion := s.conversionsStore.Get(mc.GetName())
	log.DebugF("Starting conversion for module %s. Latest version: %d\n", mc.GetName(), conversion.LatestVersion())
	var err error
	var conversed map[string]interface{}
	if mc.Spec.Version < conversion.LatestVersion() {
		set := &mc.Spec.Settings
		unstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(set)
		if err != nil {
			return []byte{}, fmt.Errorf("error converting to unstructured: %w", err)
		}
		_, conversed, err = conversion.ConvertToLatest(mc.Spec.Version, unstructured)
		if err != nil {
			return []byte{}, fmt.Errorf("error converting to unstructured: %w", err)
		}
		log.DebugF("conversion successfully applied for ModuleConfig %s\n", mc.GetName())
	} else {
		return yaml.Marshal(mc.Spec.Settings)
	}

	doc, err := yaml.Marshal(conversed)
	return doc, err
}
