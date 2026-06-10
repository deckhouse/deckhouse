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
	"encoding/json"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestMigrationStatusFromState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state *cpval.State
		want  cpapi.MigrationStatus
	}{
		{
			name:  "state A - no PCC",
			state: migrationBaseState(t),
			want:  cpapi.MigrationStatus{},
		},
		{
			name: "state B - PCC with incomplete new resources",
			state: &cpval.State{
				LegacyProviderClusterConfig: map[string]any{
					"masterNodeGroup": map[string]any{"replicas": 3},
				},
			},
			want: cpapi.MigrationStatus{
				LegacyPCCPresent:     true,
				NewResourcesComplete: false,
				MigrationPending:     true,
			},
		},
		{
			name:  "state C - PCC with complete new resources",
			state: migrationCompleteState(t),
			want: cpapi.MigrationStatus{
				LegacyPCCPresent:     true,
				NewResourcesComplete: true,
				MigrationPending:     false,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := MigrationStatusFromState(tt.state)
			if got != tt.want {
				t.Fatalf("MigrationStatusFromState() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func migrationBaseState(t *testing.T) *cpval.State {
	t.Helper()

	state := &cpval.State{
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{Name: ModuleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: cpapi.ModuleConfigSpecSettings{
					Storage: &cpapi.ModuleConfigSpecSubsystemSettings{
						Enabled:    ptr.To(true),
						Parameters: map[string]any{},
					},
					Nodes: &cpapi.ModuleConfigSpecSubsystemSettings{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cpapi.CredentialSecretName,
					Namespace: Namespace,
				},
				Type: cpapi.CredentialsSecretType,
			},
		},
		NodeGroups: []cpapi.NodeGroup{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: InstanceClassKind,
							Name: "master-dvp",
						},
					},
				},
			},
		},
		InstanceClasses: []cpapi.InstanceClass{
			{
				TypeMeta:   metav1.TypeMeta{Kind: InstanceClassKind},
				ObjectMeta: metav1.ObjectMeta{Name: "master-dvp"},
				Spec: cpapi.InstanceClassSpec{
					EtcdDisk: migrationRawJSONForTest("{}"),
				},
			},
		},
	}
	state.ModuleConfig.Spec.SetRawSettings(map[string]any{
		"provider": map[string]any{
			"parameters": map[string]any{
				"namespace": Namespace,
			},
		},
		"storage": map[string]any{
			"enabled":    true,
			"parameters": map[string]any{},
		},
		"nodes": map[string]any{
			"enabled": false,
		},
	})

	return state
}

func migrationCompleteState(t *testing.T) *cpval.State {
	t.Helper()

	state := migrationBaseState(t)
	state.LegacyProviderClusterConfig = map[string]any{
		"masterNodeGroup": map[string]any{"replicas": 3},
		"nodeGroups": []any{
			map[string]any{"name": "worker"},
		},
	}
	state.NodeGroups = append(state.NodeGroups, cpapi.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: cpapi.NodeGroupSpec{
			NodeType: cpapi.NodeTypeCloudPermanent,
		},
	})
	state.InstanceClasses = append(state.InstanceClasses, cpapi.InstanceClass{
		TypeMeta:   metav1.TypeMeta{Kind: InstanceClassKind},
		ObjectMeta: metav1.ObjectMeta{Name: "worker-dvp"},
	})

	return state
}

func migrationRawJSONForTest(value string) *json.RawMessage {
	message := json.RawMessage(value)
	return &message
}

func TestMigrationStatusIncompleteWhenModuleConfigMissing(t *testing.T) {
	t.Parallel()

	got := MigrationStatusFromState(&cpval.State{
		LegacyProviderClusterConfig: map[string]any{"masterNodeGroup": map[string]any{}},
	})
	if !got.MigrationPending || got.NewResourcesComplete {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenModuleDisabled(t *testing.T) {
	t.Parallel()

	state := migrationBaseState(t)
	state.LegacyProviderClusterConfig = map[string]any{"masterNodeGroup": map[string]any{}}
	disabled := false
	state.ModuleConfig.Spec.Enabled = &disabled

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenModuleVersionTooLow(t *testing.T) {
	t.Parallel()

	state := migrationBaseState(t)
	state.LegacyProviderClusterConfig = map[string]any{"masterNodeGroup": map[string]any{}}
	state.ModuleConfig.Spec.Version = 1

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenProviderSettingsMissing(t *testing.T) {
	t.Parallel()

	state := migrationBaseState(t)
	state.LegacyProviderClusterConfig = map[string]any{"masterNodeGroup": map[string]any{}}
	state.ModuleConfig.Spec.SetRawSettings(map[string]any{"storage": map[string]any{"enabled": true}})

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenCredentialSecretMissing(t *testing.T) {
	t.Parallel()

	state := migrationBaseState(t)
	state.LegacyProviderClusterConfig = map[string]any{"masterNodeGroup": map[string]any{}}
	state.CredentialSecrets = nil

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenLegacyPCCDecodeFails(t *testing.T) {
	t.Parallel()

	state := migrationBaseState(t)
	state.LegacyProviderClusterConfig = map[string]any{"nodeGroups": "invalid"}

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenMasterNodeGroupMissing(t *testing.T) {
	t.Parallel()

	state := migrationBaseState(t)
	state.LegacyProviderClusterConfig = map[string]any{"masterNodeGroup": map[string]any{"replicas": 3}}
	state.NodeGroups = nil

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenMasterInstanceClassMissing(t *testing.T) {
	t.Parallel()

	state := migrationBaseState(t)
	state.LegacyProviderClusterConfig = map[string]any{"masterNodeGroup": map[string]any{"replicas": 3}}
	state.InstanceClasses = nil

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenLegacyWorkerNameMissing(t *testing.T) {
	t.Parallel()

	state := migrationCompleteState(t)
	state.LegacyProviderClusterConfig = map[string]any{
		"masterNodeGroup": map[string]any{"replicas": 3},
		"nodeGroups":      []any{map[string]any{"name": ""}},
	}

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenWorkerInstanceClassMissing(t *testing.T) {
	t.Parallel()

	state := migrationCompleteState(t)
	state.InstanceClasses = state.InstanceClasses[:1]

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}
