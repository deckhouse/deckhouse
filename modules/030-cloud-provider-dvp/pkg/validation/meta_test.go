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
	dvpicv1alpha1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/instanceclass/v1alpha1"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/meta"
)

func TestStateAliasUsesProviderTypes(t *testing.T) {
	t.Parallel()

	class := &dvpicv1alpha1.DVPInstanceClass{}
	class.Name = "master-dvp"

	state := &State{
		ModuleName:      dvpmeta.ModuleName,
		NamespaceName:   dvpmeta.Namespace,
		InstanceClasses: []*dvpicv1alpha1.DVPInstanceClass{class},
	}

	found, ok := state.FindInstanceClass("master-dvp")
	if !ok || found.GetName() != "master-dvp" {
		t.Fatalf("FindInstanceClass() = (%v, %v), want the stored class", found, ok)
	}
	if state.HasProviderClusterConfig() {
		t.Fatal("HasProviderClusterConfig() = true, want false")
	}
}

func TestNewProtocolStateBuilderFactoryBuildsAliasedState(t *testing.T) {
	t.Parallel()

	factory := NewProtocolStateBuilderFactory(cpvalprotocol.StateBuilderConfig{
		NamespaceName: dvpmeta.Namespace,
		ModuleName:    dvpmeta.ModuleName,
	})

	state, err := factory.CreateBuilder().Build(proto.ValidateInput{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if state.ModuleName != dvpmeta.ModuleName {
		t.Fatalf("state.ModuleName = %q, want %s", state.ModuleName, dvpmeta.ModuleName)
	}
}
