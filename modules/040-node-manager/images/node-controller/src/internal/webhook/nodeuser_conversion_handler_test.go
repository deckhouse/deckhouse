/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"testing"

	"github.com/stretchr/testify/require"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	nodeUserV1Alpha1 = `{"apiVersion":"deckhouse.io/v1alpha1","kind":"NodeUser","metadata":{"name":"legacy"},
		"spec":{"uid":1001,"sshPublicKeys":["ssh-ed25519 AAAA"],"nodeGroups":["*"],"futureField":"kept"}}`
	nodeUserV1Alpha1AsV1 = `{"apiVersion":"deckhouse.io/v1","kind":"NodeUser","metadata":{"name":"legacy"},
		"spec":{"uid":1001,"sshPublicKeys":["ssh-ed25519 AAAA"],"nodeGroups":["*"],"futureField":"kept"},
		"status":{"errors":{}}}`
	nodeUserV1 = `{"apiVersion":"deckhouse.io/v1","kind":"NodeUser","metadata":{"name":"modern"},
		"spec":{"uid":1002,"sshPublicKeys":["ssh-ed25519 BBBB"],"isSudoer":true},
		"status":{"errors":{"node-1":"useradd failed"}}}`
	nodeUserV1AsV1Alpha1 = `{"apiVersion":"deckhouse.io/v1alpha1","kind":"NodeUser","metadata":{"name":"modern"},
		"spec":{"uid":1002,"sshPublicKeys":["ssh-ed25519 BBBB"],"isSudoer":true}}`
)

func TestConvertNodeUser(t *testing.T) {
	tests := []struct {
		name           string
		desiredVersion string
		objects        []string
		expObjects     []string
	}{
		{
			// An object stored in etcd as v1alpha1 is defaulted by the v1alpha1 schema, which has
			// no status, so the v1 status default has to come from the conversion.
			name:           "v1alpha1 to v1 adds the empty status",
			desiredVersion: "deckhouse.io/v1",
			objects:        []string{nodeUserV1Alpha1},
			expObjects:     []string{nodeUserV1Alpha1AsV1},
		},
		{
			name:           "v1 to v1alpha1 drops the status",
			desiredVersion: "deckhouse.io/v1alpha1",
			objects:        []string{nodeUserV1},
			expObjects:     []string{nodeUserV1AsV1Alpha1},
		},
		{
			name:           "objects already in the desired version stay as they are",
			desiredVersion: "deckhouse.io/v1",
			objects:        []string{nodeUserV1Alpha1, nodeUserV1},
			expObjects:     []string{nodeUserV1Alpha1AsV1, nodeUserV1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &apix.ConversionRequest{UID: "uid", DesiredAPIVersion: tc.desiredVersion}
			for _, o := range tc.objects {
				req.Objects = append(req.Objects, runtime.RawExtension{Raw: []byte(o)})
			}

			resp := (&ConversionHandler{}).handleConversion(t.Context(), req, &ProviderClusterConfiguration{})

			require.Equal(t, metav1.StatusSuccess, resp.Result.Status, resp.Result.Message)
			require.Len(t, resp.ConvertedObjects, len(tc.expObjects))
			for i, exp := range tc.expObjects {
				require.JSONEq(t, exp, string(resp.ConvertedObjects[i].Raw))
			}
		})
	}
}

func TestConvertNodeUser_UnsupportedVersion(t *testing.T) {
	_, err := (&ConversionHandler{}).convertObject(t.Context(), []byte(nodeUserV1), "deckhouse.io/v1alpha2", &ProviderClusterConfiguration{})

	require.ErrorContains(t, err, "unsupported NodeUser conversion from deckhouse.io/v1 to deckhouse.io/v1alpha2")
}
