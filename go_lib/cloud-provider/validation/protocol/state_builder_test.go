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

package protocol

import (
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

func testStateBuilderConfig() StateBuilderConfig {
	return StateBuilderConfig{
		ModuleName:        "cloud-provider-dvp",
		NamespaceName:     "d8-cloud-provider-dvp",
		InstanceClassKind: "DVPInstanceClass",
	}
}

func testProtocolModuleConfigCR(settings map[string]any) map[string]any {
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata": map[string]any{
			"name": "cloud-provider-dvp",
		},
		"spec": map[string]any{
			"enabled": true,
			"version": 2,
			"settings": settings,
		},
	}
}

func TestStateBuilderBuild(t *testing.T) {
	t.Parallel()

	state, err := NewStateBuilder(testStateBuilderConfig()).Build(proto.PrepareInput{
		ProviderClusterConfig: map[string]any{
			"masterNodeGroup": map[string]any{"replicas": 3},
		},
		Vars: &proto.CloudProviderVars{
			Settings: testProtocolModuleConfigCR(map[string]any{
				"provider": map[string]any{
					"parameters": map[string]any{
						"namespace": "d8-cloud-provider-dvp",
					},
				},
			}),
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
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if state.ModuleName != "cloud-provider-dvp" || state.NamespaceName != "d8-cloud-provider-dvp" {
		t.Fatalf("Build() provider context = %#v", state)
	}
	if state.ModuleConfig == nil || state.ModuleConfig.Name != "cloud-provider-dvp" {
		t.Fatalf("Build() module config = %#v", state.ModuleConfig)
	}
	if len(state.CredentialSecrets) != 1 || state.CredentialSecrets[0].Name != cpapi.CredentialSecretName {
		t.Fatalf("Build() credential secrets = %#v", state.CredentialSecrets)
	}
	if len(state.NodeGroups) != 1 || state.NodeGroups[0].Name != "master" {
		t.Fatalf("Build() node groups = %#v", state.NodeGroups)
	}
	if len(state.InstanceClasses) != 1 || state.InstanceClasses[0].Name != "master-dvp" {
		t.Fatalf("Build() instance classes = %#v", state.InstanceClasses)
	}
	if len(state.LegacyProviderClusterConfig) == 0 {
		t.Fatal("Build() legacy PCC not populated")
	}
}

func TestStateBuilderBuildDhctlSettingsMap(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	state, err := NewStateBuilder(cfg).Build(proto.PrepareInput{
		Vars: &proto.CloudProviderVars{
			Settings: map[string]any{
				"provider": map[string]any{
					"parameters": map[string]any{"namespace": cfg.NamespaceName},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if state.ModuleConfig == nil || state.ModuleConfig.Name != cfg.ModuleName || state.ModuleConfig.Spec.Settings.Provider == nil {
		t.Fatalf("Build() module config = %#v", state.ModuleConfig)
	}
	if state.MigrationStatus.MigrationPending {
		t.Fatalf("Build() migration status = %#v, want zero value without MigrationRules", state.MigrationStatus)
	}
}

func TestStateBuilderBuildWithMigrationRules(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	cfg.MigrationRules = &cpval.MigrationRules{
		InstanceClassName: func(nodeGroupName string) string {
			return nodeGroupName + "-dvp"
		},
	}

	state, err := NewStateBuilder(cfg).Build(proto.PrepareInput{
		ProviderClusterConfig: map[string]any{
			"masterNodeGroup": map[string]any{"replicas": 3},
		},
		Vars: &proto.CloudProviderVars{
			Settings: testProtocolModuleConfigCR(map[string]any{
				"provider": map[string]any{
					"parameters": map[string]any{"namespace": cfg.NamespaceName},
				},
			}),
			Secrets: map[string]map[string]any{
				cpapi.CredentialSecretName: {
					"metadata": map[string]any{"name": cpapi.CredentialSecretName, "namespace": cfg.NamespaceName},
					"type":     cpapi.CredentialsSecretType,
				},
			},
			NodeGroups: map[string]map[string]any{
				"master": {
					"metadata": map[string]any{"name": "master"},
					"spec":     map[string]any{"nodeType": "CloudPermanent"},
				},
			},
			InstanceClasses: map[string]map[string]any{
				"master-dvp": {
					"metadata": map[string]any{"name": "master-dvp"},
					"kind":     cfg.InstanceClassKind,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if state.MigrationStatus.MigrationPending {
		t.Fatalf("Build() migration status = %#v, want complete", state.MigrationStatus)
	}
}

func TestStateBuilderBuildEmptyInput(t *testing.T) {
	t.Parallel()

	state, err := NewStateBuilder(testStateBuilderConfig()).Build(proto.PrepareInput{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if state.ModuleConfig != nil || len(state.CredentialSecrets) != 0 || len(state.NodeGroups) != 0 || len(state.InstanceClasses) != 0 {
		t.Fatalf("Build() = %#v, want empty decoded resources", state)
	}
}

func TestStateBuilderBuildModuleConfigDecodeError(t *testing.T) {
	t.Parallel()

	_, err := NewStateBuilder(testStateBuilderConfig()).Build(proto.PrepareInput{
		Vars: &proto.CloudProviderVars{
			Settings: map[string]any{
				"metadata": map[string]any{"name": "cloud-provider-dvp"},
				"spec":     "invalid",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "decode ModuleConfig") {
		t.Fatalf("Build() error = %v", err)
	}
}

func TestStateBuilderBuildCredentialSecretsDecodeError(t *testing.T) {
	t.Parallel()

	_, err := NewStateBuilder(testStateBuilderConfig()).Build(proto.PrepareInput{
		Vars: &proto.CloudProviderVars{
			Secrets: map[string]map[string]any{
				"broken": {"metadata": "invalid"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "decode secret") {
		t.Fatalf("Build() error = %v", err)
	}
}

func TestStateBuilderBuildNodeGroupsDecodeError(t *testing.T) {
	t.Parallel()

	_, err := NewStateBuilder(testStateBuilderConfig()).Build(proto.PrepareInput{
		Vars: &proto.CloudProviderVars{
			NodeGroups: map[string]map[string]any{
				"broken": {"spec": "invalid"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "decode node group") {
		t.Fatalf("Build() error = %v", err)
	}
}

func TestStateBuilderBuildInstanceClassesDecodeError(t *testing.T) {
	t.Parallel()

	_, err := NewStateBuilder(testStateBuilderConfig()).Build(proto.PrepareInput{
		Vars: &proto.CloudProviderVars{
			InstanceClasses: map[string]map[string]any{
				"broken": {"metadata": 123},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "decode instance class") {
		t.Fatalf("Build() error = %v", err)
	}
}

func TestStateBuilderBuildModuleConfigMarshalError(t *testing.T) {
	t.Parallel()

	_, err := NewStateBuilder(testStateBuilderConfig()).Build(proto.PrepareInput{
		Vars: &proto.CloudProviderVars{
			Settings: map[string]any{"broken": func() {}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "marshal value") {
		t.Fatalf("Build() error = %v", err)
	}
}
