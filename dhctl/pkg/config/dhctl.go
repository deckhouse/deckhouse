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
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"sigs.k8s.io/yaml"
)

const (
	SSHConfigKind     = "SSHConfig"
	SSHConfigHostKind = "SSHHost"
)

type SSHConfig struct {
	SSHUser             string               `json:"sshUser"`
	SSHPort             *int32               `json:"sshPort,omitempty"`
	SSHAgentPrivateKeys []SSHAgentPrivateKey `json:"sshAgentPrivateKeys"`
	SSHBastionHost      string               `json:"sshBastionHost,omitempty"`
	SSHBastionPort      *int32               `json:"sshBastionPort,omitempty"`
	SSHBastionUser      string               `json:"sshBastionUser,omitempty"`
}

type SSHAgentPrivateKey struct {
	Key        string `json:"key"`
	Passphrase string `json:"passphrase,omitempty"`
}

type SSHHost struct {
	Host string `json:"host"`
}

type DHCTLConfig struct {
	SSHConfig *SSHConfig
	SSHHosts  []SSHHost
}

type DHCTLConfigOptions struct {
	CommanderMode bool
}

func ParseDHCTLConfig(configData string, schemaStore *SchemaStore, opts DHCTLConfigOptions) (*DHCTLConfig, error) {
	if !opts.CommanderMode {
		panic("ParseSSHConfigFromData operation currently supported only in commander mode")
	}

	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(configData), -1)

	config := &DHCTLConfig{}

	for _, doc := range docs {
		docData := []byte(doc)

		var index SchemaIndex
		err := yaml.Unmarshal(docData, &index)
		if err != nil {
			return nil, err
		}

		err = schemaStore.ValidateWithIndex(&index, &docData)
		if err != nil {
			return nil, err
		}

		switch index.Kind {
		case SSHConfigKind:
			if config.SSHConfig != nil {
				return nil, fmt.Errorf("only one SSHConfig expected")
			}

			var sshConfig *SSHConfig
			if err = yaml.Unmarshal([]byte(doc), &sshConfig); err != nil {
				return nil, fmt.Errorf("unable to unmarshal SSHConfig: %w\n---\n%s\n", err, doc)
			}
			config.SSHConfig = sshConfig
		case SSHConfigHostKind:
			var sshHost SSHHost
			if err = yaml.Unmarshal([]byte(doc), &sshHost); err != nil {
				return nil, fmt.Errorf("unable to unmarshal SSHHost: %w\n---\n%s\n", err, doc)
			}
			config.SSHHosts = append(config.SSHHosts, sshHost)
		default:
			return nil, fmt.Errorf("unknown kind %q, expected SSHConfig or SSHHost", index.Kind)
		}
	}

	if config.SSHConfig == nil {
		return nil, fmt.Errorf("SSHConfig required")
	}

	return config, nil
}
