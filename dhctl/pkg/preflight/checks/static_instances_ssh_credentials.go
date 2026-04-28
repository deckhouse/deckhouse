// Copyright 2026 Flant JSC
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

package checks

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/lib-connection/pkg/ssh"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	sdk "github.com/deckhouse/module-sdk/pkg/utils"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha2"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type staticInstance struct {
	Name     string
	Address  string
	CredName string
}

type StaticInstancesSSHCredentialsCheck struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	MetaConfig             *config.MetaConfig
}

const StaticInstancesSSHCredentialsCheckName preflight.CheckName = "static-instances-ssh-credentials"

func (StaticInstancesSSHCredentialsCheck) Description() string {
	return "SSHCredentials for StaticInstances are correct."
}

func (StaticInstancesSSHCredentialsCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (StaticInstancesSSHCredentialsCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.DefaultRetryPolicy
}

func (c StaticInstancesSSHCredentialsCheck) Run(ctx context.Context) error {
	docs := input.YAMLSplitRegexp.Split(c.MetaConfig.ResourcesYAML, -1)
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
		if err := testSSHConnection(ctx, c.SSHProviderInitializer, inst.Address, cred); err != nil {
			return fmt.Errorf("Cannot connect to %s (%s:%d): %w", inst.Name, inst.Address, cred.SSHPort, err)
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

func testSSHConnection(ctx context.Context, sshProviderInitializer *providerinitializer.SSHProviderInitializer, address string, cred *v1alpha2.SSHCredentialsSpec) error {
	nodeInterface, err := helper.GetNodeInterface(ctx, sshProviderInitializer, sshProviderInitializer.GetSettings())
	if err != nil {
		return err
	}
	_, remote := nodeInterface.(*ssh.NodeInterfaceWrapper)

	sshProvider, err := sshProviderInitializer.GetSSHProvider(ctx)
	if err != nil {
		return err
	}

	var sess *session.Session
	pkeys := make([]session.AgentPrivateKey, 0)

	if remote {
		sshClient, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}
		sess = sshClient.Session()
		pkeys = sshClient.PrivateKeys()
	}

	becomePass, err := base64.StdEncoding.DecodeString(cred.SudoPasswordEncoded)
	if err != nil {
		return err
	}

	config := session.NewSession(session.Input{
		User:       cred.User,
		Port:       strconv.Itoa(cred.SSHPort),
		BecomePass: string(becomePass),
	})
	config.AddAvailableHosts(session.Host{Host: address})

	if remote {
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
	}

	tmpDir := filepath.Join(os.Getenv("TMPDIR"), "preflight")

	err = os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create tmp directory: %w", err)
	}

	privateKeyPrefixPathWithoutSuffix := filepath.Join(tmpDir, "id_rsa_preflight.key")

	n := rand.New(rand.NewSource(time.Now().UnixNano())).Int()
	privateKeyPath := fmt.Sprintf("%s.%d", privateKeyPrefixPathWithoutSuffix, n)

	err = os.WriteFile(privateKeyPath, []byte(cred.PrivateSSHKey), 0o600)
	if err != nil {
		return fmt.Errorf("Failed to write private key: %w", err)
	}

	pkeys = append(pkeys, session.AgentPrivateKey{Key: privateKeyPath})
	client, err := sshProvider.NewStandaloneClient(ctx, config, pkeys)
	if err != nil {
		return fmt.Errorf("Cannot create SSH client: %w", err)
	}
	defer client.Stop()

	if err := client.Start(); err != nil {
		return fmt.Errorf("Cannot connect to SSH host %s: %w", address, err)
	}
	defer client.Stop()

	cmd := client.Command("true")
	cmd.Sudo(ctx)

	if err := cmd.Run(ctx); err != nil {
		return fmt.Errorf("Sudo check failed: %w\nstderr: %s", err, string(cmd.StderrBytes()))
	}
	return nil
}

func StaticInstancesSSHCredentials(metaConfig *config.MetaConfig, sshProviderInitializer *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := StaticInstancesSSHCredentialsCheck{
		SSHProviderInitializer: sshProviderInitializer,
		MetaConfig:             metaConfig,
	}
	return preflight.Check{
		Name:        StaticInstancesSSHCredentialsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
