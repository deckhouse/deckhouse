// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdk "github.com/deckhouse/module-sdk/pkg/utils"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha2"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type staticInstance struct {
	Name     string
	Address  string
	CredName string
}

func (pc *Checker) CheckStaticInstancesSSH(ctx context.Context) error {
	if app.PreflightSkipStaticInstancesWithSSHCredentials {
		log.InfoLn("SSHCredentials for StaticInstances preflight check was skipped (via skip flag)")
		return nil
	}

	docs := input.YAMLSplitRegexp.Split(pc.metaConfig.ResourcesYAML, -1)
	instances, creds, err := parseResources(docs)
	if err != nil {
		return err
	}

	for _, inst := range instances {
		cred, ok := creds[inst.CredName]
		if !ok {
			return fmt.Errorf("Instance %s: SSHCredentials %s not found", inst.Name, inst.CredName)
		}
		log.InfoF("Checking StaticInstance %s (%s)\n", inst.Name, inst.Address)
		if err := testSSHConnection(ctx, pc.nodeInterface, inst.Address, cred); err != nil {
			return fmt.Errorf(
				"Cannot connect to %s (%s:%d): %w",
				inst.Name,
				inst.Address,
				cred.SSHPort,
				err,
			)
		}
	}
	return nil
}

func parseResources(docs []string) ([]staticInstance, map[string]*v1alpha2.SSHCredentialsSpec, error) {
	var instances []staticInstance
	creds := make(map[string]*v1alpha2.SSHCredentialsSpec)

	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		var m map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
			return nil, nil, fmt.Errorf("Cannot unmarshal YAML: %w", err)
		}

		res := unstructured.Unstructured{Object: m}

		kind := res.GetKind()
		switch kind {
		case "StaticInstance":
			var si v1alpha2.StaticInstance
			if err := sdk.FromUnstructured(&res, &si); err != nil {
				return nil, nil, fmt.Errorf("StaticInstance: cannot convert from unstructured: %w", err)
			}

			name := si.GetName()
			address := strings.TrimSpace(si.Spec.Address)
			credName := strings.TrimSpace(si.Spec.CredentialsRef.Name)

			if name == "" {
				return nil, nil, fmt.Errorf("StaticInstance: metadata.name is empty")
			}
			if address == "" {
				return nil, nil, fmt.Errorf("StaticInstance %s: spec.address is empty", name)
			}
			if credName == "" {
				return nil, nil, fmt.Errorf("StaticInstance %s: spec.credentialsRef.name is empty", name)
			}

			instances = append(instances, staticInstance{
				Name:     name,
				Address:  address,
				CredName: credName,
			})

		case "SSHCredentials":
			var sc v1alpha2.SSHCredentials
			if err := sdk.FromUnstructured(&res, &sc); err != nil {
				return nil, nil, fmt.Errorf("SSHCredentials: cannot convert from unstructured: %w", err)
			}

			name := sc.GetName()
			cred, err := parseSSHCredentials(&sc)
			if err != nil {
				return nil, nil, fmt.Errorf("SSHCredentials %s: %w", name, err)
			}

			creds[name] = cred
		default:
			continue
		}
	}

	return instances, creds, nil
}

func parseSSHCredentials(sc *v1alpha2.SSHCredentials) (*v1alpha2.SSHCredentialsSpec, error) {
	name := sc.GetName()
	if name == "" {
		return nil, fmt.Errorf("SSHCredentials: metadata.name is empty")
	}

	user := strings.TrimSpace(sc.Spec.User)
	if user == "" {
		return nil, fmt.Errorf("User must be specified and not empty")
	}

	var privateKey string
	var sudoPassword string

	if k := strings.TrimSpace(sc.Spec.PrivateSSHKey); k != "" {
		keyBytes, err := base64.StdEncoding.DecodeString(k)
		if err != nil {
			return nil, fmt.Errorf("Cannot decode privateSSHKey: %w", err)
		}
		privateKey = string(keyBytes)
	}

	if sp := strings.TrimSpace(sc.Spec.SudoPasswordEncoded); sp != "" {
		passBytes, err := base64.StdEncoding.DecodeString(sp)
		if err != nil {
			return nil, fmt.Errorf("Cannot decode sudoPasswordEncoded: %w", err)
		}
		sudoPassword = string(passBytes)
	}

	if privateKey == "" && sudoPassword == "" {
		return nil, fmt.Errorf("Must contain privateSSHKey or sudoPasswordEncoded")
	}

	port := sc.Spec.SSHPort
	if port == 0 {
		port = 22
	}

	return &v1alpha2.SSHCredentialsSpec{
		User:                user,
		PrivateSSHKey:       privateKey,
		SudoPasswordEncoded: sudoPassword,
		SSHPort:             port,
	}, nil
}
func testSSHConnection(ctx context.Context, nodeInterface node.Interface, address string, cred *v1alpha2.SSHCredentialsSpec) error {
	sshClient := nodeInterface.(*ssh.NodeInterfaceWrapper).Client()
	sess := sshClient.Session()

	config := sshclient.ClientConfig{
		User:                cred.User,
		SSHPort:             cred.SSHPort,
		PrivateSSHKey:       cred.PrivateSSHKey,
		SudoPasswordEncoded: cred.SudoPasswordEncoded,
		BastionKeys:         sshClient.PrivateKeys(),
	}

	if sess.BastionHost != "" {
		config.BastionHost = sess.BastionHost
		config.BastionPort = sess.BastionPort
		config.BastionUser = sess.BastionUser
		config.BastionPassword = sess.BastionPassword
	} else {
		config.BastionHost = sess.AvailableHosts()[0].Host
		config.BastionPort = sess.Port
		config.BastionUser = sess.User
		config.BastionPassword = sess.BecomePass
	}

	client, err := sshclient.NewClientFromConfig(ctx, address, config)
	if err != nil {
		return fmt.Errorf("Cannot create SSH client: %w", err)
	}

	if err := client.Start(); err != nil {
		return fmt.Errorf("Cannot connect to SSH host %s: %w", address, err)
	}
	defer client.Stop()

	cmd := client.Command("true")
	cmd.Sudo(ctx)

	if err := cmd.Run(ctx); err != nil {
		return fmt.Errorf(
			"Sudo check failed: %w\nstderr: %s",
			err,
			string(cmd.StderrBytes()),
		)
	}
	return nil
}
