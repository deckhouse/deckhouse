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

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1alpha2"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func (pc *Checker) CheckStaticInstancesSSH(ctx context.Context) error {
	docs := input.YAMLSplitRegexp.Split(pc.metaConfig.ResourcesYAML, -1)

	for _, doc := range docs {
		var res map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &res); err != nil {
			fmt.Printf("[DEBUG] cannot unmarshal YAML: %v\n", err)
			return fmt.Errorf("cannot unmarshal YAML: %w", err)
		}

		if res["kind"] != "StaticInstance" {
			continue
		}

		meta := res["metadata"].(map[string]interface{})
		spec := res["spec"].(map[string]interface{})

		instanceName := meta["name"].(string)
		address := spec["address"].(string)

		fmt.Printf("[DEBUG] Checking StaticInstance '%s' at %s\n", instanceName, address)

		credRef := spec["credentialsRef"].(map[string]interface{})
		credName := credRef["name"].(string)

		cred, err := findSSHCredentials(docs, credName)
		if err != nil {
			fmt.Printf("[DEBUG] Cannot find SSH credentials '%s' for instance '%s': %v\n", credName, instanceName, err)
			return fmt.Errorf("instance %s: %w", instanceName, err)
		}

		fmt.Printf("[DEBUG] Using SSH credentials '%s' (user: %s, port: %d)\n", credName, cred.User, cred.SSHPort)

		if err := testSSHConnection(ctx, address, cred); err != nil {
			fmt.Printf("[DEBUG] SSH test failed for '%s': %v\n", instanceName, err)
			return fmt.Errorf(
				"cannot connect to %s (%s:%d): %w",
				instanceName,
				address,
				cred.SSHPort,
				err,
			)
		}

		fmt.Printf("[DEBUG] SSH test passed for '%s'\n", instanceName)
	}

	return nil
}

func findSSHCredentials(docs []string, name string) (*v1alpha2.SSHCredentialsSpec, error) {
	for _, doc := range docs {
		var res map[string]interface{}
		if yaml.Unmarshal([]byte(doc), &res) != nil {
			continue
		}

		if res["kind"] != "SSHCredentials" {
			continue
		}

		meta := res["metadata"].(map[string]interface{})
		if meta["name"] != name {
			continue
		}

		spec := res["spec"].(map[string]interface{})
		user := spec["user"].(string)

		var privateKey string
		var sudoPassword string

		if k, ok := spec["privateSSHKey"].(string); ok && k != "" {
			keyBytes, err := base64.StdEncoding.DecodeString(k)
			if err != nil {
				return nil, fmt.Errorf(
					"SSHCredentials %s: cannot decode privateSSHKey: %w",
					name,
					err,
				)
			}
			privateKey = string(keyBytes)
		}

		if sp, ok := spec["sudoPasswordEncoded"].(string); ok && sp != "" {
			passBytes, err := base64.StdEncoding.DecodeString(sp)
			if err != nil {
				return nil, fmt.Errorf(
					"SSHCredentials %s: cannot decode sudoPasswordEncoded: %w",
					name,
					err,
				)
			}
			sudoPassword = string(passBytes)
		}

		if privateKey == "" && sudoPassword == "" {
			return nil, fmt.Errorf(
				"SSHCredentials %s: must contain privateSSHKey or sudoPasswordEncoded",
				name,
			)
		}

		port := 22
		if p, ok := spec["sshPort"]; ok {
			port = int(p.(int64))
		}

		return &v1alpha2.SSHCredentialsSpec{
			User:                user,
			PrivateSSHKey:       privateKey,
			SudoPasswordEncoded: sudoPassword,
			SSHPort:             port,
		}, nil
	}

	return nil, fmt.Errorf("SSHCredentials %s not found", name)
}

func testSSHConnection(ctx context.Context, address string, cred *v1alpha2.SSHCredentialsSpec) error {
	sshClient, err := sshclient.NewClientFromConfig(ctx, address, cred)
	if err != nil {
		return fmt.Errorf("cannot create SSH client: %w", err)
	}

	if sshClient == nil {
		return fmt.Errorf("SSH client is nil")
	}

	if err := sshClient.Start(); err != nil {
		return fmt.Errorf("cannot connect to SSH host %s: %w", address, err)
	}
	defer sshClient.Stop()

	cmd := sshClient.Command("true")
	cmd.Sudo(ctx)

	if err := cmd.Run(ctx); err != nil {
		return fmt.Errorf(
			"sudo check failed on host %s: %w\nstderr: %s",
			address,
			err,
			string(cmd.StderrBytes()),
		)
	}

	return nil
}
