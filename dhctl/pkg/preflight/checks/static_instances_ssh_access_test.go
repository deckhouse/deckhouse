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
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha2"
)

func TestParseSSHCredentials(t *testing.T) {
	t.Run("ok: private key, default port", func(t *testing.T) {
		key := "KEY"
		keyB64 := base64.StdEncoding.EncodeToString([]byte(key))

		sc := &v1alpha2.SSHCredentials{}
		sc.SetName("cred-1")
		sc.Spec.User = "ubuntu"
		sc.Spec.PrivateSSHKey = keyB64
		sc.Spec.SSHPort = 0

		got, err := parseSSHCredentials(sc)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if got.User != "ubuntu" {
			t.Fatalf("unexpected user: %q", got.User)
		}
		if got.PrivateSSHKey != key {
			t.Fatalf("unexpected private key: %q", got.PrivateSSHKey)
		}
		if got.SSHPort != 22 {
			t.Fatalf("expected port 22, got %d", got.SSHPort)
		}
	})

	t.Run("ok: sudo password, custom port", func(t *testing.T) {
		pass := "PASS"
		passB64 := base64.StdEncoding.EncodeToString([]byte(pass))

		sc := &v1alpha2.SSHCredentials{}
		sc.SetName("cred-1")
		sc.Spec.User = "root"
		sc.Spec.SudoPasswordEncoded = passB64
		sc.Spec.SSHPort = 2222

		got, err := parseSSHCredentials(sc)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if got.SudoPasswordEncoded != pass {
			t.Fatalf("unexpected sudo password: %q", got.SudoPasswordEncoded)
		}
		if got.SSHPort != 2222 {
			t.Fatalf("expected port 2222, got %d", got.SSHPort)
		}
	})

	t.Run("err: empty name", func(t *testing.T) {
		sc := &v1alpha2.SSHCredentials{}
		sc.Spec.User = "ubuntu"
		sc.Spec.PrivateSSHKey = base64.StdEncoding.EncodeToString([]byte("k"))

		_, err := parseSSHCredentials(sc)
		if err == nil || !strings.Contains(err.Error(), "metadata.name is empty") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("err: empty user", func(t *testing.T) {
		sc := &v1alpha2.SSHCredentials{}
		sc.SetName("c")
		sc.Spec.User = "   "
		sc.Spec.PrivateSSHKey = base64.StdEncoding.EncodeToString([]byte("k"))

		_, err := parseSSHCredentials(sc)
		if err == nil || !strings.Contains(err.Error(), "User must be specified") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("err: invalid base64 key", func(t *testing.T) {
		sc := &v1alpha2.SSHCredentials{}
		sc.SetName("c")
		sc.Spec.User = "ubuntu"
		sc.Spec.PrivateSSHKey = "%%%"

		_, err := parseSSHCredentials(sc)
		if err == nil || !strings.Contains(err.Error(), "Cannot decode privateSSHKey") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("err: neither key nor password", func(t *testing.T) {
		sc := &v1alpha2.SSHCredentials{}
		sc.SetName("c")
		sc.Spec.User = "ubuntu"

		_, err := parseSSHCredentials(sc)
		if err == nil || !strings.Contains(err.Error(), "Must contain privateSSHKey or sudoPasswordEncoded") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestParseResources(t *testing.T) {
	t.Run("ok: parses creds + instances, ignores unknown/empty", func(t *testing.T) {
		keyB64 := base64.StdEncoding.EncodeToString([]byte("key"))

		docs := []string{
			"   \n",
			`apiVersion: deckhouse.io/v1alpha2
kind: SomethingElse
metadata: {name: ignore-me}
`,
			`apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: cred-1
spec:
  user: ubuntu
  privateSSHKey: ` + keyB64 + `
  sshPort: 2222
`,
			`apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: node-1
spec:
  address: "10.0.0.10"
  credentialsRef:
    name: cred-1
`,
		}

		instances, creds, err := parseResources(docs)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if len(instances) != 1 {
			t.Fatalf("expected 1 instance, got %d", len(instances))
		}
		if instances[0].Name != "node-1" || instances[0].Address != "10.0.0.10" || instances[0].CredName != "cred-1" {
			t.Fatalf("unexpected instance: %#v", instances[0])
		}
		if _, ok := creds["cred-1"]; !ok {
			t.Fatalf("expected cred-1 in creds")
		}
	})

	t.Run("err: invalid YAML", func(t *testing.T) {
		_, _, err := parseResources([]string{`kind: StaticInstance: [`})
		if err == nil || !strings.Contains(err.Error(), "Cannot unmarshal YAML") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("err: StaticInstance empty name", func(t *testing.T) {
		_, _, err := parseResources([]string{`
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: ""
spec:
  address: "10.0.0.10"
  credentialsRef:
    name: cred-1
`})
		if err == nil || !strings.Contains(err.Error(), "metadata.name is empty") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("err: StaticInstance empty address", func(t *testing.T) {
		_, _, err := parseResources([]string{`
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: node-1
spec:
  address: "   "
  credentialsRef:
    name: cred-1
`})
		if err == nil || !strings.Contains(err.Error(), "spec.address is empty") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("err: StaticInstance empty credentialsRef.name", func(t *testing.T) {
		_, _, err := parseResources([]string{`
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: node-1
spec:
  address: "10.0.0.10"
  credentialsRef:
    name: "   "
`})
		if err == nil || !strings.Contains(err.Error(), "spec.credentialsRef.name is empty") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("err: SSHCredentials wrapped error includes resource name", func(t *testing.T) {
		_, _, err := parseResources([]string{`
apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: cred-bad
spec:
  user: "   "
  privateSSHKey: a2V5
`})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "SSHCredentials cred-bad:") {
			t.Fatalf("expected wrapped error with name, got: %v", err)
		}
	})
}

// TestStaticInstanceSession pins how a StaticInstance is reached. Deckhouse adopts these machines
// from the master node, so the check has to take the same route — and the route is the part of
// this check that nothing exercised.
func TestStaticInstanceSession(t *testing.T) {
	cred := &v1alpha2.SSHCredentialsSpec{User: "caretaker", SSHPort: 2222}

	masterWith := func(input session.Input) *session.Session {
		sess := session.NewSession(input)
		if len(input.AvailableHosts) == 0 {
			sess.AddAvailableHosts(session.Host{Host: "master-0.example.com"})
		}
		return sess
	}

	t.Run("a local run reaches the instance directly", func(t *testing.T) {
		// dhctl running on a machine that already has the hosts in reach: there is no
		// master connection to borrow, and inventing a hop would only fail.
		got := staticInstanceSession("10.0.0.10", cred, nil)

		assert.Equal(t, "10.0.0.10", got.Host())
		assert.Equal(t, "caretaker", got.User)
		assert.Equal(t, "2222", got.Port)
		assert.Empty(t, got.BastionHost)
	})

	t.Run("the master becomes the hop", func(t *testing.T) {
		master := masterWith(session.Input{User: "ubuntu", Port: "22"})

		got := staticInstanceSession("10.0.0.10", cred, master)

		assert.Equal(t, "master-0.example.com", got.BastionHost)
		assert.Equal(t, "ubuntu", got.BastionUser)
		assert.Equal(t, "22", got.BastionPort)
		// The instance's own credentials are unaffected by whose machine the hop is.
		assert.Equal(t, "caretaker", got.User)
		assert.Equal(t, "2222", got.Port)
	})

	t.Run("an existing bastion is kept", func(t *testing.T) {
		// The master is behind a bastion, so everything behind the master is too. Replacing
		// it with the master would name a host this process cannot reach.
		master := masterWith(session.Input{
			User:            "ubuntu",
			Port:            "22",
			BastionHost:     "bastion.example.com",
			BastionPort:     "2200",
			BastionUser:     "jump",
			BastionPassword: "bastion-secret",
		})

		got := staticInstanceSession("10.0.0.10", cred, master)

		assert.Equal(t, "bastion.example.com", got.BastionHost)
		assert.Equal(t, "2200", got.BastionPort)
		assert.Equal(t, "jump", got.BastionUser)
		assert.Equal(t, "bastion-secret", got.BastionPassword)
	})

	t.Run("the sudo password is not offered to the hop as an SSH password", func(t *testing.T) {
		// --ask-become-pass puts the sudo password in BecomePass. It used to be copied into
		// BastionPassword when the master became the hop, which offers the operator's sudo
		// password to the master's sshd as a password attempt. The master is reached by key;
		// there is no SSH password to carry over.
		master := masterWith(session.Input{User: "ubuntu", Port: "22", BecomePass: "sudo-secret"})

		got := staticInstanceSession("10.0.0.10", cred, master)

		assert.Empty(t, got.BastionPassword)
	})

	t.Run("the instance carries its own sudo password", func(t *testing.T) {
		withSudo := &v1alpha2.SSHCredentialsSpec{User: "caretaker", SSHPort: 22, SudoPasswordEncoded: "instance-sudo"}

		got := staticInstanceSession("10.0.0.10", withSudo, nil)

		assert.Equal(t, "instance-sudo", got.BecomePass)
	})
}
