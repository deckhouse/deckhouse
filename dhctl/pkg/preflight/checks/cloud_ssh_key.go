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
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// CloudSSHKeyCheck compares the public key the cloud will install on the master with the private
// keys dhctl will try to log in with.
//
// When they do not match there is nothing to notice until the infrastructure has been created and
// the master is up: dhctl then waits 250 times for a second each, and gives up with "failed to
// wait for SSH connection on master", which reads like a machine that never booted. The keys are
// both known before anything is created.
type CloudSSHKeyCheck struct {
	MetaConfig             *config.MetaConfig
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
}

const CloudSSHKeyCheckName preflight.CheckName = "cloud-ssh-key-matches-public-key"

func (CloudSSHKeyCheck) Description() string {
	return "a private key dhctl holds matches the sshPublicKey the cloud will install"
}

func (CloudSSHKeyCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (CloudSSHKeyCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c CloudSSHKeyCheck) Run(_ context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", fmt.Errorf("metaConfig is required")
	}

	declared, err := declaredSSHPublicKey(c.MetaConfig)
	if err != nil {
		return "", err
	}
	if declared == nil {
		return "", preflight.NotApplicable("the <Provider>ClusterConfiguration declares no sshPublicKey")
	}

	connCfg := c.SSHProviderInitializer.GetConfig()
	if connCfg == nil || connCfg.Config == nil || len(connCfg.Config.PrivateKeys) == 0 {
		// An ssh-agent may hold the matching key without dhctl ever seeing it, so the absence of
		// a --ssh-agent-private-keys argument is not by itself a mistake.
		return "", preflight.NotApplicable("no --ssh-agent-private-keys were given; the key may come from an ssh-agent")
	}

	signers, err := bastionSigners(connCfg.Config.PrivateKeys)
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  "the private keys given with --ssh-agent-private-keys",
			Observed: err.Error(),
			Expected: "keys dhctl can read and parse",
			Fix:      "check the paths, and pass the passphrase if a key has one",
			Err:      err,
		})
	}

	declaredAuthorized := ssh.MarshalAuthorizedKey(declared)
	for _, signer := range signers {
		if string(ssh.MarshalAuthorizedKey(signer.PublicKey())) == string(declaredAuthorized) {
			return fmt.Sprintf("one of the %d private keys matches %s sshPublicKey (%s)",
				len(signers), c.MetaConfig.ProviderName, ssh.FingerprintSHA256(declared)), nil
		}
	}

	offered := make([]string, 0, len(signers))
	for _, signer := range signers {
		offered = append(offered, ssh.FingerprintSHA256(signer.PublicKey()))
	}

	return "", preflight.Permanent(&preflight.Failure{
		Checked:  fmt.Sprintf("%sClusterConfiguration.sshPublicKey against --ssh-agent-private-keys", c.MetaConfig.ProviderName),
		Observed: fmt.Sprintf("the cloud will install %s; dhctl holds %s", ssh.FingerprintSHA256(declared), strings.Join(offered, ", ")),
		Expected: "dhctl to hold the private key of the public key the cloud installs",
		Fix: "pass the private key of sshPublicKey with --ssh-agent-private-keys, " +
			"or set sshPublicKey in the <Provider>ClusterConfiguration to the public key of the key you are passing",
	})
}

// declaredSSHPublicKey reads sshPublicKey out of the provider configuration. A key that does not
// parse is a failure of its own: the cloud would reject it much later, during apply.
func declaredSSHPublicKey(meta *config.MetaConfig) (ssh.PublicKey, error) {
	raw, ok := meta.ProviderClusterConfig["sshPublicKey"]
	if !ok || len(raw) == 0 {
		return nil, nil
	}

	var authorizedKey string
	if err := json.Unmarshal(raw, &authorizedKey); err != nil {
		return nil, preflight.Permanent(&preflight.Failure{
			Checked:  "<Provider>ClusterConfiguration.sshPublicKey",
			Observed: "the field is not a string",
			Expected: "an OpenSSH public key, as in ~/.ssh/id_ed25519.pub",
			Fix:      "set sshPublicKey to the contents of the .pub file",
		})
	}
	if strings.TrimSpace(authorizedKey) == "" {
		return nil, nil
	}

	publicKey, err := parseAuthorizedKey(authorizedKey)
	if err != nil {
		return nil, preflight.Permanent(&preflight.Failure{
			Checked:  "<Provider>ClusterConfiguration.sshPublicKey",
			Observed: fmt.Sprintf("it is not an OpenSSH public key: %s", err),
			Expected: "one line of the form `ssh-ed25519 AAAA… comment`",
			Fix:      "paste the contents of the .pub file, not the private key and not a PEM block",
			Err:      err,
		})
	}
	return publicKey, nil
}

// parseAuthorizedKey keeps ssh.ParseAuthorizedKey's comment, options and trailing bytes out of
// the caller, which needs only the key.
func parseAuthorizedKey(authorizedKey string) (ssh.PublicKey, error) {
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey)) //nolint:dogsled // the other three are the comment, the options and the rest of the line
	return publicKey, err
}

func CloudSSHKey(meta *config.MetaConfig, sshProviderInitializer *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := CloudSSHKeyCheck{MetaConfig: meta, SSHProviderInitializer: sshProviderInitializer}
	return preflight.Check{
		Name:        CloudSSHKeyCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
