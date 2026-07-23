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

package validation

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestValidateProviderClusterConfigKubeconfigNilState(t *testing.T) {
	t.Parallel()

	result := ValidateProviderClusterConfigKubeconfig(nil, "provider.kubeconfigDataBase64")
	if !hasViolationCode(result, CodeInternalStateNil) {
		t.Fatalf("ValidateProviderClusterConfigKubeconfig(nil) = %q, want %s", result.Error(), CodeInternalStateNil)
	}
}

func TestValidateProviderClusterConfigKubeconfigSkipsMissingPath(t *testing.T) {
	t.Parallel()

	state := &State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{
				"namespace": "d8-cloud-provider-dvp",
			},
		},
	}

	if result := ValidateProviderClusterConfigKubeconfig(state, "provider.kubeconfigDataBase64"); result.HasErrors() {
		t.Fatalf("ValidateProviderClusterConfigKubeconfig() = %q, want no errors", result.Error())
	}
}

func TestValidateProviderClusterConfigKubeconfigRejectsEmptyValue(t *testing.T) {
	t.Parallel()

	state := &State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{
				"kubeconfigDataBase64": "   ",
			},
		},
	}

	result := ValidateProviderClusterConfigKubeconfig(state, "provider.kubeconfigDataBase64")
	if !result.HasErrors() || !strings.Contains(result.Error(), "must be set") {
		t.Fatalf("ValidateProviderClusterConfigKubeconfig() = %q", result.Error())
	}
}

func TestValidateProviderClusterConfigKubeconfigRejectsInvalidBase64(t *testing.T) {
	t.Parallel()

	state := &State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{
				"kubeconfigDataBase64": "%%%",
			},
		},
	}

	result := ValidateProviderClusterConfigKubeconfig(state, "provider.kubeconfigDataBase64")
	if !result.HasErrors() || !strings.Contains(result.Error(), "base64-encoded kubeconfig") {
		t.Fatalf("ValidateProviderClusterConfigKubeconfig() = %q", result.Error())
	}
}

func TestValidateProviderClusterConfigKubeconfigRejectsInvalidYAML(t *testing.T) {
	t.Parallel()

	state := &State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{
				"kubeconfigDataBase64": base64.StdEncoding.EncodeToString([]byte("not-a-kubeconfig")),
			},
		},
	}

	result := ValidateProviderClusterConfigKubeconfig(state, "provider.kubeconfigDataBase64")
	if !result.HasErrors() || !strings.Contains(result.Error(), "base64-encoded kubeconfig") {
		t.Fatalf("ValidateProviderClusterConfigKubeconfig() = %q", result.Error())
	}
}

func TestValidateProviderClusterConfigKubeconfigAcceptsValidKubeconfig(t *testing.T) {
	t.Parallel()

	state := &State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{
				"kubeconfigDataBase64": validTestKubeconfigB64(),
			},
		},
	}

	if result := ValidateProviderClusterConfigKubeconfig(state, "provider.kubeconfigDataBase64"); result.HasErrors() {
		t.Fatalf("ValidateProviderClusterConfigKubeconfig() = %q, want no errors", result.Error())
	}
}

func TestLookupMapStringPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"provider": map[string]any{
			"kubeconfigDataBase64": "value",
		},
	}

	if got, ok := lookupMapStringPath(data, "provider.kubeconfigDataBase64"); !ok || got != "value" {
		t.Fatalf("lookupMapStringPath() = (%q, %v), want (value, true)", got, ok)
	}
	if _, ok := lookupMapStringPath(data, "provider.missing"); ok {
		t.Fatal("lookupMapStringPath() missing path = true, want false")
	}
}
