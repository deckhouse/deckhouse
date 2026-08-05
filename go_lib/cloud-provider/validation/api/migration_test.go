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

package api

import (
	"testing"

	"k8s.io/utils/ptr"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/internal/testprovider"
)

func TestMigrationStatusFromState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state *testState
		want  cpapi.MigrationStatus
	}{
		{
			name:  "state A - no PCC",
			state: migrationBaseState(),
			want:  cpapi.MigrationStatus{},
		},
		{
			name: "state B - PCC with incomplete new resources",
			state: &testState{
				ProviderClusterConfig: &testprovider.ProviderClusterConfig{
					MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
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
			state: newCompleteMigrationState(),
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

func migrationBaseState() *testState {
	const (
		moduleName        = "cloud-provider-test"
		namespaceName     = "d8-cloud-provider-test"
		instanceClassKind = "TestInstanceClass"
	)

	state := &testState{
		ModuleName:    moduleName,
		NamespaceName: namespaceName,
		ModuleConfig: &cpapi.ModuleConfig[*testprovider.Settings]{
			ObjectMeta: cpapi.ObjectMeta{Name: moduleName},
			Spec: cpapi.ModuleConfigSpec[*testprovider.Settings]{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: &testprovider.Settings{
					Provider: testprovider.Section{Parameters: map[string]string{"namespace": namespaceName}},
					Storage:  testprovider.Section{Disabled: false, Parameters: map[string]string{}},
					Nodes:    testprovider.Section{Disabled: true},
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
							Name: cpapi.BuildInstanceClassName("master"),
						},
					},
				},
			},
		},
		InstanceClasses: []*testprovider.InstanceClass{
			testInstanceClass(cpapi.BuildInstanceClassName("master")),
		},
	}
	return state
}

func newCompleteMigrationState() *testState {
	state := migrationBaseState()
	state.ProviderClusterConfig = &testprovider.ProviderClusterConfig{
		MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
		NodeGroups:      []testprovider.NodeGroup{{Name: "worker", Replicas: 1}},
	}
	state.NodeGroups = append(state.NodeGroups, cpapi.NodeGroup{
		ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
		Spec: cpapi.NodeGroupSpec{
			NodeType: cpapi.NodeTypeCloudPermanent,
		},
	})
	state.InstanceClasses = append(state.InstanceClasses, testInstanceClass(cpapi.BuildInstanceClassName("worker")))

	return state
}

func TestIsNewResourcesCompleteRequiresProviderSection(t *testing.T) {
	t.Parallel()
	state := newCompleteMigrationState()
	state.ModuleConfig.Spec.Settings = &testprovider.Settings{}
	if IsNewResourcesComplete(state) {
		t.Fatal("IsNewResourcesComplete() = true, want false without provider section")
	}
}

func TestMigrationStatusIncompleteWhenModuleConfigMissing(t *testing.T) {
	t.Parallel()

	got := MigrationStatusFromState(&testState{
		ProviderClusterConfig: &testprovider.ProviderClusterConfig{
			MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
		},
	})
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenModuleDisabled(t *testing.T) {
	t.Parallel()

	state := migrationBaseState()
	state.ProviderClusterConfig = &testprovider.ProviderClusterConfig{
		MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
	}
	disabled := false
	state.ModuleConfig.Spec.Enabled = &disabled

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenModuleVersionTooLow(t *testing.T) {
	t.Parallel()

	state := migrationBaseState()
	state.ProviderClusterConfig = &testprovider.ProviderClusterConfig{
		MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
	}
	state.ModuleConfig.Spec.Version = 1

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenProviderSettingsMissing(t *testing.T) {
	t.Parallel()

	state := migrationBaseState()
	state.ProviderClusterConfig = &testprovider.ProviderClusterConfig{
		MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
	}
	state.ModuleConfig.Spec.Settings = &testprovider.Settings{
		Storage: testprovider.Section{Disabled: false},
	}

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenCredentialSecretMissing(t *testing.T) {
	t.Parallel()

	state := migrationBaseState()
	state.ProviderClusterConfig = &testprovider.ProviderClusterConfig{
		MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
	}
	state.CredentialSecrets = nil

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenMasterNodeGroupMissing(t *testing.T) {
	t.Parallel()

	state := migrationBaseState()
	state.ProviderClusterConfig = &testprovider.ProviderClusterConfig{
		MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
	}
	state.NodeGroups = nil

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenMasterInstanceClassMissing(t *testing.T) {
	t.Parallel()

	state := migrationBaseState()
	state.ProviderClusterConfig = &testprovider.ProviderClusterConfig{
		MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
	}
	state.InstanceClasses = nil

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenLegacyWorkerNameMissing(t *testing.T) {
	t.Parallel()

	state := newCompleteMigrationState()
	state.ProviderClusterConfig = &testprovider.ProviderClusterConfig{
		MasterNodeGroup: &testprovider.MasterNodeGroup{Replicas: 3},
		NodeGroups:      []testprovider.NodeGroup{{Name: ""}},
	}

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}

func TestMigrationStatusIncompleteWhenWorkerInstanceClassMissing(t *testing.T) {
	t.Parallel()

	state := newCompleteMigrationState()
	state.InstanceClasses = state.InstanceClasses[:1]

	got := MigrationStatusFromState(state)
	if !got.MigrationPending {
		t.Fatalf("MigrationStatusFromState() = %#v, want pending migration", got)
	}
}
