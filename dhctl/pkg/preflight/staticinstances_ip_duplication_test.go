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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestCheckSIIPIntersection(t *testing.T) {
	type fields struct {
		metaConfig *config.MetaConfig
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "happy path: 2 instances, different addresses",
			fields: fields{metaConfig: &config.MetaConfig{
				ResourcesYAML: `---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 10.128.0.22
  credentialsRef:
    kind: SSHCredentials
    name: credentials
---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-1
spec:
  address: 10.128.0.23
  credentialsRef:
    kind: SSHCredentials
    name: credentials
`,
			}},
			wantErr: assert.NoError,
		},
		{
			name: "happy path: no instances",
			fields: fields{metaConfig: &config.MetaConfig{
				ResourcesYAML: ``,
			}},
			wantErr: assert.NoError,
		},
		{
			name: "happy path: single instance",
			fields: fields{metaConfig: &config.MetaConfig{
				ResourcesYAML: `---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 10.128.0.22
  credentialsRef:
    kind: SSHCredentials
    name: credentials
`,
			}},
			wantErr: assert.NoError,
		},
		{
			name: "intersects addresses",
			fields: fields{metaConfig: &config.MetaConfig{
				ResourcesYAML: `---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 10.128.0.22
  credentialsRef:
    kind: SSHCredentials
    name: credentials
---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-1
spec:
  address: 10.128.0.23
  credentialsRef:
    kind: SSHCredentials
    name: credentials
---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-2
spec:
  address: 10.128.0.24
  credentialsRef:
    kind: SSHCredentials
    name: credentials
---
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-3
spec:
  address: 10.128.0.22
  credentialsRef:
    kind: SSHCredentials
    name: credentials
`,
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "Duplicate address")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &Checker{
				metaConfig: tt.fields.metaConfig,
			}
			tt.wantErr(t,
				pc.CheckStaticInstancesIPDuplication(context.Background()),
				fmt.Sprintf("CheckStaticInstancesIPDuplication()"),
			)
		})
	}
}
