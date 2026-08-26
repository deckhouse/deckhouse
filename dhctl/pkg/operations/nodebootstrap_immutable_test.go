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

package operations

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// Only an immutable group is configured by the installer. Every other group is
// created the way it always was, and taking the new path for one of them would
// ask its layout for an address no provider publishes.
func TestOnlyAnImmutableGroupIsConfiguredByTheInstaller(t *testing.T) {
	t.Parallel()

	configure := func(context.Context, *client.KubernetesClient, string, string, string) error { return nil }

	cases := map[string]struct {
		group     config.TerraNodeGroupSpec
		configure ConfigureImmutableNode
		want      bool
	}{
		"an immutable group": {
			group:     config.TerraNodeGroupSpec{Name: "front", SystemType: "Immutable"},
			configure: configure,
			want:      true,
		},
		"a group with no system type": {
			group:     config.TerraNodeGroupSpec{Name: "worker"},
			configure: configure,
			want:      false,
		},
		"a group of another system type": {
			group:     config.TerraNodeGroupSpec{Name: "worker", SystemType: "Classic"},
			configure: configure,
			want:      false,
		},
		"an immutable group on a bootstrap that configures nothing": {
			group:     config.TerraNodeGroupSpec{Name: "front", SystemType: "Immutable"},
			configure: nil,
			want:      false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, configureFor(tc.group, tc.configure) != nil)
		})
	}
}

// A group the installer configures is created bare: its machines take a document
// pushed to them, and the cloud config published for the group is a bashible
// script an immutable machine cannot run.
func TestAConfiguredGroupIsCreatedWithNoCloudConfig(t *testing.T) {
	t.Parallel()

	configure := func(context.Context, *client.KubernetesClient, string, string, string) error { return nil }

	cloudConfig, err := groupCloudConfig(t.Context(), nil, "front", configure)
	require.NoError(t, err, "a configured group must not ask the cluster for a cloud config")
	require.Empty(t, cloudConfig)
}

// Both node paths have to honour the configurator. The parallel one is the path
// a CloudPermanent group actually takes, and it once accepted the argument and
// dropped it: the group was then created with no cloud config AND never handed a
// document, so its machines waited in the installer for good.
func TestBothNodePathsHonourTheConfigurator(t *testing.T) {
	t.Parallel()

	const file = "nodebootstrap.go"

	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	require.NoError(t, err)

	seen := map[string]bool{}
	ast.Inspect(parsed, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		switch fn.Name.Name {
		case "BootstrapAdditionalNode", "BootstrapAdditionalNodeForParallelRun":
			seen[fn.Name.Name] = true
			require.Truef(t, namesIdentifier(fn.Body, "configure"),
				"%s takes a configurator and never calls it: its group is created bare and nothing configures it", fn.Name.Name)
		}
		return true
	})

	require.Len(t, seen, 2, "both node paths must exist in %s", file)
}

// namesIdentifier reports whether n mentions the given identifier anywhere.
func namesIdentifier(n ast.Node, name string) bool {
	var found bool
	ast.Inspect(n, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && ident.Name == name {
			found = true
		}
		return !found
	})
	return found
}
