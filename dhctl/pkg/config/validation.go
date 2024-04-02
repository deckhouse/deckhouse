// Copyright 2024 Flant JSC
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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/go-openapi/spec"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sYAML "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"
)

const (
	InitConfigurationKind    = "InitConfiguration"
	ClusterConfigurationKind = "ClusterConfiguration"
)

var cloudProviderToProviderKind = map[string]string{
	"OpenStack": "OpenStackClusterConfiguration",
	"AWS":       "AWSClusterConfiguration",
	"GCP":       "GCPClusterConfiguration",
	"Yandex":    "YandexClusterConfiguration",
	"vSphere":   "VsphereClusterConfiguration",
	"Azure":     "AzureClusterConfiguration",
	"VCD":       "VCDClusterConfiguration",
}

type ClusterConfig struct {
	ClusterType string `yaml:"clusterType"`
	Cloud       struct {
		Provider string `json:"provider"`
	} `yaml:"cloud"`
}

// ValidateResources parses and validates cluster ResourcesConfiguration/InitResourcesConfiguration.
// It requires all resources to have group, version and kind.
func ValidateResources(configData string, opts ...ValidateOption) error {
	options := applyOptions(opts...)
	if !options.commanderMode {
		panic("ValidateResources operation currently supported only in commander mode")
	}

	if k8sYAML.IsJSONBuffer([]byte(configData)) {
		return errors.New("got json format, but expected yaml")
	}

	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(configData), -1)
	errs := &ValidationError{}

	for i, doc := range docs {
		if doc == "" {
			continue
		}
		docData := []byte(doc)

		obj := &unstructured.Unstructured{}
		_, gvk, err := scheme.Codecs.UniversalDecoder().Decode(docData, nil, obj)
		if err != nil {
			errs.Append(ErrKindInvalidYAML, Error{
				Index:    pointer.Int(i),
				Messages: []string{fmt.Errorf("unmarshal: %w", err).Error()},
			})
			continue
		}

		var errMessages []string

		if gvk.Version == "" {
			errMessages = append(errMessages, ".apiVersion is required")
		}

		if gvk.Kind == "CustomResourceDefinition" {
			errMessages = append(errMessages, "got unacceptable resource kind: CustomResourceDefinition")
		}

		if len(errMessages) != 0 {
			errs.Append(ErrKindValidationFailed, Error{
				Index:    pointer.Int(i),
				Group:    gvk.Group,
				Version:  gvk.Version,
				Kind:     gvk.Kind,
				Name:     obj.GetName(),
				Messages: errMessages,
			})
		}
	}

	return errs.ErrorOrNil()
}

// ValidateInitConfiguration parses and validates cluster InitConfiguration.
// It requires one doc with InitConfiguration kind.
func ValidateInitConfiguration(configData string, schemaStore *SchemaStore, opts ...ValidateOption) error {
	options := applyOptions(opts...)
	if !options.commanderMode {
		panic("ValidateInitConfiguration operation currently supported only in commander mode")
	}

	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(configData), -1)
	errs := &ValidationError{}
	var initConfigDocsCount int

	for i, doc := range docs {
		if doc == "" {
			continue
		}

		docData := []byte(doc)

		obj := unstructured.Unstructured{}
		err := yaml.Unmarshal(docData, &obj)
		if err != nil {
			errs.Append(ErrKindInvalidYAML, Error{
				Index:    pointer.Int(i),
				Messages: []string{fmt.Errorf("unmarshal: %w", err).Error()},
			})
			continue
		}

		gvk := obj.GroupVersionKind()
		index := SchemaIndex{
			Kind:    gvk.Kind,
			Version: gvk.GroupVersion().String(),
		}

		var errMessages []string

		err = schemaStore.ValidateWithIndex(&index, &docData, opts...)
		if err != nil {
			errMessages = append(errMessages, err.Error())
		}

		switch index.Kind {
		case InitConfigurationKind:
			initConfigDocsCount++
		case ModuleConfigKind:
		default:
			errMessages = append(errMessages, fmt.Errorf(
				"unknown kind, expected one of (%q, %q)", InitConfigurationKind, ModuleConfigKind,
			).Error())
		}

		if len(errMessages) != 0 {
			errs.Append(ErrKindValidationFailed, Error{
				Index:    pointer.Int(i),
				Group:    gvk.Group,
				Version:  gvk.Version,
				Kind:     gvk.Kind,
				Name:     obj.GetName(),
				Messages: errMessages,
			})
		}
	}

	if initConfigDocsCount != 1 {
		errs.Append(ErrKindValidationFailed, Error{
			Messages: []string{fmt.Errorf("exactly one %q required", InitConfigurationKind).Error()},
		})
	}

	return errs.ErrorOrNil()
}

