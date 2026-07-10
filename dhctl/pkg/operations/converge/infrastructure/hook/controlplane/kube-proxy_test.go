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

package controlplane

import (
	"context"
	"strings"
	"testing"

	"github.com/deckhouse/lib-connection/pkg/settings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestKubeProxyCheckerIsReadyValidation(t *testing.T) {
	tests := []struct {
		name        string
		checker     *KubeProxyChecker
		expectedErr string
	}{
		{
			name:        "Kubernetes init params are not configured",
			checker:     NewKubeProxyChecker(),
			expectedErr: "kube proxy checker: Kubernetes init params are not configured",
		},
		{
			name: "base provider settings are not configured",
			checker: NewKubeProxyChecker().
				WithInitParams(&client.KubernetesInitParams{}),
			expectedErr: "kube proxy checker: base provider settings are not configured",
		},
		{
			name: "SSH provider is not configured",
			checker: NewKubeProxyChecker().
				WithInitParams(&client.KubernetesInitParams{}).
				WithSSHProvider(nil, &settings.BaseProviders{}),
			expectedErr: "kube proxy checker: SSH provider is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.checker.IsReady(context.Background(), "master-0")
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Fatalf(
					"expected error containing %q, got %q",
					tt.expectedErr,
					err.Error(),
				)
			}
		})
	}
}
