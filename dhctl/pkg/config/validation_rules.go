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
	"fmt"

	"sigs.k8s.io/yaml"
)

const (
	xUnsafeRuleNotLessThanPrevious = "notLessThanPrevious"
	xUnsafeRuleDeleteZones         = "deleteZones"
)

type ValidationRule func(oldValue, newValue json.RawMessage) error

var validators = map[string]ValidationRule{
	xUnsafeRuleNotLessThanPrevious: NotLessRuleThanPrevious,
	xUnsafeRuleDeleteZones:         DeleteZonesRule,
}

func NotLessRuleThanPrevious(oldRaw, newRaw json.RawMessage) error {
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

func DeleteZonesRule(oldRaw, newRaw json.RawMessage) error {
	type clusterConfig struct {
		Zones           []string `yaml:"zones"`
		MasterNodeGroup struct {
			Replicas int `yaml:"replicas"`
		} `yaml:"masterNodeGroup"`
	}

	var oldClusterConfig clusterConfig
	var newClusterConfig clusterConfig

	err := yaml.Unmarshal(oldRaw, &oldClusterConfig)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(newRaw, &newClusterConfig)
	if err != nil {
		return err
	}

	if len(newClusterConfig.Zones) >= len(oldClusterConfig.Zones) {
		return nil
	}

	if newClusterConfig.MasterNodeGroup.Replicas >= 3 {
		return nil
	}

	return fmt.Errorf(
		"%w: can't delete zone if masterNodeGroup.Replicas < 3 (%d)",
		ErrValidationRuleFailed,
		newClusterConfig.MasterNodeGroup.Replicas,
	)
}
