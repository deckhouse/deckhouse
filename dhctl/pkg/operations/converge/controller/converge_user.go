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

package controller

import (
	"bytes"
	gocontext "context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	ssh "github.com/deckhouse/lib-gossh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
)

const (
	// bashible's 000_add_node_users.sh.tpl deletes every local user whose GECOS is
	// "created by deckhouse" and that is absent from its NodeUser list, so ours
	// must carry a different one.
	convergeUserGecos = "dhctl converge"

	cloudConfigHeader = "#cloud-config"

	// convergeUserLifetime is two days because useradd -e disables the account at 00:00
	// on the date it is given: one day would leave a converge started at 23:50 ten
	// minutes. Two guarantee at least 24 hours, whatever time of day it started.
	convergeUserLifetime = 48 * time.Hour
)

// masterCloudConfig puts the converge user into a master's cloud-init payload. Its input
// is the payload as the manual-bootstrap-for-master secret holds it.
func masterCloudConfig(ctx gocontext.Context, metaConfig *config.MetaConfig, keys []sshconfig.AgentPrivateKey, cloudConfigB64 string) (string, error) {
	authorized, err := convergeAuthorizedKeys(ctx, metaConfig, keys)
	if err != nil {
		return "", err
	}

	return withConvergeUser(cloudConfigB64, authorized, time.Now().UTC().Add(convergeUserLifetime))
}

// operatorPrivateKeys are the keys dhctl was started with, read from the connection config
// and never from the live SSH client: converge switches that client to a user of its own,
// whose generated public key must not reach a new master's authorized_keys.
func operatorPrivateKeys(ctx *context.Context) []sshconfig.AgentPrivateKey {
	connection := ctx.SSHProviderInitializer.GetConfig()
	if connection == nil || connection.Config == nil {
		return nil
	}

	return connection.Config.PrivateKeys
}

// convergeAuthorizedKeys collects the public keys the converge user is authorized with: the
// cluster key from the provider config, found by value because its field is sshPublicKey for
// ten providers and sshKey for GCP, plus the public halves of the keys dhctl logs in with.
func convergeAuthorizedKeys(ctx gocontext.Context, metaConfig *config.MetaConfig, keys []sshconfig.AgentPrivateKey) ([]string, error) {
	collected := make([]string, 0, len(keys)+1)

	for _, field := range slices.Sorted(maps.Keys(metaConfig.ProviderClusterConfig)) {
		value := metaConfig.ProviderClusterConfig[field]

		var publicKey string
		if err := json.Unmarshal(value, &publicKey); err != nil {
			continue
		}

		// The very check the x-rules: [sshPublicKey] schema mark runs on this field.
		if err := config.ValidateSSHPublicKey(value); err != nil {
			continue
		}

		collected = append(collected, publicKey)
	}

	for _, key := range keys {
		publicKey, err := publicKeyFromPrivateKey(key)
		if err != nil {
			// An unreadable key is not a converge stopper: it may be encrypted with a
			// passphrase dhctl was not given. It is worth a warning, though — the account
			// is then left with whatever the provider configuration carries.
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Skipping an ssh key for the converge user: %v", err))
			continue
		}

		collected = append(collected, publicKey)
	}

	authorized := make([]string, 0, len(collected))
	seen := make(map[string]struct{}, len(collected))

	for _, publicKey := range collected {
		publicKey = strings.TrimSpace(publicKey)
		if _, ok := seen[publicKey]; ok {
			continue
		}

		seen[publicKey] = struct{}{}
		authorized = append(authorized, publicKey)
	}

	if len(authorized) == 0 {
		return nil, errors.New("collect authorized keys for the converge user: neither the provider cluster configuration nor the ssh keys dhctl uses contain a public key")
	}

	return authorized, nil
}

