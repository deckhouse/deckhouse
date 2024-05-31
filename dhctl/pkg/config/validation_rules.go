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

	"golang.org/x/crypto/ssh"
	"sigs.k8s.io/yaml"
)

const (
	xUnsafeExtension      = "x-unsafe"
	xUnsafeRulesExtension = "x-unsafe-rules"
	xRulesExtension       = "x-rules"
)

const (
	xUnsafeRuleUpdateReplicas    = "updateReplicas"
	xUnsafeRuleDeleteZones       = "deleteZones"
	xUnsafeRuleUpdateMasterImage = "updateMasterImage"

	xRulesSSHPrivateKey = "sshPrivateKey"
)

var xUnsafeRulesValidators = map[string]func(oldValue, newValue json.RawMessage) error{
	xUnsafeRuleUpdateReplicas:    UpdateReplicasRule,
	xUnsafeRuleDeleteZones:       DeleteZonesRule,
	xUnsafeRuleUpdateMasterImage: UpdateMasterImageRule,
}

var xRulesValidators = map[string]func(oldValue json.RawMessage) error{
	xRulesSSHPrivateKey: ValidateSSHPrivateKey,
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
		return fmt.Errorf("%w: got unacceptable .masterNodeGroup.replicas zero value", ErrValidationRuleFailed)
	}

	if newValue < oldValue && newValue == 1 {
		return fmt.Errorf(
			"%w: can't reduce the number of master nodegroup replicas to 1, functionality will be available in future versions",
			ErrValidationRuleFailed,
		)
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

	var oldConfig clusterConfig
	var newConfig clusterConfig

	err := yaml.Unmarshal(oldRaw, &oldConfig)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(newRaw, &newConfig)
	if err != nil {
		return err
	}

	if len(newConfig.Zones) >= len(oldConfig.Zones) {
		return nil
	}

	if newConfig.MasterNodeGroup.Replicas >= 3 {
		return nil
	}

	return fmt.Errorf(
		"%w: can't delete zone if .masterNodeGroup.replicas < 3 (%d)",
		ErrValidationRuleFailed,
		newConfig.MasterNodeGroup.Replicas,
	)
}

func UpdateMasterImageRule(oldRaw, newRaw json.RawMessage) error {
	type clusterConfig struct {
		Replicas      int `yaml:"replicas"`
		InstanceClass struct {
			AMI       string `yaml:"ami"`       // AWSClusterConfiguration
			URN       string `yaml:"urn"`       // AzureClusterConfiguration
			Image     string `yaml:"image"`     // GCPClusterConfiguration
			ImageID   string `yaml:"imageID"`   // YandexClusterConfiguration
			ImageName string `yaml:"imageName"` // OpenStackClusterConfiguration
			Template  string `yaml:"template"`  // VsphereClusterConfiguration,VCDClusterConfiguration,ZvirtClusterConfiguration
		} `yaml:"instanceClass"`
	}

	var oldConfig clusterConfig
	var newConfig clusterConfig

	err := yaml.Unmarshal(oldRaw, &oldConfig)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(newRaw, &newConfig)
	if err != nil {
		return err
	}

	for _, images := range []struct {
		old   string
		new   string
		field string
	}{
		{old: oldConfig.InstanceClass.AMI, new: newConfig.InstanceClass.AMI, field: "ami"},
		{old: oldConfig.InstanceClass.URN, new: newConfig.InstanceClass.URN, field: "urn"},
		{old: oldConfig.InstanceClass.Image, new: newConfig.InstanceClass.Image, field: "image"},
		{old: oldConfig.InstanceClass.ImageID, new: newConfig.InstanceClass.ImageID, field: "imageID"},
		{old: oldConfig.InstanceClass.ImageName, new: newConfig.InstanceClass.ImageName, field: "imageName"},
		{old: oldConfig.InstanceClass.Template, new: newConfig.InstanceClass.Template, field: "template"},
	} {
		if images.new != "" && images.old != images.new {
			if oldConfig.Replicas > 1 && newConfig.Replicas > 1 {
				return fmt.Errorf(
					"%w: can't update .masterNodeGroup.%s in multi-master cluster, functionality will be available in future versions",
					ErrValidationRuleFailed, images.field,
				)
			}

			return fmt.Errorf(
				"%w: can't update .masterNodeGroup.%s in single-master cluster, functionality will be available in future versions",
				ErrValidationRuleFailed, images.field,
			)
		}
	}

	return nil
}

func ValidateSSHPrivateKey(value json.RawMessage) error {
	type keyConfig struct {
		Key        string `yaml:"key"`
		Passphrase string `yaml:"passphrase"`
	}

	var key keyConfig

	err := yaml.Unmarshal(value, &key)
	if err != nil {
		return err
	}

	switch key.Passphrase {
	case "":
		_, err = ssh.ParseRawPrivateKey([]byte(key.Key))
		if err != nil {
			return fmt.Errorf("%w: invalid ssh key: %w", ErrValidationRuleFailed, err)
		}
	default:
		_, err = ssh.ParseRawPrivateKeyWithPassphrase([]byte(key.Key), []byte(key.Passphrase))
		if err != nil {
			return fmt.Errorf("%w: invalid ssh key: %w", ErrValidationRuleFailed, err)
		}
	}

	return nil
}
