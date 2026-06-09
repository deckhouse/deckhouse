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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, v1alpha2.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func newUpdatePolicy(name string) *v1alpha2.ModuleUpdatePolicy {
	return &v1alpha2.ModuleUpdatePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func newModuleConfigWithPolicy(name, policy string) *v1alpha1.ModuleConfig {
	return &v1alpha1.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.ModuleConfigSpec{
			UpdatePolicy: policy,
		},
	}
}

// TestUpdatePolicyHandler covers the deletion guard that forbids removing a
// ModuleUpdatePolicy while it is still referenced by a ModuleConfig.
func TestUpdatePolicyHandler(t *testing.T) {
	tests := []struct {
		name        string
		policy      string
		configs     []client.Object
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "policy not referenced by any config is allowed",
			policy:      "free-policy",
			configs:     []client.Object{newModuleConfigWithPolicy("cfg-a", "other-policy")},
			wantAllowed: true,
		},
		{
			name:        "policy referenced by a config is rejected",
			policy:      "used-policy",
			configs:     []client.Object{newModuleConfigWithPolicy("cfg-a", "used-policy")},
			wantAllowed: false,
			wantMessage: "is used by the 'cfg-a' module config",
		},
		{
			name:   "policy referenced by one of several configs is rejected",
			policy: "shared-policy",
			configs: []client.Object{
				newModuleConfigWithPolicy("cfg-a", "other-policy"),
				newModuleConfigWithPolicy("cfg-b", "shared-policy"),
			},
			wantAllowed: false,
			wantMessage: "is used by the 'cfg-b' module config",
		},
		{
			name:        "no configs at all is allowed",
			policy:      "lonely-policy",
			configs:     nil,
			wantAllowed: true,
		},
		{
			name:        "configs without any policy reference are allowed",
			policy:      "free-policy",
			configs:     []client.Object{newModuleConfigWithPolicy("cfg-a", "")},
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := newFakeClient(t, tt.configs...)
			handler := updatePolicyHandler(cli)

			review := newModuleConfigAdmissionReview("DELETE", nil, newUpdatePolicy(tt.policy))

			resp := callHandler(t, handler, review)

			if tt.wantAllowed {
				assert.True(t, resp.Allowed)
				return
			}

			require.False(t, resp.Allowed)
			require.NotNil(t, resp.Result)
			assert.Contains(t, resp.Result.Message, tt.wantMessage)
		})
	}
}