// publicKeyFromPrivateKey takes the public half of an operator key. A key given by path
// (--ssh-agent-private-keys) is read from disk; one given in a connection config carries
// its PEM in Key itself.
func publicKeyFromPrivateKey(key sshconfig.AgentPrivateKey) (string, error) {
	content := []byte(key.Key)

	if key.IsPath {
		fromFile, err := os.ReadFile(key.Key)
		if err != nil {
			return "", fmt.Errorf("read private key file: %w", err)
		}

		content = fromFile
	}

	if key.Passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(content, []byte(key.Passphrase))
		if err != nil {
			return "", fmt.Errorf("parse private key with passphrase: %w", err)
		}

		return string(ssh.MarshalAuthorizedKey(signer.PublicKey())), nil
	}

	signer, err := ssh.ParsePrivateKey(content)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	return string(ssh.MarshalAuthorizedKey(signer.PublicKey())), nil
}

// withConvergeUser appends the converge user to the manual-bootstrap-for-master payload,
// base64 in and out. The payload is copied byte for byte and only the users key is added:
// re-rendering someone else's document would put every scalar in it through yaml.v3, which
// reads YAML 1.2, while the consumer is cloud-init's PyYAML, which reads 1.1.
func withConvergeUser(cloudConfigB64 string, keys []string, expire time.Time) (string, error) {
	if len(keys) == 0 {
		return "", errors.New("render cloud-config: the converge user has no authorized keys")
	}

	decoded, err := base64.StdEncoding.DecodeString(cloudConfigB64)
	if err != nil {
		return "", fmt.Errorf("decode cloud-config: %w", err)
	}

	present, err := convergeUserPresent(decoded)
	if err != nil {
		return "", err
	}

	if present {
		return cloudConfigB64, nil
	}

	// cloud-init keeps one users list per document and the distro default user (ubuntu,
	// ec2-user) is skipped as soon as that list exists without "default" in it.
	block, err := renderUsers([]any{"default", map[string]any{
		"name":                global.ConvergeUserName,
		"gecos":               convergeUserGecos,
		"expiredate":          expire.Format(time.DateOnly),
		"lock_passwd":         true,
		"shell":               "/bin/bash",
		"sudo":                []string{"ALL=(ALL) NOPASSWD:ALL"},
		"ssh_authorized_keys": keys,
	}})
	if err != nil {
		return "", err
	}

	out := append(bytes.TrimRight(decoded, "\n"), '\n')

	return base64.StdEncoding.EncodeToString(append(out, block...)), nil
}

// convergeUserPresent reads the payload without rewriting it. A payload with a users list
// of its own is refused rather than merged into: cloud-init reads the last users key of a
// document and drops the rest, so which list survives would depend on what the provider
// appends after ours.
func convergeUserPresent(payload []byte) (bool, error) {
	// The list mixes shapes: "default" is a string, an account is a mapping.
	var doc struct {
		Users []any `yaml:"users"`
	}

	if err := yaml.Unmarshal(payload, &doc); err != nil {
		return false, fmt.Errorf("parse cloud-config: %w", err)
	}

	if len(bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(payload), []byte(cloudConfigHeader)))) == 0 {
		return false, errors.New("parse cloud-config: the document is empty")
	}

	for _, user := range doc.Users {
		named, ok := user.(map[string]any)
		if ok && named["name"] == global.ConvergeUserName {
			return true, nil
		}
	}

	if len(doc.Users) > 0 {
		return false, fmt.Errorf(
			"render cloud-config: the payload already lists %d users of its own, and cloud-init keeps one users key per document",
			len(doc.Users))
	}

	return false, nil
}

// renderUsers is the one block this converge owns, indented the way the payload around it is.
func renderUsers(users []any) ([]byte, error) {
	var out bytes.Buffer

	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)

	if err := encoder.Encode(map[string]any{"users": users}); err != nil {
		return nil, fmt.Errorf("render cloud-config: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("render cloud-config: %w", err)
	}

	return out.Bytes(), nil
}
