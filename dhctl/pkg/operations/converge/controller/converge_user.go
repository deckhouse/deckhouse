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

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	ssh "github.com/deckhouse/lib-gossh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

const (
	convergeUserName = "d8-converge"
	// bashible's 000_add_node_users.sh.tpl deletes every local user whose GECOS is
	// "created by deckhouse" and that is absent from its NodeUser list, so ours
	// must carry a different one.
	convergeUserGecos = "dhctl converge"

	cloudConfigHeader = "#cloud-config"
)

// convergeAuthorizedKeys collects the public keys the converge user is authorized with: the
// cluster key from the provider config, found by value because its field is sshPublicKey for
// ten providers and sshKey for GCP, plus the public halves of the keys dhctl logs in with.
func convergeAuthorizedKeys(metaConfig *config.MetaConfig, keys []session.AgentPrivateKey) ([]string, error) {
	ctx := gocontext.Background()

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
		publicKey, err := publicKeyFromPrivateKeyFile(key)
		if err != nil {
			// An unreadable key is not a converge stopper: it may be encrypted
			// with a passphrase dhctl was not given.
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Skipping ssh key %s for the converge user: %v", key.Key, err))
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

func publicKeyFromPrivateKeyFile(key session.AgentPrivateKey) (string, error) {
	content, err := os.ReadFile(key.Key)
	if err != nil {
		return "", fmt.Errorf("read private key file: %w", err)
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

// withConvergeUser adds the converge user to a base64-encoded cloud-config payload and
// returns it base64-encoded again. Re-applying it to its own output is a no-op.
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

		if named["name"] == convergeUserName {
			return cloudConfigB64, nil
		}
	}

	doc["users"] = append(users, map[string]any{
		"name":                convergeUserName,
		"gecos":               convergeUserGecos,
		"expiredate":          expire.Format(time.DateOnly),
		"lock_passwd":         true,
		"sudo":                []string{"ALL=(ALL) NOPASSWD:ALL"},
		"ssh_authorized_keys": keys,
	})

	var out bytes.Buffer
	out.WriteString(cloudConfigHeader + "\n")

	// The payload is capped by some providers (16 KB on AWS), and the default
	// indent of four costs a couple of kilobytes on a bootstrap script this long.
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

func cloudConfigUsers(doc map[string]any) ([]any, error) {
	value, ok := doc["users"]
	if !ok || value == nil {
		return nil, nil
	}

	users, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("parse cloud-config: users is %T, not a list", value)
	}

	return users, nil
}
