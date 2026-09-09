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

package v1

import (
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func TestYandexProviderClusterConfigurationImplementsContract(t *testing.T) {
	t.Parallel()

	var _ cpapi.ProviderClusterConfigObject = (*YandexProviderClusterConfiguration)(nil)

	pcc := &YandexProviderClusterConfiguration{
		MasterNodeGroup: YandexMasterNodeGroup{Replicas: 3},
		NodeGroups: []YandexStaticNodeGroup{
			{Name: "worker", Replicas: 2},
			{Name: "system", Replicas: 1},
		},
	}

	if !pcc.HasMasterNodeGroup() {
		t.Fatal("HasMasterNodeGroup() = false, want true")
	}

	names := pcc.NodeGroupNames()
	if len(names) != 2 || names[0] != "worker" || names[1] != "system" {
		t.Fatalf("NodeGroupNames() = %v, want [worker system]", names)
	}
}

func TestYandexProviderClusterConfigurationContractAbsent(t *testing.T) {
	t.Parallel()

	empty := &YandexProviderClusterConfiguration{}
	if empty.HasMasterNodeGroup() {
		t.Fatal("HasMasterNodeGroup() on zero value = true, want false")
	}
	if empty.NodeGroupNames() != nil {
		t.Fatal("NodeGroupNames() on zero value must be nil")
	}

	var nilPCC *YandexProviderClusterConfiguration
	if nilPCC.HasMasterNodeGroup() {
		t.Fatal("HasMasterNodeGroup() on nil must be false")
	}
	if nilPCC.NodeGroupNames() != nil {
		t.Fatal("NodeGroupNames() on nil must be nil")
	}
}
