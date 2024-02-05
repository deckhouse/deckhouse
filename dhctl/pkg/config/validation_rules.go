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

	"sigs.k8s.io/yaml"
)

const (
	xUnsafeRuleUpdateReplicas = "updateReplicas"
	xUnsafeRuleDeleteZones    = "deleteZones"
)

type ValidationRule func(oldValue, newValue json.RawMessage) error

var validators = map[string]ValidationRule{
	xUnsafeRuleUpdateReplicas: UpdateReplicasRule,
	xUnsafeRuleDeleteZones:    DeleteZonesRule,
}

func UpdateReplicasRule(oldRaw, newRaw json.RawMessage) error {
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

	if newValue == 0 {
		return fmt.Errorf("%w: got unacceptable replicas zero value", ErrValidationRuleFailed)
	}

	if newValue < oldValue && newValue < 2 {
		return fmt.Errorf("%w: the new replicas value (%d) cannot be less that than 2 (%d)", ErrValidationRuleFailed, newValue, oldValue)
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
