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
	gocontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	ssh "github.com/deckhouse/lib-gossh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
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
