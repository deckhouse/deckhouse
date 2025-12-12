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
	"net"
	"strings"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type SSHCredential struct {
	User         string
	Key          string
	SudoPassword string
	Port         string
}

func (pc *Checker) CheckStaticInstancesSSH(ctx context.Context) error {
	config := input.YAMLSplitRegexp.Split(pc.metaConfig.ResourcesYAML, -1)

	for _, doc := range config {

		var res map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &res); err != nil {
			return fmt.Errorf("cannot unmarshal YAML: %v", err)
		}

		if res["kind"] != "StaticInstance" {
			continue
		}

		meta := res["metadata"].(map[string]interface{})
		instanceName := meta["name"].(string)

		spec := res["spec"].(map[string]interface{})
		address := spec["address"].(string)

		credRef := spec["credentialsRef"].(map[string]interface{})
		credName := credRef["name"].(string)

		cred, err := findSSHCredentials(config, credName)
		if err != nil {
			return fmt.Errorf("instance %s: %v", instanceName, err)
		}

		if err := testSSHConnection(address, cred.Port, cred); err != nil {
			return fmt.Errorf("cannot connect to %s (%s:%s): %v", instanceName, address, cred.Port, err)
		}
	}

	return nil
}

func findSSHCredentials(docs []string, name string) (*SSHCredential, error) {
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

		var key, sudoPassword string

		if k, ok := spec["privateSSHKey"].(string); ok && k != "" {
			keyBytes, err := base64.StdEncoding.DecodeString(k)
			if err != nil {
				return nil, fmt.Errorf("SSHCredentials %s: cannot decode privateSSHKey from Base64: %v", name, err)
			}
			key = string(keyBytes)
		}

		if sp, ok := spec["sudoPasswordEncoded"].(string); ok && sp != "" {
			decoded, err := base64.StdEncoding.DecodeString(sp)
			if err != nil {
				return nil, fmt.Errorf("SSHCredentials %s: cannot decode sudoPasswordEncoded: %v", name, err)
			}
			sudoPassword = string(decoded)
		}

		if key == "" && sudoPassword == "" {
			return nil, fmt.Errorf("SSHCredentials %s: must contain privateSSHKey or sudoPasswordEncoded", name)
		}

		port := "22"
		if p, ok := spec["sshPort"]; ok {
			port = fmt.Sprintf("%d", int(p.(int64)))
		}

		return &SSHCredential{
			User:         user,
			Key:          key,
			SudoPassword: sudoPassword,
			Port:         port,
		}, nil
	}

	return nil, fmt.Errorf("SSHCredentials %s not found", name)
}

func testSSHConnection(address, port string, cred *SSHCredential) error {
	var auth ssh.AuthMethod

	if cred.Key != "" {
		signer, err := ssh.ParsePrivateKey([]byte(cred.Key))
		if err != nil {
			return fmt.Errorf("invalid private key: %v", err)
		}
		auth = ssh.PublicKeys(signer)
	} else {
		auth = ssh.Password(cred.SudoPassword)
	}

	config := &ssh.ClientConfig{
		User:            cred.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", net.JoinHostPort(address, port), config)
	if err != nil {
		return err
	}
	defer conn.Close()

	if cred.SudoPassword != "" {
		session, err := conn.NewSession()
		if err != nil {
			return fmt.Errorf("cannot create SSH session: %v", err)
		}
		defer session.Close()

		session.Stdin = strings.NewReader(cred.SudoPassword + "\n")
		if err := session.Run("sudo -S -l"); err != nil {
			return fmt.Errorf("sudo password invalid or sudo not allowed: %v", err)
		}
	}

	return nil
}
