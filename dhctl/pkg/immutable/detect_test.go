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

package immutable

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsImmutableMaster(t *testing.T) {
	const immutableMaster = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  systemType: Immutable
`

	const mutableMaster = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
`

	const immutableWorker = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  systemType: Immutable
`

	const secret = `
apiVersion: v1
kind: Secret
metadata:
  name: some-credentials
  namespace: d8-system
data:
  key: dmFsdWU=
`

	// A templated document dhctl cannot read this early. It is not the master
	// NodeGroup, so it is skipped rather than reported.
	const templatedIngress = `
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: {{ .cloudDiscovery.ingressClass }}
`

	tests := []struct {
		name      string
		resources string
		want      bool
		wantErr   bool
	}{
		{
			name:      "empty resources",
			resources: "",
		},
		{
			name:      "immutable master among other documents",
			resources: secret + "\n---\n" + immutableMaster + "\n---\n" + immutableWorker,
			want:      true,
		},
		{
			name:      "master without systemType",
			resources: mutableMaster + "\n---\n" + immutableWorker,
		},
		{
			name:      "only an immutable worker",
			resources: immutableWorker,
		},
		{
			name:      "unparseable document that is not the master NodeGroup",
			resources: templatedIngress + "\n---\n" + immutableMaster,
			want:      true,
		},
		{
			name: "templated master NodeGroup is an error",
			resources: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  systemType: {{ .systemType }}
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsImmutableMaster(tt.resources)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "master NodeGroup")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
