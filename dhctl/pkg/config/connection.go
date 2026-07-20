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
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const (
	SSHConfigKind     = "SSHConfig"
	SSHConfigHostKind = "SSHHost"
)

type SSHConfig struct {
	SSHUser             string               `json:"sshUser"`
	SSHPort             *int32               `json:"sshPort,omitempty"`
	SSHAgentPrivateKeys []SSHAgentPrivateKey `json:"sshAgentPrivateKeys,omitempty"`
	SSHExtraArgs        string               `json:"sshExtraArgs,omitempty"`
	SSHBastionHost      string               `json:"sshBastionHost,omitempty"`
	SSHBastionPort      *int32               `json:"sshBastionPort,omitempty"`
	SSHBastionUser      string               `json:"sshBastionUser,omitempty"`
	SSHBastionPassword  string               `json:"sshBastionPassword,omitempty"`
	SudoPassword        string               `json:"sudoPassword,omitempty"`
	LegacyMode          bool                 `json:"legacyMode,omitempty"`
	ModernMode          bool                 `json:"modernMode,omitempty"`
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
	ctx context.Context,
	configData string,
	schemaStore *SchemaStore,
	opts ...ValidateOption,
) (*ConnectionConfig, error) {
	options := applyOptions(opts...)
	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(configData), -1)
	var connectionConfigDocsCount int
	var sshHostConfigDocsCount int

	errs := &ValidationError{}

	appendValidationError := func(msg string, docNumber int, gvk *schema.GroupVersionKind, obj *unstructured.Unstructured) {
		err := Error{
			Index:    new(docNumber),
			Messages: []string{msg},
		}

		if gvk != nil {
			err.Group = gvk.Group
			err.Version = gvk.Version
			err.Kind = gvk.Kind
		}

		if obj != nil {
			err.Name = obj.GetName()
		}

		errs.Append(ErrKindValidationFailed, err)
	}

	unmarshallError := func(err error, kind string, docNumber int) string {
		return fmt.Sprintf("Cannot unmarshal %s document %d: %v", kind, docNumber, err)
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Connection config has %d documents", len(docs)))

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
				Index:    new(i),
				Messages: []string{unmarshallError(err, "Unstructured", i)},
			})
			continue
		}

		gvk := obj.GroupVersionKind()
		index := SchemaIndex{
			Kind:    gvk.Kind,
			Version: gvk.GroupVersion().String(),
		}

		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Validating and parsing connection config document %d for index %v", i, index))

		err = schemaStore.ValidateWithIndex(&index, &docData, opts...)
		if err != nil {
			appendValidationError(err.Error(), i, &gvk, &obj)
			continue
		}

		switch index.Kind {
		case SSHConfigKind:
			connectionConfigDocsCount++
			var sshConfig SSHConfig
			if err = yaml.Unmarshal([]byte(doc), &sshConfig); err != nil {
				appendValidationError(unmarshallError(err, SSHConfigKind, i), i, &gvk, &obj)
				continue
			}
			config.SSHConfig = &sshConfig
			dhlog.FromContext(ctx).DebugContext(ctx, "SSHConfig added to the result config")
		case SSHConfigHostKind:
			sshHostConfigDocsCount++
			var sshHost SSHHost
			if err = yaml.Unmarshal([]byte(doc), &sshHost); err != nil {
				appendValidationError(unmarshallError(err, SSHConfigHostKind, i), i, &gvk, &obj)
				continue
			}
			config.SSHHosts = append(config.SSHHosts, sshHost)
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("SSHHost added to the result config, hosts in result config %d", len(config.SSHHosts)))
		default:
			msg := fmt.Sprintf("Unknown kind, expected one of (%q, %q)", SSHConfigKind, SSHConfigHostKind)
			appendValidationError(msg, i, &gvk, &obj)
			continue
		}
	}

	if connectionConfigDocsCount != 1 {
		errs.Append(ErrKindValidationFailed, Error{
			Messages: []string{fmt.Errorf("exactly one %q required", SSHConfigKind).Error()},
		})
	}

	if options.requiredSSHHost && sshHostConfigDocsCount == 0 {
		errs.Append(ErrKindValidationFailed, Error{
			Messages: []string{fmt.Errorf("at least one %q required", SSHConfigHostKind).Error()},
		})
	}

	if err := errs.ErrorOrNil(); err != nil {
		return nil, err
	}

	return config, nil
}

