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

package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

// TestStaticConfigurationHandler covers the deletion guard and the
// "absent/empty configuration is allowed" shortcut of the static cluster
// configuration secret.
func TestStaticConfigurationHandler(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		secret      *v1.Secret
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "delete is always rejected",
			operation:   "DELETE",
			secret:      newSecret(staticClusterConfigurationSecretName, nil, nil),
			wantAllowed: false,
			wantMessage: "forbidden to delete",
		},
		{
			name:        "create with missing configuration key is allowed",
			operation:   "CREATE",
			secret:      newSecret(staticClusterConfigurationSecretName, nil, map[string][]byte{}),
			wantAllowed: true,
		},
		{
			name:      "create with empty configuration is allowed",
			operation: "CREATE",
			secret: newSecret(staticClusterConfigurationSecretName, nil, map[string][]byte{
				staticClusterConfigurationSecretDataKey: []byte(""),
			}),
			wantAllowed: true,
		},
		{
			name:      "create with invalid configuration is rejected",
			operation: "CREATE",
			secret: newSecret(staticClusterConfigurationSecretName, nil, map[string][]byte{
				staticClusterConfigurationSecretDataKey: []byte("this: is: not: valid: static: configuration"),
			}),
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := staticConfigurationHandler(nil)

			var review = newModuleConfigAdmissionReview(tt.operation, tt.secret, nil)
			if tt.operation == "DELETE" {
				review = newModuleConfigAdmissionReview(tt.operation, nil, tt.secret)
			}

			resp := callHandler(t, handler, review)

			if tt.wantAllowed {
				assert.True(t, resp.Allowed)
				return
			}

			require.False(t, resp.Allowed)
			if tt.wantMessage != "" {
				require.NotNil(t, resp.Result)
				assert.Contains(t, resp.Result.Message, tt.wantMessage)
			}
		})
	}
}
