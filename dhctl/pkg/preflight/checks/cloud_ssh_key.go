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
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

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

	var configured []sshconfig.AgentPrivateKey
	if connCfg != nil && connCfg.Config != nil {
		configured = connCfg.Config.PrivateKeys
	}

	held, err := heldPublicKeys(configured)
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  "the private keys given with --ssh-agent-private-keys",
			Observed: err.Error(),
			Expected: "keys dhctl can read and parse",
			Fix:      "check the paths, and pass the passphrase if a key has one",
			Err:      err,
		})
	}

	if len(held) == 0 {
		return "", preflight.NotApplicable(
			"dhctl was given no private key and no ssh-agent is running, so there is nothing to compare")
	}

	declaredAuthorized := ssh.MarshalAuthorizedKey(declared)
	for _, key := range held {
		if string(ssh.MarshalAuthorizedKey(key.publicKey)) == string(declaredAuthorized) {
			return fmt.Sprintf("%s matches %sClusterConfiguration.sshPublicKey (%s)",
				key.source, c.MetaConfig.ProviderName, ssh.FingerprintSHA256(declared)), nil
		}
	}

	offered := make([]string, 0, len(held))
	for _, key := range held {
		offered = append(offered, fmt.Sprintf("%s (%s)", ssh.FingerprintSHA256(key.publicKey), key.source))
	}

	return "", preflight.Permanent(&preflight.Failure{
		Checked:  fmt.Sprintf("%sClusterConfiguration.sshPublicKey against the private keys dhctl holds", c.MetaConfig.ProviderName),
		Observed: fmt.Sprintf("the cloud will install %s; dhctl holds %s", ssh.FingerprintSHA256(declared), strings.Join(offered, ", ")),
		Expected: "dhctl to hold the private key of the public key the cloud installs",
		Fix: "pass the private key of sshPublicKey with --ssh-agent-private-keys, " +
			"or set sshPublicKey in the <Provider>ClusterConfiguration to the public key of the key you are passing",
	})
}

// heldKey is a public key dhctl can authenticate with, and where it came from — the source is what
// makes the failure actionable, since "dhctl holds a different key" reads very differently for a
// key the operator passed and for one their agent happens to have loaded.
type heldKey struct {
	publicKey ssh.PublicKey
	source    string
}

// heldPublicKeys collects every key dhctl could offer the master: the ones given with
// --ssh-agent-private-keys, and the ones a running ssh-agent has loaded.
//
// The agent half is what makes this check useful rather than usually not applicable. Passing keys
// by path is the minority case — most operators have theirs in an agent — and the check used to
// give up whenever no path was given, which is exactly when the mismatch goes unnoticed until the
// infrastructure exists and the master will not accept a login.
func heldPublicKeys(configured []sshconfig.AgentPrivateKey) ([]heldKey, error) {
	signers, err := bastionSigners(configured)
	if err != nil {
		return nil, err
	}

	held := make([]heldKey, 0, len(signers))
	for i, signer := range signers {
		source := "the key given with --ssh-agent-private-keys"
		if len(signers) > 1 {
			source = fmt.Sprintf("key %d of --ssh-agent-private-keys", i+1)
		}
		held = append(held, heldKey{publicKey: signer.PublicKey(), source: source})
	}

	return append(held, agentPublicKeys()...), nil
}

// agentPublicKeys asks the running ssh-agent what it holds. A missing or unreachable agent is not
// an error: it only means there is nothing to add.
func agentPublicKeys() []heldKey {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil
	}
	defer conn.Close()

	keys, err := agent.NewClient(conn).List()
	if err != nil {
		return nil
	}

	held := make([]heldKey, 0, len(keys))
	for _, key := range keys {
		publicKey, err := ssh.ParsePublicKey(key.Blob)
		if err != nil {
			continue
		}
		source := "a key in the ssh-agent"
		if comment := strings.TrimSpace(key.Comment); comment != "" {
			source = fmt.Sprintf("the ssh-agent key %q", comment)
		}
		held = append(held, heldKey{publicKey: publicKey, source: source})
	}
	return held
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
