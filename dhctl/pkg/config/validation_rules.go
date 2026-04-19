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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	ssh "github.com/deckhouse/lib-gossh"
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
	xRulesSSHPublicKey  = "sshPublicKey"
)

var xUnsafeRulesValidators = map[string]func(oldValue, newValue json.RawMessage) error{
	xUnsafeRuleUpdateReplicas:    UpdateReplicasRule,
	xUnsafeRuleDeleteZones:       DeleteZonesRule,
	xUnsafeRuleUpdateMasterImage: UpdateMasterImageRule,
}

var xRulesValidators = map[string]func(oldValue json.RawMessage) error{
	xRulesSSHPrivateKey: ValidateSSHPrivateKey,
	xRulesSSHPublicKey:  ValidateSSHPublicKey,
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
			ImageName string `yaml:"imageName"` // OpenStackClusterConfiguration,HuaweiCloudClusterConfiguration,DynamixClusterConfiguration
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

	// Image update is now allowed for both single-master and multi-master clusters.
	// dhctl converge handles image updates correctly by automatically scaling to 3 replicas
	// and back to 1 for single-master clusters if needed.
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

func ValidateSSHPublicKey(value json.RawMessage) error {
	var key string
	err := yaml.Unmarshal(value, &key)
	if err != nil {
		return err
	}
	data := []byte(key)

	// only error matters, this is only point of nil error return
	//nolint: dogsled
	_, _, _, _, err = ssh.ParseAuthorizedKey(data)
	if err == nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var base64Str string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "----") || strings.Contains(line, ":") || line == "" {
			continue
		}
		base64Str += line
	}
	keyBytes, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return fmt.Errorf("%w: failed to decode base64 string: %w", ErrValidationRuleFailed, err)
	}
	_, err = ssh.ParsePublicKey(keyBytes)
	if err != nil {
		return fmt.Errorf("%w: failed to parse public key: %w", ErrValidationRuleFailed, err)
	}

	return fmt.Errorf("%w: wrong public key format: please, convert it to openssh format using command like ssh-keygen -i -f key.ssh2 > key.pub", ErrValidationRuleFailed)
}
