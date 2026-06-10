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
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

func TestBuildStateFromProtocolInput(t *testing.T) {
	t.Parallel()

	state, err := BuildStateFromProtocolInput("cloud-provider-dvp", proto.PrepareInput{
		ModuleConfig: map[string]any{
			"provider": map[string]any{
				"parameters": map[string]any{
					"namespace": "d8-cloud-provider-dvp",
				},
			},
		},
		ProviderClusterConfig: map[string]any{
			"masterNodeGroup": map[string]any{"replicas": 3},
		},
	}, &proto.CloudProviderVars{
		Secrets: map[string]map[string]any{
			"d8-credentials": {
				"metadata": map[string]any{"name": "d8-credentials"},
				"type":     cpapi.CredentialsSecretType,
				"stringData": map[string]any{
					"authScheme": "kubeconfig",
					"secret":     "token",
				},
			},
		},
		NodeGroups: map[string]map[string]any{
			"master": {
				"metadata": map[string]any{"name": "master"},
				"spec": map[string]any{
					"nodeType": "CloudPermanent",
				},
			},
		},
		InstanceClasses: map[string]map[string]any{
			"master-dvp": {
				"metadata": map[string]any{"name": "master-dvp"},
				"kind":     "DVPInstanceClass",
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildStateFromProtocolInput() error = %v", err)
	}

	if state.ModuleConfig == nil || state.ModuleConfig.Name != "cloud-provider-dvp" {
		t.Fatalf("BuildStateFromProtocolInput() module config = %#v", state.ModuleConfig)
	}
	if len(state.CredentialSecrets) != 1 || state.CredentialSecrets[0].Name != cpapi.CredentialSecretName {
		t.Fatalf("BuildStateFromProtocolInput() credential secrets = %#v", state.CredentialSecrets)
	}
	if len(state.NodeGroups) != 1 || state.NodeGroups[0].Name != "master" {
		t.Fatalf("BuildStateFromProtocolInput() node groups = %#v", state.NodeGroups)
	}
	if len(state.InstanceClasses) != 1 || state.InstanceClasses[0].Name != "master-dvp" {
		t.Fatalf("BuildStateFromProtocolInput() instance classes = %#v", state.InstanceClasses)
	}
	if len(state.LegacyProviderClusterConfig) == 0 {
		t.Fatal("BuildStateFromProtocolInput() legacy PCC not populated")
	}
}

func TestDecodeCredentialSecretsNilVars(t *testing.T) {
	t.Parallel()

	secrets, err := DecodeCredentialSecrets(nil)
	if err != nil {
		t.Fatalf("DecodeCredentialSecrets(nil) error = %v", err)
	}
	if len(secrets) != 0 {
		t.Fatalf("DecodeCredentialSecrets(nil) = %#v, want empty slice", secrets)
	}
}

func TestDecodeNodeGroupsNilVars(t *testing.T) {
	t.Parallel()

	nodeGroups, err := DecodeNodeGroups(nil)
	if err != nil || len(nodeGroups) != 0 {
		t.Fatalf("DecodeNodeGroups(nil) = %#v, err = %v", nodeGroups, err)
	}
}

func TestDecodeInstanceClassesNilVars(t *testing.T) {
	t.Parallel()

	classes, err := DecodeInstanceClasses(nil)
	if err != nil || len(classes) != 0 {
		t.Fatalf("DecodeInstanceClasses(nil) = %#v, err = %v", classes, err)
	}
}

func TestDecodeModuleConfigEmptyRaw(t *testing.T) {
	t.Parallel()

	cfg, err := DecodeModuleConfig("cloud-provider-test", nil)
	if err != nil || cfg != nil {
		t.Fatalf("DecodeModuleConfig(nil) = %#v, err = %v", cfg, err)
	}
}

func TestDecodeModuleConfigFullObject(t *testing.T) {
	t.Parallel()

	cfg, err := DecodeModuleConfig("ignored", map[string]any{
		"metadata": map[string]any{"name": "cloud-provider-dvp"},
		"spec": map[string]any{
			"enabled": true,
			"version": 2,
		},
	})
	if err != nil {
		t.Fatalf("DecodeModuleConfig() error = %v", err)
	}
	if cfg.Name != "cloud-provider-dvp" {
		t.Fatalf("DecodeModuleConfig() name = %q", cfg.Name)
	}
}

func TestDecodeModuleConfigFillsMissingName(t *testing.T) {
	t.Parallel()

	cfg, err := DecodeModuleConfig("cloud-provider-dvp", map[string]any{
		"spec": map[string]any{"enabled": true},
	})
	if err != nil {
		t.Fatalf("DecodeModuleConfig() error = %v", err)
	}
	if cfg.Name != "cloud-provider-dvp" {
		t.Fatalf("DecodeModuleConfig() name = %q, want module name fallback", cfg.Name)
	}
}

func TestDecodeCredentialSecretsInvalidPayload(t *testing.T) {
	t.Parallel()

	_, err := DecodeCredentialSecrets(&proto.CloudProviderVars{
		Secrets: map[string]map[string]any{
			"broken": {"metadata": "not-an-object"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "decode secret") {
		t.Fatalf("DecodeCredentialSecrets() error = %v, want decode secret failure", err)
	}
}

func TestDecodeJSONValueRoundTrip(t *testing.T) {
	t.Parallel()

	value, err := DecodeJSONValue[cpapi.NodeGroup](map[string]any{
		"metadata": map[string]any{"name": "master"},
		"spec":     map[string]any{"nodeType": "CloudPermanent"},
	})
	if err != nil {
		t.Fatalf("DecodeJSONValue() error = %v", err)
	}
	if value.Name != "master" || value.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
		t.Fatalf("DecodeJSONValue() = %#v", value)
	}
}

func TestDecodeJSONValueInvalidTarget(t *testing.T) {
	t.Parallel()

	_, err := DecodeJSONValue[int]("not-a-number")
	if err == nil || !strings.Contains(err.Error(), "unmarshal value") {
		t.Fatalf("DecodeJSONValue() error = %v, want unmarshal failure", err)
	}
}

func TestBuildStateFromProtocolInputEmptyInput(t *testing.T) {
	t.Parallel()

	state, err := BuildStateFromProtocolInput("cloud-provider-dvp", proto.PrepareInput{}, nil)
	if err != nil {
		t.Fatalf("BuildStateFromProtocolInput() error = %v", err)
	}
	if state.ModuleConfig != nil || len(state.CredentialSecrets) != 0 || len(state.NodeGroups) != 0 || len(state.InstanceClasses) != 0 {
		t.Fatalf("BuildStateFromProtocolInput() = %#v, want empty state", state)
	}
}

func TestBuildStateFromProtocolInputModuleConfigDecodeError(t *testing.T) {
	t.Parallel()

	_, err := BuildStateFromProtocolInput("cloud-provider-dvp", proto.PrepareInput{
		ModuleConfig: map[string]any{"spec": "invalid"},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "decode ModuleConfig object") {
		t.Fatalf("BuildStateFromProtocolInput() error = %v", err)
	}
}

func TestBuildStateFromProtocolInputCredentialSecretsDecodeError(t *testing.T) {
	t.Parallel()

	_, err := BuildStateFromProtocolInput("cloud-provider-dvp", proto.PrepareInput{}, &proto.CloudProviderVars{
		Secrets: map[string]map[string]any{
			"broken": {"metadata": "invalid"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "decode secret") {
		t.Fatalf("BuildStateFromProtocolInput() error = %v", err)
	}
}

func TestBuildStateFromProtocolInputNodeGroupsDecodeError(t *testing.T) {
	t.Parallel()

	_, err := BuildStateFromProtocolInput("cloud-provider-dvp", proto.PrepareInput{}, &proto.CloudProviderVars{
		NodeGroups: map[string]map[string]any{
			"broken": {"spec": "invalid"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "decode node group") {
		t.Fatalf("BuildStateFromProtocolInput() error = %v", err)
	}
}

func TestBuildStateFromProtocolInputInstanceClassesDecodeError(t *testing.T) {
	t.Parallel()

	_, err := BuildStateFromProtocolInput("cloud-provider-dvp", proto.PrepareInput{}, &proto.CloudProviderVars{
		InstanceClasses: map[string]map[string]any{
			"broken": {"metadata": 123},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "decode instance class") {
		t.Fatalf("BuildStateFromProtocolInput() error = %v", err)
	}
}

func TestBuildStateFromProtocolInputModuleConfigSettingsDecodeError(t *testing.T) {
	t.Parallel()

	_, err := BuildStateFromProtocolInput("cloud-provider-dvp", proto.PrepareInput{
		ModuleConfig: map[string]any{"provider": "invalid"},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "decode module settings") {
		t.Fatalf("BuildStateFromProtocolInput() error = %v", err)
	}
}
