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

	cpvalprotocol "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/protocol"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
)

func TestStateAliasUsesProviderTypes(t *testing.T) {
	t.Parallel()

	class := &ycicv1.YandexInstanceClass{}
	class.Name = "master-yandex"

	state := &State{
		ModuleName:      ycmeta.ModuleName,
		NamespaceName:   ycmeta.Namespace,
		InstanceClasses: []*ycicv1.YandexInstanceClass{class},
	}

	found, ok := state.FindInstanceClass("master-yandex")
	if !ok || found.GetName() != "master-yandex" {
		t.Fatalf("FindInstanceClass() = (%v, %v), want the stored class", found, ok)
	}
	if state.HasProviderClusterConfig() {
		t.Fatal("HasProviderClusterConfig() = true, want false")
	}
}

func TestNewProtocolStateBuilderFactoryBuildsAliasedState(t *testing.T) {
	t.Parallel()

	factory := NewProtocolStateBuilderFactory(cpvalprotocol.StateBuilderConfig{
		NamespaceName: ycmeta.Namespace,
		ModuleName:    ycmeta.ModuleName,
	})

	state, err := factory.CreateBuilder().Build(proto.ValidateInput{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if state.ModuleName != ycmeta.ModuleName {
		t.Fatalf("state.ModuleName = %q, want %s", state.ModuleName, ycmeta.ModuleName)
	}
}
