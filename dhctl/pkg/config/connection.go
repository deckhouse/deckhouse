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
	"os"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
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
	SSHExtraArgs        string               `json:"sshExtraArgs,omitempty"`
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

type ConnectionConfig struct {
	SSHConfig *SSHConfig
	SSHHosts  []SSHHost
}

func ParseConnectionConfig(
	configData string,
	schemaStore *SchemaStore,
	opts ...ValidateOption,
) (*ConnectionConfig, error) {
	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(configData), -1)
	errs := &ValidationError{}
	var connectionConfigDocsCount int

	config := &ConnectionConfig{}

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
		case SSHConfigKind:
			connectionConfigDocsCount++
			var sshConfig *SSHConfig
			if err = yaml.Unmarshal([]byte(doc), &sshConfig); err != nil {
				errMessages = append(errMessages, fmt.Errorf("unmarshal: %w", err).Error())
			}
			config.SSHConfig = sshConfig
		case SSHConfigHostKind:
			var sshHost SSHHost
			if err = yaml.Unmarshal([]byte(doc), &sshHost); err != nil {
				errMessages = append(errMessages, fmt.Errorf("unmarshal: %w", err).Error())
			}
			config.SSHHosts = append(config.SSHHosts, sshHost)
		default:
			errMessages = append(errMessages, fmt.Errorf(
				"unknown kind, expected one of (%q, %q)", SSHConfigKind, SSHConfigHostKind,
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

	if connectionConfigDocsCount != 1 {
		errs.Append(ErrKindValidationFailed, Error{
			Messages: []string{fmt.Errorf("exactly one %q required", SSHConfigKind).Error()},
		})
	}

	if err := errs.ErrorOrNil(); err != nil {
		return nil, err
	}

	return config, nil
}

type ConnectionConfigParser struct{}

// ParseConnectionConfigFromFile parses SSH connection config from file (app.ConnectionConfigPath)
// and fills app.SSH* variables with corresponding data.
func (ConnectionConfigParser) ParseConnectionConfigFromFile() error {
	cfg, err := parseConnectionConfigFromFile(app.ConnectionConfigPath)
	if err != nil {
		return fmt.Errorf("parsing ssh config from file: %w", err)
	}

	keys := make([]string, 0, len(cfg.SSHConfig.SSHAgentPrivateKeys))
	for _, key := range cfg.SSHConfig.SSHAgentPrivateKeys {
		keys = append(keys, key.Key)
	}

	hosts := make([]string, 0, len(cfg.SSHHosts))
	for _, host := range cfg.SSHHosts {
		hosts = append(hosts, host.Host)
	}

	bastionPort := ""
	if cfg.SSHConfig.SSHBastionPort != nil {
		bastionPort = strconv.Itoa(int(*cfg.SSHConfig.SSHBastionPort))
	}

	port := ""
	if cfg.SSHConfig.SSHPort != nil {
		port = strconv.Itoa(int(*cfg.SSHConfig.SSHPort))
	}

	app.SSHPrivateKeys = keys
	app.SSHBastionHost = cfg.SSHConfig.SSHBastionHost
	app.SSHBastionPort = bastionPort
	app.SSHBastionUser = cfg.SSHConfig.SSHBastionUser
	app.SSHUser = cfg.SSHConfig.SSHUser
	app.SSHHosts = hosts
	app.SSHPort = port
	app.SSHExtraArgs = cfg.SSHConfig.SSHExtraArgs

	return nil
}

func parseConnectionConfigFromFile(path string) (*ConnectionConfig, error) {
	configData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading connection config file: %v", err)
	}

	return ParseConnectionConfig(string(configData), NewSchemaStore(), ValidateOptionValidateExtensions(true))
}
