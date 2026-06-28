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
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"k8s.io/utils/ptr"
)

func TestMigrationStatusFromState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state *State
		want  cpapi.MigrationStatus
	}{
		{
			name:  "state A - no PCC",
			state: migrationBaseState(t),
			want:  cpapi.MigrationStatus{},
		},
		{
			name: "state B - PCC with incomplete new resources",
			state: &State{
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

func migrationBaseState(t *testing.T) *State {
	t.Helper()

	const (
		moduleName        = "cloud-provider-test"
		namespaceName     = "d8-cloud-provider-test"
		instanceClassKind = "TestInstanceClass"
	)

	state := &State{
		ModuleName:        moduleName,
		NamespaceName:     namespaceName,
		InstanceClassKind: instanceClassKind,
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: cpapi.ObjectMeta{Name: moduleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: cpapi.ModuleConfigSpecSettings{
					Provider: &cpapi.ModuleConfigSpecProviderSettings{
						Parameters: map[string]any{
							"namespace": namespaceName,
						},
					},
					Storage: &cpapi.ModuleConfigSpecSubsystemSettings{
						Disabled:   ptr.To(false),
						Parameters: map[string]any{},
					},
					Nodes: &cpapi.ModuleConfigSpecSubsystemSettings{
						Disabled: ptr.To(true),
					},
				},
			},
		},
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{
					Name:      cpapi.CredentialSecretName,
					Namespace: namespaceName,
				},
				Type: cpapi.CredentialsSecretType,
			},
		},
		NodeGroups: []cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: instanceClassKind,
							Name: "master-fc613b4dfd67",
						},
					},
				},
			},
		},
		InstanceClasses: []cpapi.InstanceClass{
			{
				TypeMeta:   cpapi.TypeMeta{Kind: instanceClassKind},
				ObjectMeta: cpapi.ObjectMeta{Name: "master-fc613b4dfd67"},
				Spec: cpapi.InstanceClassSpec{
					EtcdDisk: map[string]any{},
				},
			},
		},
	}
	return state
}

func migrationCompleteState(t *testing.T) *State {
	t.Helper()

	state := migrationBaseState(t)
	state.LegacyProviderClusterConfig = map[string]any{
		"masterNodeGroup": map[string]any{"replicas": 3},
		"nodeGroups": []any{
			map[string]any{"name": "worker"},
		},
	}
	state.NodeGroups = append(state.NodeGroups, cpapi.NodeGroup{
		ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
		Spec: cpapi.NodeGroupSpec{
			NodeType: cpapi.NodeTypeCloudPermanent,
		},
	})
	state.InstanceClasses = append(state.InstanceClasses, cpapi.InstanceClass{
		TypeMeta:   cpapi.TypeMeta{Kind: state.InstanceClassKind},
		ObjectMeta: cpapi.ObjectMeta{Name: "worker-87eba76e7f31"},
	})

	return state
}

func TestMigrationStatusIncompleteWhenModuleConfigMissing(t *testing.T) {
	t.Parallel()

	got := MigrationStatusFromState(&State{
		LegacyProviderClusterConfig: map[string]any{"masterNodeGroup": map[string]any{}},
	})
	if !got.MigrationPending {
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
	state.ModuleConfig.Spec.Settings = cpapi.ModuleConfigSpecSettings{
		Storage: &cpapi.ModuleConfigSpecSubsystemSettings{Disabled: ptr.To(false)},
	}

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
