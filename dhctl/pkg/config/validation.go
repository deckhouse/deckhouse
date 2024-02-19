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
	k8sYAML "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
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

	for _, doc := range docs {
		if doc == "" {
			continue
		}
		docData := []byte(doc)

		_, gvk, err := scheme.Codecs.UniversalDecoder().Decode(docData, nil, &unstructured.Unstructured{})
		if err != nil {
			return err
		}

		if gvk.Version == "" {
			return errors.New("no version information, but it's required")
		}

		if gvk.Kind == "CustomResourceDefinition" {
			return errors.New("got unacceptable resource kind: CustomResourceDefinition")
		}
	}

	return nil
}

// ValidateInitConfiguration parses and validates cluster InitConfiguration.
// It requires at one doc with InitConfiguration kind.
func ValidateInitConfiguration(configData string, schemaStore *SchemaStore, opts ...ValidateOption) error {
	options := applyOptions(opts...)
	if !options.commanderMode {
		panic("ValidateInitConfiguration operation currently supported only in commander mode")
	}

	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(configData), -1)
	var initConfigDocsCount int

	for _, doc := range docs {
		if doc == "" {
			continue
		}

		docData := []byte(doc)

		var index SchemaIndex
		err := yaml.Unmarshal(docData, &index)
		if err != nil {
			return fmt.Errorf("unmarshal init configuration: %w", err)
		}

		switch index.Kind {
		case InitConfigurationKind:
			initConfigDocsCount++
			if initConfigDocsCount > 1 {
				return fmt.Errorf("only one %q expected", InitConfigurationKind)
			}
			err = schemaStore.ValidateWithIndex(&index, &docData, opts...)
			if err != nil {
				return err
			}
		case ModuleConfigKind:
			err = schemaStore.ValidateWithIndex(&index, &docData, opts...)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown kind %q, expected %q or %q", index.Kind, InitConfigurationKind, ModuleConfigKind)
		}
	}

	if initConfigDocsCount == 0 {
		return fmt.Errorf("%q required", InitConfigurationKind)
	}

	return nil
}

// ValidateClusterConfiguration parses and validates cluster ClusterConfiguration.
// It requires at one doc with ClusterConfiguration kind.
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
	var clusterConfig *ClusterConfig

	for _, doc := range clusterConfigurationDocs {
		if doc == "" {
			continue
		}

		docData := []byte(doc)

		var index SchemaIndex
		err := yaml.Unmarshal(docData, &index)
		if err != nil {
			return ClusterConfig{}, fmt.Errorf("unmarshal cluster configuration: %w", err)
		}

		switch index.Kind {
		case ClusterConfigurationKind:
			if clusterConfig != nil {
				return ClusterConfig{}, fmt.Errorf("only one %q expected", ClusterConfigurationKind)
			}

			err = schemaStore.ValidateWithIndex(&index, &docData, opts...)
			if err != nil {
				return ClusterConfig{}, err
			}

			if err = yaml.Unmarshal([]byte(doc), &clusterConfig); err != nil {
				return ClusterConfig{}, fmt.Errorf("unable to unmarshal %q: %w\n---\n%s\n", ClusterConfigurationKind, err, doc)
			}
		default:
			return ClusterConfig{}, fmt.Errorf("unknown kind %q, expected %q", index.Kind, InitConfigurationKind)
		}
	}

	if clusterConfig == nil {
		return ClusterConfig{}, fmt.Errorf("%q required", ClusterConfigurationKind)
	}

	return *clusterConfig, nil
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

	providerKind, ok := cloudProviderToProviderKind[clusterConfig.Cloud.Provider]
	if !ok {
		return fmt.Errorf("unknown cloud provider %q", clusterConfig.Cloud.Provider)
	}

	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(providerSpecificClusterConfiguration), -1)
	var clusterConfigDocsCount int

	for _, doc := range docs {
		if doc == "" {
			continue
		}

		docData := []byte(doc)

		var index SchemaIndex
		err := yaml.Unmarshal(docData, &index)
		if err != nil {
			return fmt.Errorf("unmarshal init configuration: %w", err)
		}

		switch index.Kind {
		case providerKind:
			clusterConfigDocsCount++
			if clusterConfigDocsCount > 1 {
				return fmt.Errorf("only one %q expected", providerKind)
			}
			err = schemaStore.ValidateWithIndex(&index, &docData, opts...)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown kind %q, expected %q", index.Kind, providerKind)
		}
	}

	if clusterConfigDocsCount == 0 {
		return fmt.Errorf("%q required", providerKind)
	}

	return nil
}

