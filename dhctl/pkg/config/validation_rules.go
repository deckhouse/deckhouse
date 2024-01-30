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
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

type ValidationRule func(oldValue, newValue json.RawMessage) error

type RuleValidator struct {
	rules      map[string]ValidationRule
	validators map[string]*RuleValidator
}

func (v *RuleValidator) CreateRule(path string, rule ValidationRule) {
	separatorIndex := strings.IndexByte(path, '.')
	if separatorIndex == -1 {
		if v.rules == nil {
			v.rules = make(map[string]ValidationRule)
		}
		v.rules[path] = rule
		return
	}

	if v.validators == nil {
		v.validators = make(map[string]*RuleValidator)
	}

	if v.validators[path[:separatorIndex]] == nil {
		v.validators[path[:separatorIndex]] = &RuleValidator{}
	}

	v.validators[path[:separatorIndex]].CreateRule(path[separatorIndex+1:], rule)
}

func NewDefaultRuleValidators() map[SchemaIndex]RuleValidator {
	var v RuleValidator
	v.CreateRule("masterNodeGroup.replicas", NotLessRule)
	v.CreateRule("nodeGroups.replicas", NotLessRule)

	return map[SchemaIndex]RuleValidator{
		SchemaIndex{
			Kind:    "YandexClusterConfiguration",
			Version: "deckhouse.io/v1",
		}: v,
	}
}

func NotLessRule(oldRaw, newRaw json.RawMessage) error {
	var oldValue int
	var newValue int

	err := yaml.Unmarshal(oldRaw, &oldValue)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(newRaw, &newValue)
	if err != nil {
		return err
	}

	if newValue < oldValue {
		return fmt.Errorf("%w: the new value (%d) cannot be less that than previous one (%d)", ErrValidationRuleFailed, newValue, oldValue)
	}

	if newValue == 0 {
		return fmt.Errorf("%w: got unacceptable zero value", ErrValidationRuleFailed)
	}

	return nil
}