// ValidateClusterConfiguration parses and validates cluster ClusterConfiguration.
// It requires one doc with ClusterConfiguration kind.
// Returns data that needs to validate ProviderSpecificClusterConfiguration.
func ValidateClusterConfiguration(
	clusterConfigData string,
	schemaStore *SchemaStore,
	opts ...ValidateOption,
) (ClusterConfig, error) {
	options := applyOptions(opts...)
	if !options.commanderMode {
		panic("ValidateClusterConfiguration operation currently supported only in commander mode")
	}

	clusterConfigurationDocs := input.YAMLSplitRegexp.Split(strings.TrimSpace(clusterConfigData), -1)
	errs := &ValidationError{}
	var clusterConfigDocsCount int
	var clusterConfig ClusterConfig

	for i, doc := range clusterConfigurationDocs {
		if doc == "" {
			continue
		}

		docData := []byte(doc)

		obj := unstructured.Unstructured{}
		err := yaml.Unmarshal(docData, &obj)
		if err != nil {
			errs.Append(ErrKindInvalidYAML, Error{
				Index:    pointer.Int(i),
				Messages: []string{fmt.Errorf("unmarshal: %w", err).Error()},
			})
			continue
		}

		gvk := obj.GroupVersionKind()
		index := SchemaIndex{
			Kind:    gvk.Kind,
			Version: gvk.GroupVersion().String(),
		}

		var errMessages []string

		err = schemaStore.ValidateWithIndex(&index, &docData, opts...)
		if err != nil {
			errMessages = append(errMessages, err.Error())
		}

		switch index.Kind {
		case ClusterConfigurationKind:
			clusterConfigDocsCount++

			if err = yaml.Unmarshal([]byte(doc), &clusterConfig); err != nil {
				errMessages = append(errMessages, fmt.Errorf("unmarshal: %w", err).Error())
			}
		default:
			errMessages = append(errMessages, fmt.Errorf(
				"unknown kind, expected %q", ClusterConfigurationKind,
			).Error())
		}

		if len(errMessages) != 0 {
			errs.Append(ErrKindValidationFailed, Error{
				Index:    pointer.Int(i),
				Group:    gvk.Group,
				Version:  gvk.Version,
				Kind:     gvk.Kind,
				Name:     obj.GetName(),
				Messages: errMessages,
			})
		}
	}

	if clusterConfigDocsCount != 1 {
		errs.Append(ErrKindValidationFailed, Error{
			Messages: []string{fmt.Errorf("exactly one %q required", ClusterConfigurationKind).Error()},
		})
	}

	if err := errs.ErrorOrNil(); err != nil {
		return ClusterConfig{}, err
	}

	return clusterConfig, nil
}