// ConnectionConfigParser parses an SSH connection config file and writes the
// result back into the supplied options struct. The parser is invoked from
// kingpin Action when --connection-config is set.
type ConnectionConfigParser struct {
	opts *options.Options
}

// NewConnectionConfigParser builds a parser that writes into the supplied opts.
func NewConnectionConfigParser(opts *options.Options) *ConnectionConfigParser {
	return &ConnectionConfigParser{opts: opts}
}

// ParseConnectionConfigFromFile parses SSH connection config from
// p.opts.SSH.ConnectionConfigPath and fills the SSH/Become options with the result.
func (p *ConnectionConfigParser) ParseConnectionConfigFromFile() error {
	if p == nil || p.opts == nil {
		return fmt.Errorf("ConnectionConfigParser is not initialized")
	}

	connectionConfigPath := p.opts.SSH.ConnectionConfigPath

	ctx := context.Background()
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Connection config path: %s", connectionConfigPath))

	cfg, err := parseConnectionConfigFromFile(ctx, connectionConfigPath, &p.opts.Global)
	if err != nil {
		return fmt.Errorf("Parsing ssh config from file: %w", err)
	}

	pathToPassPhrase := make(options.PrivateKeyFileToPassphrase)

	keysPaths := make([]string, 0, len(cfg.SSHConfig.SSHAgentPrivateKeys))
	for _, key := range cfg.SSHConfig.SSHAgentPrivateKeys {
		f, err := os.CreateTemp(p.opts.Global.TmpDir, "ssh-key-*")
		if err != nil {
			return fmt.Errorf("unable to create temp file: %w", err)
		}

		if _, err := f.Write([]byte(strings.TrimSpace(key.Key) + "\n")); err != nil {
			return fmt.Errorf("unable to write temp file %s: %w", f.Name(), err)
		}

		fullPath := f.Name()

		keysPaths = append(keysPaths, fullPath)
		if len(key.Passphrase) > 0 {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Passphrase for key %s added to the map", fullPath))
			pathToPassPhrase[fullPath] = key.Passphrase
		}
	}

	hosts := make([]session.Host, 0, len(cfg.SSHHosts))
	for i, host := range cfg.SSHHosts {
		hosts = append(hosts, session.Host{Host: host.Host, Name: strconv.Itoa(i)})
	}

	bastionPort := "22"
	if cfg.SSHConfig.SSHBastionPort != nil {
		bastionPort = strconv.Itoa(int(*cfg.SSHConfig.SSHBastionPort))
	}

	port := "22"
	if cfg.SSHConfig.SSHPort != nil {
		port = strconv.Itoa(int(*cfg.SSHConfig.SSHPort))
	}

	p.opts.SSH.PrivateKeys = keysPaths
	p.opts.SSH.BastionHost = cfg.SSHConfig.SSHBastionHost
	p.opts.SSH.BastionPort = bastionPort
	p.opts.SSH.BastionUser = cfg.SSHConfig.SSHBastionUser
	p.opts.Become.BecomePass = cfg.SSHConfig.SudoPassword
	p.opts.SSH.User = cfg.SSHConfig.SSHUser
	p.opts.SSH.Hosts = hosts
	p.opts.SSH.Port = port
	p.opts.SSH.ExtraArgs = cfg.SSHConfig.SSHExtraArgs
	p.opts.SSH.BastionPass = cfg.SSHConfig.SSHBastionPassword
	p.opts.SSH.LegacyMode = cfg.SSHConfig.LegacyMode
	p.opts.SSH.ModernMode = cfg.SSHConfig.ModernMode
	// todo it is ugly solution
	p.opts.SSH.PrivateKeysToPassPhrasesFromConfig = pathToPassPhrase

	return nil
}

func parseConnectionConfigFromFile(ctx context.Context, path string, globalOptions *options.GlobalOptions) (*ConnectionConfig, error) {
	configData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading connection config file: %v", err)
	}

	return ParseConnectionConfig(
		ctx,
		string(configData),
		NewSchemaStore(globalOptions),
		ValidateOptionValidateExtensions(true),
	)
}
