// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestCheckStaticInstancesSSH(t *testing.T) {
	fakeSudo := base64.StdEncoding.EncodeToString([]byte("supersecret"))
	fakeKey := base64.StdEncoding.EncodeToString([]byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBALe5t...
-----END RSA PRIVATE KEY-----`))

	tests := []struct {
		name    string
		yaml    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "happy path: sudoPasswordEncoded present",
			yaml: `---
apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: sudo-creds
spec:
  user: caps
  sudoPasswordEncoded: "` + fakeSudo + `"
---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-sudo
spec:
  address: 10.128.1.10
  credentialsRef:
    kind: SSHCredentials
    name: sudo-creds
`,
			wantErr: assert.Error,
		},
		{
			name: "happy path: privateSSHKey present",
			yaml: `---
apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: key-creds
spec:
  user: caps
  privateSSHKey: "` + fakeKey + `"
---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-key
spec:
  address: 10.128.1.11
  credentialsRef:
    kind: SSHCredentials
    name: key-creds
`,
			wantErr: assert.Error,
		},
		{
			name: "error: invalid Base64 sudo password",
			yaml: `---
apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: bad-sudo
spec:
  user: caps
  sudoPasswordEncoded: "!!!INVALID!!!"
---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-bad-sudo
spec:
  address: 10.128.1.12
  credentialsRef:
    kind: SSHCredentials
    name: bad-sudo
`,
			wantErr: assert.Error,
		},
		{
			name: "error: missing both key and sudo password",
			yaml: `---
apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: empty-creds
spec:
  user: caps
---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-empty
spec:
  address: 10.128.1.13
  credentialsRef:
    kind: SSHCredentials
    name: empty-creds
`,
			wantErr: assert.Error,
		},
		{
			name: "error: SSHCredentials not found",
			yaml: `---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-missing-creds
spec:
  address: 10.128.1.14
  credentialsRef:
    kind: SSHCredentials
    name: missing
`,
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &Checker{
				metaConfig: &config.MetaConfig{
					ResourcesYAML: tt.yaml,
				},
			}

			tt.wantErr(t, pc.CheckStaticInstancesSSH(context.Background()))
		})
	}
}