// ValidateProviderSpecificClusterConfiguration parses and validates cluster ProviderSpecificClusterConfiguration.
// For cloud clusters it requires one doc with kind in
// [
// "OpenStackClusterConfiguration",
// "AWSClusterConfiguration",
// "GCPClusterConfiguration",
// "YandexClusterConfiguration",
// "VsphereClusterConfiguration",
// "AzureClusterConfiguration",
// ]
func ValidateProviderSpecificClusterConfiguration(
	providerSpecificClusterConfiguration string,
	clusterConfig ClusterConfig,
	schemaStore *SchemaStore,
	opts ...ValidateOption,
) error {
	options := applyOptions(opts...)
	if !options.commanderMode {
		panic("ValidateProviderSpecificClusterConfiguration operation currently supported only in commander mode")
	}

	if clusterConfig.ClusterType == "Static" {
		return nil
	}

	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(providerSpecificClusterConfiguration), -1)
	errs := &ValidationError{}
	var clusterConfigDocsCount int

	providerKind, ok := cloudProviderToProviderKind[clusterConfig.Cloud.Provider]
	if !ok {
		errs.Append(ErrKindValidationFailed, Error{
			Messages: []string{fmt.Sprintf(
				"unknown cloud provider '%s', check if 'ClusterConfiguration' is valid",
				clusterConfig.Cloud.Provider,
			)},
		})
	}

	for i, doc := range docs {
		if doc == "" {
			continue
		}

		docData := []byte(doc)

		obj := unstructured.Unstructured{}
		err := yaml.Unmarshal(docData, &obj)
		if err != nil {
			errs.Append(ErrKindInvalidYAML, Error{
				Index:    pointer.Int(i),
				Messages: []string{fmt.Errorf("unmarshal: %w", err).Error()},
			})
			continue
		}

		gvk := obj.GroupVersionKind()
		index := SchemaIndex{
			Kind:    gvk.Kind,
			Version: gvk.GroupVersion().String(),
		}

		var errMessages []string

		err = schemaStore.ValidateWithIndex(&index, &docData, opts...)
		if err != nil {
			errMessages = append(errMessages, err.Error())
		}

		switch index.Kind {
		case providerKind:
			clusterConfigDocsCount++
		default:
			errMessages = append(errMessages, fmt.Errorf("unknown kind, expected %q", providerKind).Error())
		}

		if len(errMessages) != 0 {
			errs.Append(ErrKindValidationFailed, Error{
				Index:    pointer.Int(i),
				Group:    gvk.Group,
				Version:  gvk.Version,
				Kind:     gvk.Kind,
				Name:     obj.GetName(),
				Messages: errMessages,
			})
		}
	}

	if clusterConfigDocsCount != 1 {
		errs.Append(ErrKindValidationFailed, Error{
			Messages: []string{fmt.Errorf("exactly one %q required", providerKind).Error()},
		})
	}

	return errs.ErrorOrNil()
}

// ValidateClusterSettingsChanges validates changes of current cluster configuration with the previous one.
// It checks the configuration changes for compliance with the current phase and schema extension rule (x-unsafe).
// It denies any changes for fields with `x-unsafe: true`.
// It applies all validation rules to fields with not empty `x-unsafe-rules` extension.
// On the BaseInfra phase changes are allowed.
// Non-config resources are checked only for compliance with the yaml format and the validity of apiVersion and kind fields: no changes validation for them.
// It can be used as an imported functionality in external modules.
func ValidateClusterSettingsChanges(
	phase phases.OperationPhase,
	oldConfig, newConfig string,
	schemaStore *SchemaStore,
	opts ...ValidateOption,
) error {
	options := applyOptions(opts...)
	if !options.commanderMode {
		panic("ValidateClusterSettingsChanges operation currently supported only in commander mode")
	}

	// todo: > bashible
	if phase == phases.BaseInfraPhase {
		return nil
	}

	oldRawDocs := input.YAMLSplitRegexp.Split(strings.TrimSpace(oldConfig), -1)
	newRawDocs := input.YAMLSplitRegexp.Split(strings.TrimSpace(newConfig), -1)

	oldDocs := map[namedIndex]string{}
	newDocs := map[namedIndex]string{}

	for _, rawDoc := range oldRawDocs {
		if rawDoc == "" {
			continue
		}
		err := setConfigs(schemaStore, oldDocs, rawDoc)
		if err != nil {
			return err
		}
	}

	for _, rawDoc := range newRawDocs {
		if rawDoc == "" {
			continue
		}
		err := setConfigs(schemaStore, newDocs, rawDoc)
		if err != nil {
			return err
		}
	}

	errs := &ValidationError{}

	for index, newDoc := range newDocs {
		oldDoc, ok := oldDocs[index]
		if !ok {
			continue
		}

		docSchema := schemaStore.getV1alpha1CompatibilitySchema(&SchemaIndex{
			Kind:    index.Kind,
			Version: index.Version,
		})
		if docSchema == nil {
			errs.Append(ErrKindChangesValidationFailed, Error{
				Messages: []string{"unknown yaml configuration index"},
			})
			continue
		}

		err := compareWith([]byte(oldDoc), []byte(newDoc), *docSchema)
		if err != nil {
			errs.Append(ErrKindChangesValidationFailed, Error{
				Messages: []string{err.Error()},
			})
			continue
		}
	}

	return errs.ErrorOrNil()
}

