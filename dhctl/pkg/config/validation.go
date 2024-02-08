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

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/go-openapi/spec"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// ValidateClusterSettingsFormat parses and validates cluster configuration and resources.
// It checks the cluster configuration yamls for compliance with the yaml format and schema.
// Non-config resources are checked only for compliance with the yaml format and the validity of apiVersion and kind fields.
// It can be used as an imported functionality in external modules.
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
	oldSettings, newSettings string,
	opts ...ValidateOption,
) error {
	options := applyOptions(opts...)
	if !options.commanderMode {
		panic("ValidateClusterSettingsChanges operation currently supported only in commander mode")
	}

	if phase == phases.BaseInfraPhase {
		return nil
	}

	schemaStore := NewSchemaStore()

	oldRawDocs := input.YAMLSplitRegexp.Split(strings.TrimSpace(oldSettings), -1)
	newRawDocs := input.YAMLSplitRegexp.Split(strings.TrimSpace(newSettings), -1)

	oldDocs := map[SchemaIndex]string{}
	newDocs := map[SchemaIndex]string{}

	for _, rawDoc := range oldRawDocs {
		err := setConfigs(schemaStore, oldDocs, rawDoc, opts...)
		if err != nil {
			return err
		}
	}

	for _, rawDoc := range newRawDocs {
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
			return fmt.Errorf("%s: %w", field, err)
		}

		err = compareWith(oldProperties[field], newProperties[field], fieldSchema)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
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
