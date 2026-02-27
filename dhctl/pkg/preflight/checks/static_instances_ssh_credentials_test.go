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
