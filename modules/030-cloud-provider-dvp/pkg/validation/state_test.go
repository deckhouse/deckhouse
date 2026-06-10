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
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

func TestBuildStateFromProtocolInputSetsMigrationStatus(t *testing.T) {
	t.Parallel()

	state, err := BuildStateFromProtocolInput(proto.PrepareInput{
		ProviderClusterConfig: map[string]any{
			"masterNodeGroup": map[string]any{"replicas": 3},
		},
	}, &proto.CloudProviderVars{})
	if err != nil {
		t.Fatalf("BuildStateFromProtocolInput() error = %v", err)
	}

	if !state.MigrationStatus.LegacyPCCPresent || !state.MigrationStatus.MigrationPending {
		t.Fatalf("BuildStateFromProtocolInput() migration status = %#v", state.MigrationStatus)
	}
}

func TestBuildStateFromProtocolInputCompleteMigration(t *testing.T) {
	t.Parallel()

	state, err := BuildStateFromProtocolInput(proto.PrepareInput{
		ModuleConfig: map[string]any{
			"provider": map[string]any{
				"parameters": map[string]any{"namespace": Namespace},
			},
		},
		ProviderClusterConfig: map[string]any{
			"masterNodeGroup": map[string]any{"replicas": 3},
		},
	}, &proto.CloudProviderVars{
		Secrets: map[string]map[string]any{
			cpapi.CredentialSecretName: {
				"metadata": map[string]any{"name": cpapi.CredentialSecretName, "namespace": Namespace},
				"type":     cpapi.CredentialsSecretType,
			},
		},
		NodeGroups: map[string]map[string]any{
			"master": {
				"metadata": map[string]any{"name": "master"},
				"spec":     map[string]any{"nodeType": string(cpapi.NodeTypeCloudPermanent)},
			},
		},
		InstanceClasses: map[string]map[string]any{
			"master-dvp": {
				"metadata": map[string]any{"name": "master-dvp"},
				"kind":     InstanceClassKind,
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildStateFromProtocolInput() error = %v", err)
	}

	if state.MigrationStatus.MigrationPending {
		t.Fatalf("BuildStateFromProtocolInput() migration status = %#v, want complete migration", state.MigrationStatus)
	}
}
