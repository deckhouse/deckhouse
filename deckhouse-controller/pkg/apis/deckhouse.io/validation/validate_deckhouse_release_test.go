/*
Copyright 2025 Flant JSC

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
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/slok/kubewebhook/v2/pkg/model"
)

type mockModuleManager struct {
	enabledModules []string
}

func (m *mockModuleManager) GetEnabledModuleNames() []string {
	return m.enabledModules
}

var secret = &corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "kube-system",
		Name:      "d8-cluster-configuration",
	},
	Data: map[string][]byte{
		"cluster-configuration.yaml": []byte("kubernetesVersion: 1.31"),
	},
}

func TestValidateDeckhouseReleaseApproval(t *testing.T) {
	err := os.Setenv("TEST_EXTENDER_KUBERNETES_VERSION", "1.28.0")
	require.NoError(t, err)
	defer os.Unsetenv("TEST_EXTENDER_KUBERNETES_VERSION")

	tests := []struct {
		name           string
		release        *v1alpha1.DeckhouseRelease
		oldRelease     *v1alpha1.DeckhouseRelease
		enabledModules []string
		operation      string
		shouldAllow    bool
		expectedError  string
	}{
		{
			name: "allow when not approved",
			release: &v1alpha1.DeckhouseRelease{
				ObjectMeta: metav1.ObjectMeta{Name: "test-release"},
				Spec:       v1alpha1.DeckhouseReleaseSpec{Version: "v1.30.0"},
				Approved:   false,
			},
			operation:   "CREATE",
			shouldAllow: true,
		},
		{
			name: "allow update if already approved",
			release: &v1alpha1.DeckhouseRelease{
				ObjectMeta: metav1.ObjectMeta{Name: "test-release"},
				Spec:       v1alpha1.DeckhouseReleaseSpec{Version: "v1.30.0"},
				Approved:   true,
			},
			oldRelease: &v1alpha1.DeckhouseRelease{
				ObjectMeta: metav1.ObjectMeta{Name: "test-release"},
				Spec:       v1alpha1.DeckhouseReleaseSpec{Version: "v1.30.0"},
				Approved:   true,
			},
			operation:   "UPDATE",
			shouldAllow: true,
		},
		{
			name: "reject when requirements not met",
			release: &v1alpha1.DeckhouseRelease{
				ObjectMeta: metav1.ObjectMeta{Name: "test-release"},
				Spec: v1alpha1.DeckhouseReleaseSpec{
					Version:      "v1.30.0",
					Requirements: map[string]string{"k8s": "9.9.9"},
				},
				Approved: true,
			},
			oldRelease: &v1alpha1.DeckhouseRelease{
				ObjectMeta: metav1.ObjectMeta{Name: "test-release"},
				Spec: v1alpha1.DeckhouseReleaseSpec{
					Version:      "v1.30.0",
					Requirements: map[string]string{"k8s": "9.9.9"},
				},
				Approved: false,
			},
			operation:     "UPDATE",
			shouldAllow:   false,
			expectedError: "cannot approve DeckhouseRelease",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithObjects(secret).Build()
			mm := &mockModuleManager{enabledModules: tt.enabledModules}
			exts := extenders.NewExtendersStack(&edition.Edition{Version: "v1.30.0"}, nil, log.NewLogger())

			var oldRaw []byte
			if tt.oldRelease != nil {
				oldRaw, _ = json.Marshal(tt.oldRelease)
			}

			review := &model.AdmissionReview{
				Operation:    model.AdmissionReviewOp(tt.operation),
				OldObjectRaw: oldRaw,
			}

			ctx := context.Background()
			res, err := validateDeckhouseReleaseApproval(ctx, review, tt.release, client, mm, exts)
			if tt.shouldAllow {
				assert.NoError(t, err)
				assert.True(t, res.Valid)
			}
			if !tt.shouldAllow {
				assert.NoError(t, err)
				assert.False(t, res.Valid)
				assert.Contains(t, res.Message, tt.expectedError)
			}
		})
	}
}
