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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-openapi/spec"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

const xUnsafeExtension = "x-unsafe"

// ValidateClusterSettingsFormat parses and validates cluster configuration and resources.
// It checks the cluster configuration yamls for compliance with the yaml format and schema.
// Non-config resources are checked only for compliance with the yaml format and the validity of apiVersion and kind fields.
// It can be used as an imported functionality in external modules.
func ValidateClusterSettingsFormat(settings string) error {
	schemaStore := NewSchemaStore()

	bigFileTmp := strings.TrimSpace(settings)
	docs := regexp.MustCompile(`(?:^|\s*\n)---\s*`).Split(bigFileTmp, -1)

	metaConfig := MetaConfig{}
	for _, doc := range docs {
		err := parseDocument(doc, &metaConfig, schemaStore)
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

type RuleValidationOption struct {
	path string
	rule ValidationRule
}

func NewRuleValidationOption(path string, rule ValidationRule) RuleValidationOption {
	return RuleValidationOption{
		path: path,
		rule: rule,
	}
}

// ValidateClusterSettingsChanges validates changes of current cluster configuration with the previous one.
// It checks the configuration changes for compliance with the current phase and schema extension rule (x-unsafe).
// It denies any changes for fields with `x-unsafe: true` for non-BaseInfra phases. On the BaseInfra phase changes are allowed.
// Non-config resources are checked only for compliance with the yaml format and the validity of apiVersion and kind fields: no changes validation for them.
// It can be used as an imported functionality in external modules.
func ValidateClusterSettingsChanges(phase phases.OperationPhase, oldSettings, newSettings string, options ...RuleValidationOption) error {
	if phase == phases.BaseInfraPhase {
		return nil
	}

	schemaStore := NewSchemaStore()

	oldRawDocs := regexp.MustCompile(`(?:^|\s*\n)---\s*`).Split(strings.TrimSpace(oldSettings), -1)
	newRawDocs := regexp.MustCompile(`(?:^|\s*\n)---\s*`).Split(strings.TrimSpace(newSettings), -1)

	oldDocs := map[SchemaIndex]string{}
	newDocs := map[SchemaIndex]string{}

	for _, rawDoc := range oldRawDocs {
		err := setConfigs(schemaStore, oldDocs, rawDoc)
		if err != nil {
			return err
		}
	}

	for _, rawDoc := range newRawDocs {
		err := setConfigs(schemaStore, newDocs, rawDoc)
		if err != nil {
			return err
		}
	}

	if len(oldDocs) != len(newDocs) {
		return ErrConfigAmountChanged
	}

	ruleValidators := NewDefaultRuleValidators()

	for index, newDoc := range newDocs {
		oldDoc, ok := oldDocs[index]
		if !ok {
			return errors.New("cannot to add additional configuration file")
		}

		schema := schemaStore.getV1alpha1CompatibilitySchema(&index)
		if schema == nil {
			return errors.New("unknown yaml configuration index")
		}

		ruleValidator := ruleValidators[index]
		for _, option := range options {
			ruleValidator.CreateRule(option.path, option.rule)
		}

		err := compareWith([]byte(oldDoc), []byte(newDoc), *schema, &ruleValidator)
		if err != nil {
			return err
		}
	}

	return nil
}

func setConfigs(schemaStore *SchemaStore, configs map[SchemaIndex]string, doc string) error {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return nil
	}

	docData := []byte(doc)

	index, err := schemaStore.Validate(&docData)
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

func compareWith(oldDoc, newDoc json.RawMessage, schema spec.Schema, ruleValidator *RuleValidator) error {
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

	for field, fieldSchema := range schema.Properties {
		isUnsafe, _ := fieldSchema.Extensions.GetBool(xUnsafeExtension)
		if isUnsafe {
			if bytes.Equal(oldProperties[field], newProperties[field]) {
				continue
			}

			return fmt.Errorf("%s: %w", field, ErrUnsafeFieldChanged)
		}

		if ruleValidator != nil && ruleValidator.rules != nil && ruleValidator.rules[field] != nil {
			err = ruleValidator.rules[field](oldProperties[field], newProperties[field])
			if err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
		}

		var fieldValidator *RuleValidator
		if ruleValidator != nil && ruleValidator.validators != nil {
			fieldValidator = ruleValidator.validators[field]
		}

		err = compareWith(oldProperties[field], newProperties[field], fieldSchema, fieldValidator)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
	}

	return nil
}