func setConfigs(
	schemaStore *SchemaStore,
	configs map[namedIndex]string,
	doc string,
) error {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return nil
	}

	docData := []byte(doc)

	index := namedIndex{}
	err := yaml.Unmarshal(docData, &index)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if !index.IsValid() {
		return fmt.Errorf(
			"document must contain \"kind\" and \"apiVersion\" fields:\n\tapiVersion: %s\n\tkind: %s",
			index.Version, index.Kind,
		)
	}

	docSchema := schemaStore.getV1alpha1CompatibilitySchema(&SchemaIndex{
		Kind:    index.Kind,
		Version: index.Version,
	})
	if docSchema == nil {
		// No need to compare Resources that are not stored in the cache.
		return nil
	}

	configs[index] = doc

	return nil
}

func compareWith(oldDoc, newDoc json.RawMessage, schema spec.Schema) error {
	if schema.Properties == nil {
		return nil
	}

	var oldProperties map[string]json.RawMessage
	var newProperties map[string]json.RawMessage

	err := yaml.Unmarshal(oldDoc, &oldProperties)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(newDoc, &newProperties)
	if err != nil {
		return err
	}

	err = validateXUnsafeExtensions(oldDoc, newDoc, schema)
	if err != nil {
		return err
	}

	for field, fieldSchema := range schema.Properties {
		err = validateXUnsafeExtensions(oldProperties[field], newProperties[field], fieldSchema)
		if err != nil {
			return fmt.Errorf("%q: %w", field, err)
		}

		err = compareWith(oldProperties[field], newProperties[field], fieldSchema)
		if err != nil {
			return fmt.Errorf("%q: %w", field, err)
		}
	}

	return nil
}

func validateXUnsafeExtensions(
	oldDoc, newDoc json.RawMessage,
	schema spec.Schema,
) error {
	isUnsafe, _ := schema.Extensions.GetBool(xUnsafeExtension)
	if isUnsafe && !bytes.Equal(oldDoc, newDoc) {
		return ErrUnsafeFieldChanged
	}

	if xRules, ok := schema.Extensions.GetStringSlice(xUnsafeRulesExtension); ok {
		for _, rule := range xRules {
			validator, ok := xUnsafeRulesValidators[rule]
			if !ok {
				continue
			}

			err := validator(oldDoc, newDoc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type ErrorKind int

const (
	ErrKindChangesValidationFailed ErrorKind = iota + 1
	ErrKindValidationFailed
	ErrKindInvalidYAML
)

func (k ErrorKind) String() string {
	switch k {
	case ErrKindChangesValidationFailed:
		return "ChangesValidationFailed"
	case ErrKindValidationFailed:
		return "ValidationFailed"
	case ErrKindInvalidYAML:
		return "InvalidYAML"
	default:
		return "unknown"
	}
}

type ValidationError struct {
	Kind   ErrorKind
	Errors []Error
}

func (v *ValidationError) Append(kind ErrorKind, e Error) {
	if v.Kind < kind {
		v.Kind = kind
	}
	v.Errors = append(v.Errors, e)
}

func (v *ValidationError) Error() string {
	if v == nil {
		return ""
	}
	errs := make([]string, 0, len(v.Errors))
	for _, e := range v.Errors {
		b := strings.Builder{}
		if e.Index != nil {
			b.WriteString(fmt.Sprintf("[%d]", *e.Index))
		}

		if e.Group != "" {
			b.WriteString(fmt.Sprintf(" %s", schema.GroupVersionKind{
				Group:   e.Group,
				Version: e.Version,
				Kind:    e.Kind,
			}.String()))
		}
		if e.Name != "" {
			b.WriteString(fmt.Sprintf(" %q", e.Name))
		}
		if b.Len() != 0 {
			b.WriteString(": ")
		}
		b.WriteString(strings.Join(e.Messages, "; "))

		errs = append(errs, b.String())
	}

	return fmt.Sprintf("%s: %s", v.Kind, strings.Join(errs, "\n"))
}

func (v *ValidationError) ErrorOrNil() error {
	if v == nil {
		return nil
	}
	if len(v.Errors) == 0 {
		return nil
	}

	return v
}

type Error struct {
	Index    *int
	Group    string
	Version  string
	Kind     string
	Name     string
	Messages []string
}

type namedIndex struct {
	Kind     string `json:"kind"`
	Version  string `json:"apiVersion"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

func (i *namedIndex) IsValid() bool {
	return i.Kind != "" && i.Version != ""
}

func (i *namedIndex) String() string {
	if i.Metadata.Name != "" {
		return fmt.Sprintf("%s, %s", i.Kind, i.Version)
	}
	return fmt.Sprintf("%s, %s, metadata.name: %q", i.Kind, i.Version, i.Metadata.Name)
}
