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

	held, err := heldPublicKeys(configured, authSockOf(c.SSHProviderInitializer))
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
			return fmt.Sprintf("%s matches %s.sshPublicKey (%s)",
				key.source, providerDocumentKind(c.MetaConfig.ProviderName), ssh.FingerprintSHA256(declared)), nil
		}
	}

	offered := make([]string, 0, len(held))
	for _, key := range held {
		offered = append(offered, fmt.Sprintf("%s (%s)", ssh.FingerprintSHA256(key.publicKey), key.source))
	}

	document := providerDocumentKind(c.MetaConfig.ProviderName)
	named := len(configured) > 0

	expected := "dhctl to hold the private key of the public key the cloud installs"
	fix := fmt.Sprintf("pass the private key of sshPublicKey with --ssh-agent-private-keys, "+
		"or set %s.sshPublicKey to the public key of the key you are passing", document)
	if named {
		// The operator named a key. A running agent may hold the right one as well, and the
		// connection would offer both — but the key they asked for is still the wrong one, and
		// each wrong key offered spends one of the few authentication attempts the node allows.
		expected = "the key named with --ssh-agent-private-keys to be the one the cloud installs"
		fix = fmt.Sprintf("pass the private key of %s.sshPublicKey with --ssh-agent-private-keys "+
			"(drop the flag to use the keys your ssh-agent holds), or set %s.sshPublicKey to the "+
			"public key of the key you are passing", document, document)
	}

	return "", preflight.Permanent(&preflight.Failure{
		Checked:  fmt.Sprintf("%s.sshPublicKey against the private keys dhctl will offer", document),
		Observed: fmt.Sprintf("the cloud will install %s; dhctl holds %s", ssh.FingerprintSHA256(declared), strings.Join(offered, ", ")),
		Expected: expected,
		Fix:      fix,
	})
}

// providerDocumentKind names the document the key came from the way the operator wrote it.
// MetaConfig.ProviderName is lowercased while the configuration is parsed, so printing it raw
// produced "yandexClusterConfiguration" — a document nobody has.
func providerDocumentKind(providerName string) string {
	if kind, ok := providerDocumentKinds[strings.ToLower(providerName)]; ok {
		return kind
	}
	return "<Provider>ClusterConfiguration"
}

// providerDocumentKinds mirrors config.cloudProviderToProviderKind, keyed by the lowercased name
// this side has. It is a copy rather than an import: pkg/config imports the preflight packages.
var providerDocumentKinds = map[string]string{
	"openstack":   "OpenStackClusterConfiguration",
	"aws":         "AWSClusterConfiguration",
	"gcp":         "GCPClusterConfiguration",
	"yandex":      "YandexClusterConfiguration",
	"vsphere":     "VsphereClusterConfiguration",
	"azure":       "AzureClusterConfiguration",
	"vcd":         "VCDClusterConfiguration",
	"zvirt":       "ZvirtClusterConfiguration",
	"huaweicloud": "HuaweiCloudClusterConfiguration",
	"dynamix":     "DynamixClusterConfiguration",
	"dvp":         "DVPClusterConfiguration",
}

// heldKey is a public key dhctl can authenticate with, and where it came from — the source is what
// makes the failure actionable, since "dhctl holds a different key" reads very differently for a
// key the operator passed and for one their agent happens to have loaded.
type heldKey struct {
	publicKey ssh.PublicKey
	source    string
}

// heldPublicKeys collects the keys this configuration says the master will be reached with.
//
// Named keys win outright: --ssh-agent-private-keys is the operator saying which key to use, and
// if the one they named is not the one the cloud installs, that is worth reporting even when a
// running agent happens to hold a match. It was the union once, and the union let exactly this
// through — a wrong key on the flag, the right one in the agent, the check green and the bootstrap
// failing on the master minutes later. The connection does offer both, so a union is not wrong in
// principle; what it is not is what the operator asked for, and in practice the extra wrong key
// spends one of the server's few permitted authentication attempts.
//
// The agent is read only when no key was named. That is the common case — most operators keep
// theirs in an agent — and it is what makes this check apply at all rather than give up.
func heldPublicKeys(configured []sshconfig.AgentPrivateKey, authSock string) ([]heldKey, error) {
	signers, err := bastionSigners(configured)
	if err != nil {
		return nil, err
	}

	if len(signers) == 0 {
		return agentPublicKeys(authSock), nil
	}

	held := make([]heldKey, 0, len(signers))
	for i, signer := range signers {
		source := "the key given with --ssh-agent-private-keys"
		if len(signers) > 1 {
			source = fmt.Sprintf("key %d of --ssh-agent-private-keys", i+1)
		}
		held = append(held, heldKey{publicKey: signer.PublicKey(), source: source})
	}

	return held, nil
}

// authSockOf is the agent socket dhctl will use, which is not always the one in the environment:
// the settings carry an explicit path that overrides it. Reading the environment directly was
// wrong — the check would consult an agent the connection never talks to, and pass on a key that
// is never offered.
func authSockOf(initializer *providerinitializer.SSHProviderInitializer) string {
	sett := initializer.GetSettings()
	if sett == nil {
		return ""
	}
	return sett.AuthSock()
}

// agentPublicKeys asks the ssh-agent at socket what it holds. A missing or unreachable agent is
// not an error: it only means there is nothing to add.
func agentPublicKeys(socket string) []heldKey {
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
