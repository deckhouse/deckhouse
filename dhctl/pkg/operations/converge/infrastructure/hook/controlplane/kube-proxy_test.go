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
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestKubeProxyCheckerIsReadyValidation(t *testing.T) {
	tests := []struct {
		name        string
		checker     *KubeProxyChecker
		expectedErr string
	}{
		{
			name:    "Kubernetes init params are not configured",
			checker: NewKubeProxyChecker(),
			expectedErr: "kube proxy checker: " +
				"Kubernetes init params are not configured",
		},
		{
			name: "base provider settings are not configured",
			checker: NewKubeProxyChecker().
				WithInitParams(&client.KubernetesInitParams{}),
			expectedErr: "kube proxy checker: " +
				"base provider settings are not configured",
		},
		{
			name: "SSH provider is not configured",
			checker: NewKubeProxyChecker().
				WithInitParams(&client.KubernetesInitParams{}).
				WithSSHProvider(nil, &settings.BaseProviders{}),
			expectedErr: "kube proxy checker: " +
				"SSH provider is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.checker.IsReady(
				context.Background(),
				"master-0",
			)
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

func TestCopyKubernetesInitParams(t *testing.T) {
	original := &client.KubernetesInitParams{
		KubeConfig:          "original-kubeconfig",
		KubeConfigContext:   "original-context",
		KubeConfigInCluster: true,
		RestConfig: &rest.Config{
			Host:        "https://original-api-server:6443",
			BearerToken: "original-token",
		},
	}

	copied := copyKubernetesInitParams(original)

	if copied == nil {
		t.Fatal("expected copied init params, got nil")
	}

	if copied == original {
		t.Fatal(
			"expected a separate KubernetesInitParams instance",
		)
	}

	if copied.RestConfig == original.RestConfig {
		t.Fatal("expected a separate RestConfig instance")
	}

	if copied.KubeConfig != original.KubeConfig {
		t.Fatalf(
			"expected copied KubeConfig %q, got %q",
			original.KubeConfig,
			copied.KubeConfig,
		)
	}

	if copied.KubeConfigContext != original.KubeConfigContext {
		t.Fatalf(
			"expected copied KubeConfigContext %q, got %q",
			original.KubeConfigContext,
			copied.KubeConfigContext,
		)
	}

	if copied.KubeConfigInCluster != original.KubeConfigInCluster {
		t.Fatalf(
			"expected copied KubeConfigInCluster %v, got %v",
			original.KubeConfigInCluster,
			copied.KubeConfigInCluster,
		)
	}

	copied.KubeConfig = "changed-kubeconfig"
	copied.KubeConfigContext = "changed-context"
	copied.KubeConfigInCluster = false
	copied.RestConfig.Host = "http://127.0.0.1:22322"
	copied.RestConfig.BearerToken = "changed-token"

	if original.KubeConfig != "original-kubeconfig" {
		t.Fatalf(
			"original KubeConfig was modified: got %q",
			original.KubeConfig,
		)
	}

	if original.KubeConfigContext != "original-context" {
		t.Fatalf(
			"original KubeConfigContext was modified: got %q",
			original.KubeConfigContext,
		)
	}

	if !original.KubeConfigInCluster {
		t.Fatal("original KubeConfigInCluster was modified")
	}

	if original.RestConfig.Host !=
		"https://original-api-server:6443" {
		t.Fatalf(
			"original RestConfig.Host was modified: got %q",
			original.RestConfig.Host,
		)
	}

	if original.RestConfig.BearerToken != "original-token" {
		t.Fatalf(
			"original RestConfig.BearerToken was modified: got %q",
			original.RestConfig.BearerToken,
		)
	}
}

func TestCopyKubernetesInitParamsWithoutRestConfig(t *testing.T) {
	original := &client.KubernetesInitParams{
		KubeConfig:        "original-kubeconfig",
		KubeConfigContext: "original-context",
	}

	copied := copyKubernetesInitParams(original)

	if copied == nil {
		t.Fatal("expected copied init params, got nil")
	}

	if copied == original {
		t.Fatal(
			"expected a separate KubernetesInitParams instance",
		)
	}

	if copied.RestConfig != nil {
		t.Fatalf(
			"expected nil RestConfig, got %#v",
			copied.RestConfig,
		)
	}
}

func TestCopyKubernetesInitParamsNil(t *testing.T) {
	copied := copyKubernetesInitParams(nil)
	if copied != nil {
		t.Fatalf("expected nil, got %#v", copied)
	}
}
