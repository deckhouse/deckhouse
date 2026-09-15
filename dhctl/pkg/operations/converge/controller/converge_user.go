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
// is the payload as the manual-bootstrap-for-master secret holds it. With skip set the
// payload comes back byte-identical.
func masterCloudConfig(ctx gocontext.Context, metaConfig *config.MetaConfig, keys []sshconfig.AgentPrivateKey, cloudConfigB64 string, skip bool) (string, error) {
	if skip {
		return cloudConfigB64, nil
	}

	authorized, err := convergeAuthorizedKeys(ctx, metaConfig, keys)
	if err != nil {
		return "", err
	}

	return withConvergeUser(cloudConfigB64, authorized, time.Now().UTC().Add(convergeUserLifetime))
}

// operatorPrivateKeys are the keys dhctl was started with. They are read from the
// connection config and never from the live SSH client: converge switches that client to
// a user of its own before a master is rendered, and the public half of the key it
// generates for that user must not reach a new master's authorized_keys.
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

// withConvergeUser adds the converge user to the unmodified manual-bootstrap-for-master
// payload, base64 in and out. Given its own output it returns that as is, with the keys
// and expiry already rendered there, so callers must not feed it back to refresh them.
func withConvergeUser(cloudConfigB64 string, keys []string, expire time.Time) (string, error) {
	if len(keys) == 0 {
		return "", errors.New("render cloud-config: the converge user has no authorized keys")
	}

	decoded, err := base64.StdEncoding.DecodeString(cloudConfigB64)
	if err != nil {
		return "", fmt.Errorf("decode cloud-config: %w", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(decoded, &doc); err != nil {
		return "", fmt.Errorf("parse cloud-config: %w", err)
	}

	if doc == nil {
		return "", errors.New("parse cloud-config: the document is empty")
	}

	users, err := cloudConfigUsers(doc)
	if err != nil {
		return "", err
	}

	for _, user := range users {
		named, ok := user.(map[string]any)
		if !ok {
			continue
		}

		if named["name"] == global.ConvergeUserName {
			return cloudConfigB64, nil
		}
	}

	doc["users"] = append(users, map[string]any{
		"name":                global.ConvergeUserName,
		"gecos":               convergeUserGecos,
		"expiredate":          expire.Format(time.DateOnly),
		"lock_passwd":         true,
		"shell":               "/bin/bash",
		"sudo":                []string{"ALL=(ALL) NOPASSWD:ALL"},
		"ssh_authorized_keys": keys,
	})

	var out bytes.Buffer
	out.WriteString(cloudConfigHeader + "\n")

	// yaml.v3 re-renders the whole document, and its default indent of four would push
	// every line of the bootstrap script two columns further right for nothing.
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)

	if err := encoder.Encode(doc); err != nil {
		return "", fmt.Errorf("render cloud-config: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("render cloud-config: %w", err)
	}

	return base64.StdEncoding.EncodeToString(out.Bytes()), nil
}

// cloudConfigUsers returns the list our user is appended to. cloud-init skips the distro
// default user (ubuntu, ec2-user) as soon as a users list exists and does not name
// "default", so a document with no list of its own is seeded with it.
func cloudConfigUsers(doc map[string]any) ([]any, error) {
	value, ok := doc["users"]
	if !ok || value == nil {
		return []any{"default"}, nil
	}

	users, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("parse cloud-config: users is %T, not a list", value)
	}

	return users, nil
}
