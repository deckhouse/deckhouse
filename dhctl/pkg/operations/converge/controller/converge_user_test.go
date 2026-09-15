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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	ssh "github.com/deckhouse/lib-gossh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestConvergeAuthorizedKeys(t *testing.T) {
	const pub = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBjoNkgxOUgHOBR6kRCRXyO+XEcnsQ8+A6FHPExg4nMQ test@example"

	t.Run("takes the key from provider config", func(t *testing.T) {
		meta := &config.MetaConfig{
			ProviderName: "OpenStack",
			ProviderClusterConfig: map[string]json.RawMessage{
				"sshPublicKey": json.RawMessage(strconv.Quote(pub)),
			},
		}

		got, err := convergeAuthorizedKeys(meta, nil)
		require.NoError(t, err)
		require.Equal(t, []string{pub}, got)
	})

	t.Run("gcp names the field sshKey", func(t *testing.T) {
		meta := &config.MetaConfig{
			ProviderName: "GCP",
			ProviderClusterConfig: map[string]json.RawMessage{
				"sshKey": json.RawMessage(strconv.Quote(pub)),
			},
		}

		got, err := convergeAuthorizedKeys(meta, nil)
		require.NoError(t, err)
		require.Equal(t, []string{pub}, got)
	})

	t.Run("other provider fields are not mistaken for a key", func(t *testing.T) {
		meta := &config.MetaConfig{
			ProviderName: "OpenStack",
			ProviderClusterConfig: map[string]json.RawMessage{
				"sshPublicKey": json.RawMessage(strconv.Quote(pub)),
				"apiVersion":   json.RawMessage(strconv.Quote("deckhouse.io/v1")),
				"kind":         json.RawMessage(strconv.Quote("OpenStackClusterConfiguration")),
				"masterNodeGroup": json.RawMessage(
					`{"replicas":3,"instanceClass":{"flavorName":"m1.large"}}`,
				),
			},
		}

		got, err := convergeAuthorizedKeys(meta, nil)
		require.NoError(t, err)
		require.Equal(t, []string{pub}, got)
	})

	t.Run("adds public halves of the keys dhctl logs in with", func(t *testing.T) {
		keyPath := writeTestPrivateKey(t, "")

		meta := &config.MetaConfig{
			ProviderName:          "OpenStack",
			ProviderClusterConfig: map[string]json.RawMessage{"sshPublicKey": json.RawMessage(strconv.Quote(pub))},
		}

		got, err := convergeAuthorizedKeys(meta, []session.AgentPrivateKey{{Key: keyPath}})
		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Contains(t, got, pub)
		require.Contains(t, got, testPublicKey(t, keyPath, ""))
	})

	t.Run("the key dhctl logs in with is listed once", func(t *testing.T) {
		keyPath := writeTestPrivateKey(t, "")
		authorized := testPublicKey(t, keyPath, "")

		meta := &config.MetaConfig{
			ProviderName:          "OpenStack",
			ProviderClusterConfig: map[string]json.RawMessage{"sshPublicKey": json.RawMessage(strconv.Quote(authorized))},
		}

		got, err := convergeAuthorizedKeys(meta, []session.AgentPrivateKey{{Key: keyPath}})
		require.NoError(t, err)
		require.Equal(t, []string{authorized}, got)
	})

	t.Run("takes the passphrase protected key", func(t *testing.T) {
		keyPath := writeTestPrivateKey(t, "s3cret")

		meta := &config.MetaConfig{ProviderName: "OpenStack", ProviderClusterConfig: map[string]json.RawMessage{}}

		got, err := convergeAuthorizedKeys(meta, []session.AgentPrivateKey{{Key: keyPath, Passphrase: "s3cret"}})
		require.NoError(t, err)
		require.Equal(t, []string{testPublicKey(t, keyPath, "s3cret")}, got)
	})

	t.Run("an unreadable key is skipped, not fatal", func(t *testing.T) {
		encrypted := writeTestPrivateKey(t, "s3cret")

		meta := &config.MetaConfig{
			ProviderName:          "OpenStack",
			ProviderClusterConfig: map[string]json.RawMessage{"sshPublicKey": json.RawMessage(strconv.Quote(pub))},
		}

		got, err := convergeAuthorizedKeys(meta, []session.AgentPrivateKey{
			{Key: encrypted},
			{Key: filepath.Join(t.TempDir(), "missing")},
		})
		require.NoError(t, err)
		require.Equal(t, []string{pub}, got)
	})

	t.Run("no key at all is an error", func(t *testing.T) {
		meta := &config.MetaConfig{ProviderName: "OpenStack", ProviderClusterConfig: map[string]json.RawMessage{}}

		_, err := convergeAuthorizedKeys(meta, nil)
		require.Error(t, err)
	})
}

func writeTestPrivateKey(t *testing.T, passphrase string) string {
	t.Helper()

	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(private, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(private, "", []byte(passphrase))
	}
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))

	return path
}

func testPublicKey(t *testing.T, path, passphrase string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	var signer ssh.Signer
	if passphrase == "" {
		signer, err = ssh.ParsePrivateKey(content)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(content, []byte(passphrase))
	}
	require.NoError(t, err)

	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
}
