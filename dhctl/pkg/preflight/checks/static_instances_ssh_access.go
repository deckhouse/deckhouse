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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/lib-connection/pkg/ssh"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	sdk "github.com/deckhouse/module-sdk/pkg/utils"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha2"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
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

type StaticInstancesSSHAccessCheck struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	MetaConfig             *config.MetaConfig
}

const StaticInstancesSSHAccessCheckName preflight.CheckName = "static-instances-ssh-access"

func (StaticInstancesSSHAccessCheck) Description() string {
	return "ssh access to StaticInstances is configured correctly"
}

func (StaticInstancesSSHAccessCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (StaticInstancesSSHAccessCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c StaticInstancesSSHAccessCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil || strings.TrimSpace(c.MetaConfig.ResourcesYAML) == "" {
		return "", preflight.NotApplicable("the --config file declares no resources")
	}

	docs := input.YAMLSplitRegexp.Split(c.MetaConfig.ResourcesYAML, -1)
	instances, creds, err := parseResources(docs)
	if err != nil {
		return "", err
	}
	if len(instances) == 0 {
		return "", preflight.NotApplicable("the --config file declares no StaticInstance")
	}

	// Every instance, not the first that fails: these are separate machines, and an operator
	// setting up a static cluster would otherwise fix them one bootstrap run at a time.
	var unreachable []string
	for _, inst := range instances {
		cred, ok := creds[inst.CredName]
		if !ok {
			return "", preflight.Permanent(&preflight.Failure{
				Checked:  fmt.Sprintf("StaticInstance %q in the --config file", inst.Name),
				Observed: fmt.Sprintf("it refers to SSHCredentials %q, which the file does not contain", inst.CredName),
				Expected: "an SSHCredentials resource for every StaticInstance",
				Fix:      fmt.Sprintf("add the SSHCredentials %q document, or correct spec.credentialsRef.name", inst.CredName),
			})
		}

		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Checking StaticInstance %s (%s)", inst.Name, inst.Address))
		if err := checkSSHAccess(ctx, c.SSHProviderInitializer, inst.Address, cred); err != nil {
			unreachable = append(unreachable, fmt.Sprintf("%s (%s@%s:%d): %s",
				inst.Name, cred.User, inst.Address, cred.SSHPort, oneLineError(err)))
		}
	}

	if len(unreachable) > 0 {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("ssh login to each of the %d StaticInstances, from the master node", len(instances)),
			Observed: "- " + strings.Join(unreachable, "\n- "),
			Expected: "every StaticInstance to accept the SSHCredentials it names",
			Fix: "check spec.user and the private key of the SSHCredentials, and that the machines accept SSH " +
				"from the master node (Deckhouse adopts them from there, not from this host)",
		}
	}

	return fmt.Sprintf("all %d StaticInstances accept the credentials they name", len(instances)), nil
}

// oneLineError keeps a per-instance line to one line: the full text is in the debug log, and a
// dozen multi-line causes in one block is unreadable.
func oneLineError(err error) string {
	text := strings.TrimSpace(err.Error())
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = strings.TrimSpace(text[:idx]) + " …"
	}
	return text
}

func parseResources(docs []string) ([]staticInstance, map[string]*v1alpha2.SSHCredentialsSpec, error) {
	var instances []staticInstance
	creds := make(map[string]*v1alpha2.SSHCredentialsSpec)

	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		var m map[string]any
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

// staticInstanceSession builds the connection to one StaticInstance.
//
// The hop matters as much as the credentials. Deckhouse adopts static instances from the master
// node, not from the installer host, so the check has to reach them the same way — through
// whatever the master reaches them through. master is the session of the connection to the master,
// or nil when the run is local and there is no hop at all.
func staticInstanceSession(address string, cred *v1alpha2.SSHCredentialsSpec, master *session.Session) *session.Session {
	config := session.NewSession(session.Input{
		User:       cred.User,
		Port:       strconv.Itoa(cred.SSHPort),
		BecomePass: cred.SudoPasswordEncoded,
	})
	config.AddAvailableHosts(session.Host{Host: address})

	switch {
	case master == nil:
		// Nothing to hop through.
	case master.BastionHost != "":
		// The master is itself reached through a bastion, and so is everything behind it.
		config.BastionHost = master.BastionHost
		config.BastionPort = master.BastionPort
		config.BastionUser = master.BastionUser
		config.BastionPassword = master.BastionPassword
	default:
		// The master is the hop.
		config.BastionHost = master.Host()
		config.BastionPort = master.Port
		config.BastionUser = master.User
		// Deliberately not master.BecomePass: that is the sudo password, and putting it
		// here would offer it to the master's sshd as an SSH password. The master is
		// reached by key, so there is no password to carry over.
	}

	return config
}

func checkSSHAccess(ctx context.Context, sshProviderInitializer *providerinitializer.SSHProviderInitializer, address string, cred *v1alpha2.SSHCredentialsSpec) error {
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

	config := staticInstanceSession(address, cred, sess)

	if cred.PrivateSSHKey != "" {
		// The dhctl pod sets no TMPDIR and mounts / read-only; only /tmp is writable.
		tmpDir, err := os.MkdirTemp("", "preflight")
		if err != nil {
			return fmt.Errorf("failed to create tmp directory: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		privateKeyPath := filepath.Join(tmpDir, "id_rsa_preflight.key")
		if err := os.WriteFile(privateKeyPath, []byte(cred.PrivateSSHKey), 0o600); err != nil {
			return fmt.Errorf("Failed to write private key: %w", err)
		}

		pkeys = append(pkeys, session.AgentPrivateKey{Key: privateKeyPath})
	}
	client, err := sshProvider.NewStandaloneClient(ctx, config, pkeys)
	if err != nil {
		return fmt.Errorf("Cannot create SSH client: %w", err)
	}

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("Cannot connect to SSH host %s: %w", address, err)
	}
	defer client.Stop()

	if cred.User == "root" {
		cmd := client.Command("true")
		if err := cmd.Run(ctx); err != nil {
			return fmt.Errorf(
				"SSH command check failed on host %s for user %q: %w\nstderr: %s",
				address,
				cred.User,
				err,
				string(cmd.StderrBytes()),
			)
		}
		return nil
	}

	if err := checkSudo(ctx, client); err != nil {
		return fmt.Errorf(
			"sudo check failed on host %s for user %q: %w",
			address,
			cred.User,
			err,
		)
	}
	return nil
}

func StaticInstancesSSHAccess(metaConfig *config.MetaConfig, sshProviderInitializer *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := StaticInstancesSSHAccessCheck{
		SSHProviderInitializer: sshProviderInitializer,
		MetaConfig:             metaConfig,
	}
	return preflight.Check{
		Name:        StaticInstancesSSHAccessCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.LongCheckTimeout,
		Run:         check.Run,
	}
}
