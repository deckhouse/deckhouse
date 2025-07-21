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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type mockModuleManager struct {
	enabledModules []string
}

func (m *mockModuleManager) GetEnabledModuleNames() []string {
	return m.enabledModules
}

func TestDeckhouseReleaseValidationHandler(t *testing.T) {
	tests := []struct {
		name           string
		release        *v1alpha1.DeckhouseRelease
		enabledModules []string
		shouldAllow    bool
		expectedError  string
	}{
		{
			name: "should allow when not approved",
			release: &v1alpha1.DeckhouseRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
				Spec: v1alpha1.DeckhouseReleaseSpec{
					Version: "v1.30.0",
				},
				Approved: false,
			},
			enabledModules: []string{},
			shouldAllow:    true,
		},
		{
			name: "should allow when approved and no requirements",
			release: &v1alpha1.DeckhouseRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
				Spec: v1alpha1.DeckhouseReleaseSpec{
					Version: "v1.30.0",
				},
				Approved: true,
			},
			enabledModules: []string{},
			shouldAllow:    true,
		},
		{
			name: "should reject when approved with requirements",
			release: &v1alpha1.DeckhouseRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release",
				},
				Spec: v1alpha1.DeckhouseReleaseSpec{
					Version: "v1.30.0",
					Requirements: map[string]string{
						"k8s": "1.29.0",
					},
				},
				Approved: true,
			},
			enabledModules: []string{},
			shouldAllow:    false,
			expectedError:  "Cannot approve DeckhouseRelease",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().Build()

			mm := &mockModuleManager{
				enabledModules: tt.enabledModules,
			}

			exts := extenders.NewExtendersStack(nil, "v1.30.0", log.NewLogger())

			logger := log.NewLogger()

			handler := deckhouseReleaseValidationHandler(client, mm, exts, logger)

			assert.NotNil(t, handler)
		})
	}
}
