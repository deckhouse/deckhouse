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
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

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

func TestWithConvergeUser(t *testing.T) {
	const base = `#cloud-config
package_update: false
write_files:
- path: '/var/lib/bashible/bootstrap.sh'
  permissions: '0700'
  content: |
    #!/bin/bash
    echo hi
runcmd:
- /var/lib/bashible/bootstrap.sh
`
	expire := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	keys := []string{"ssh-ed25519 AAAAC3 test@example"}

	render := func(t *testing.T, in string) map[string]any {
		out, err := withConvergeUser(base64.StdEncoding.EncodeToString([]byte(in)), keys, expire)
		require.NoError(t, err)

		raw, err := base64.StdEncoding.DecodeString(out)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(string(raw), "#cloud-config\n"))

		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(raw, &doc))
		return doc
	}

	convergeUser := func(t *testing.T, doc map[string]any) map[string]any {
		users, ok := doc["users"].([]any)
		require.True(t, ok)
		require.Len(t, users, 2)
		return users[1].(map[string]any)
	}

	t.Run("user carries name, expiry, keys and sudo", func(t *testing.T) {
		user := convergeUser(t, render(t, base))

		require.Equal(t, "d8-converge", user["name"])
		require.Equal(t, "2026-09-16", user["expiredate"])
		require.Equal(t, true, user["lock_passwd"])
		require.Equal(t, []any{keys[0]}, user["ssh_authorized_keys"])
		require.Equal(t, []any{"ALL=(ALL) NOPASSWD:ALL"}, user["sudo"])
		// cloud-init passes --shell to useradd only when the key is there, and the
		// Debian default is /bin/sh. dhctl drives commands over this account.
		require.Equal(t, "/bin/bash", user["shell"])
	})

	// cloud-init creates the distro default user (ubuntu, ec2-user) only while the users
	// list is absent or names "default". None of the real payloads carry a users key, so
	// without the marker our user would be the only one on the node.
	t.Run("the distro default user is kept", func(t *testing.T) {
		doc := render(t, base)

		require.Equal(t, "default", doc["users"].([]any)[0])
	})

	// bashible removes users whose comment is "created by deckhouse" and that are
	// absent from its NodeUser list. Ours must not look like one.
	t.Run("gecos is set and is not the bashible marker", func(t *testing.T) {
		user := convergeUser(t, render(t, base))

		require.Equal(t, "dhctl converge", user["gecos"])
	})

	t.Run("bootstrap payload is untouched", func(t *testing.T) {
		doc := render(t, base)

		require.Equal(t, []any{"/var/lib/bashible/bootstrap.sh"}, doc["runcmd"])
		require.Len(t, doc["write_files"].([]any), 1)
		require.Equal(t, false, doc["package_update"])

		written := doc["write_files"].([]any)[0].(map[string]any)
		require.Equal(t, "#!/bin/bash\necho hi\n", written["content"])
		require.Equal(t, "/var/lib/bashible/bootstrap.sh", written["path"])
		require.Equal(t, "0700", written["permissions"])
	})

	// The whole render is a yaml round-trip of the node-controller payload, so the guard
	// that matters is a real one going through it unchanged but for our users key.
	t.Run("a real node-controller payload survives the round-trip", func(t *testing.T) {
		const golden = "../../../../../modules/040-node-manager/images/node-controller/src/internal/bootstrap/testdata/golden/mcm-aws-userData.txt"

		payload, err := os.ReadFile(golden)
		require.NoError(t, err)

		var before map[string]any
		require.NoError(t, yaml.Unmarshal(payload, &before))
		require.NotContains(t, before, "users")

		after := render(t, string(payload))
		require.Equal(t, "default", after["users"].([]any)[0])
		require.Equal(t, convergeUserName, convergeUser(t, after)["name"])

		delete(after, "users")
		require.Equal(t, before, after)
	})

	t.Run("applying twice changes nothing", func(t *testing.T) {
		once, err := withConvergeUser(base64.StdEncoding.EncodeToString([]byte(base)), keys, expire)
		require.NoError(t, err)
		twice, err := withConvergeUser(once, keys, expire)
		require.NoError(t, err)
		require.Equal(t, once, twice)
	})

	t.Run("a user with no keys is an error, not a locked door", func(t *testing.T) {
		_, err := withConvergeUser(base64.StdEncoding.EncodeToString([]byte(base)), nil, expire)
		require.Error(t, err)
	})

	t.Run("a payload that is not a cloud-config is an error", func(t *testing.T) {
		_, err := withConvergeUser(base64.StdEncoding.EncodeToString([]byte("#cloud-config\n")), keys, expire)
		require.Error(t, err)
	})

	t.Run("cloud-config users of the provider survive", func(t *testing.T) {
		doc := render(t, base+"users:\n- name: user\n  sudo: 'ALL=(ALL) NOPASSWD:ALL'\n")

		users, ok := doc["users"].([]any)
		require.True(t, ok)
		require.Len(t, users, 2)
		require.Equal(t, "user", users[0].(map[string]any)["name"])
		require.Equal(t, "d8-converge", users[1].(map[string]any)["name"])
	})

	// A sshPublicKey field may hold several keys separated by newlines, so an
	// authorized key can be a multi-line string. It must stay one list element.
	t.Run("a multi-line key stays one list element", func(t *testing.T) {
		multiline := "ssh-ed25519 AAAAC3 first@example\nssh-ed25519 AAAAC4 second@example"

		out, err := withConvergeUser(base64.StdEncoding.EncodeToString([]byte(base)), []string{multiline}, expire)
		require.NoError(t, err)

		raw, err := base64.StdEncoding.DecodeString(out)
		require.NoError(t, err)

		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(raw, &doc))
		require.Equal(t, []any{multiline}, convergeUser(t, doc)["ssh_authorized_keys"])
	})

	t.Run("a multi-line provider key reaches the render intact", func(t *testing.T) {
		const twoKeys = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBjoNkgxOUgHOBR6kRCRXyO+XEcnsQ8+A6FHPExg4nMQ first@example\n" +
			"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBjoNkgxOUgHOBR6kRCRXyO+XEcnsQ8+A6FHPExg4nMQ second@example"

		meta := &config.MetaConfig{
			ProviderName:          "OpenStack",
			ProviderClusterConfig: map[string]json.RawMessage{"sshPublicKey": json.RawMessage(strconv.Quote(twoKeys))},
		}

		authorized, err := convergeAuthorizedKeys(meta, nil)
		require.NoError(t, err)
		require.Equal(t, []string{twoKeys}, authorized)

		out, err := withConvergeUser(base64.StdEncoding.EncodeToString([]byte(base)), authorized, expire)
		require.NoError(t, err)

		raw, err := base64.StdEncoding.DecodeString(out)
		require.NoError(t, err)

		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(raw, &doc))
		require.Equal(t, []any{twoKeys}, convergeUser(t, doc)["ssh_authorized_keys"])
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