// ValidateClusterSettingsFormat parses and validates cluster configuration and resources.
// It checks the cluster configuration yamls for compliance with the yaml format and schema.
// Non-config resources are checked only for compliance with the yaml format and the validity of apiVersion and kind fields.
// It can be used as an imported functionality in external modules.
// Deprecated! Use ValidateClusterConfiguration.
func ValidateClusterSettingsFormat(settings string, opts ...ValidateOption) error {
	options := applyOptions(opts...)
	if !options.commanderMode {
		panic("ValidateClusterSettingsFormat operation currently supported only in commander mode")
	}

	schemaStore := NewSchemaStore()

	bigFileTmp := strings.TrimSpace(settings)
	docs := input.YAMLSplitRegexp.Split(bigFileTmp, -1)

	metaConfig := MetaConfig{}
	for _, doc := range docs {
		if doc == "" {
			continue
		}

		err := parseDocument(doc, &metaConfig, schemaStore, opts...)
		// Cluster resources are not stored in the dhctl cache, there is no need to check them for compliance with the schema: just check the index and yaml format.
		if err != nil && !errors.Is(err, ErrSchemaNotFound) {
			return err
		}
	}

	_, err := metaConfig.Prepare()
	if err != nil {
		return err
	}

	return nil
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

	oldDocs := map[SchemaIndex]string{}
	newDocs := map[SchemaIndex]string{}

	for _, rawDoc := range oldRawDocs {
		if rawDoc == "" {
			continue
		}
		err := setConfigs(schemaStore, oldDocs, rawDoc, opts...)
		if err != nil {
			return err
		}
	}

	for _, rawDoc := range newRawDocs {
		if rawDoc == "" {
			continue
		}
		err := setConfigs(schemaStore, newDocs, rawDoc, opts...)
		if err != nil {
			return err
		}
	}

	if len(oldDocs) != len(newDocs) {
		return ErrConfigAmountChanged
	}

	for index, newDoc := range newDocs {
		oldDoc, ok := oldDocs[index]
		if !ok {
			return errors.New("cannot to add additional configuration file")
		}

		schema := schemaStore.getV1alpha1CompatibilitySchema(&index)
		if schema == nil {
			return errors.New("unknown yaml configuration index")
		}

		err := compareWith([]byte(oldDoc), []byte(newDoc), *schema)
		if err != nil {
			return err
		}
	}

	return nil
}

func setConfigs(
	schemaStore *SchemaStore,
	configs map[SchemaIndex]string,
	doc string,
	opts ...ValidateOption,
) error {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return nil
	}

	docData := []byte(doc)

	index, err := schemaStore.Validate(&docData, opts...)
	if err != nil && !errors.Is(err, ErrSchemaNotFound) {
		return err
	}

	if !index.IsValid() {
		return fmt.Errorf(
			"document must contain \"kind\" and \"apiVersion\" fields:\n\tapiVersion: %s\n\tkind: %s\n\n%s",
			index.Version, index.Kind, doc,
		)
	}

	schema := schemaStore.getV1alpha1CompatibilitySchema(index)
	if schema == nil {
		// No need to compare Resources that are not stored in the cache.
		return nil
	}

	configs[*index] = doc

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
