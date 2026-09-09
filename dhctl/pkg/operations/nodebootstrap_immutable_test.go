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

// Only an immutable group boots from a document the installer renders. Every
// other group is created the way it always was, from the cloud config the
// cluster publishes for it.
func TestOnlyAnImmutableGroupBuildsItsOwnPayload(t *testing.T) {
	t.Parallel()

	build := func(context.Context, *client.KubernetesClient, string, string) (string, error) { return "", nil }

	cases := map[string]struct {
		group config.TerraNodeGroupSpec
		build ImmutablePayloadBuilder
		want  bool
	}{
		"an immutable group": {
			group: config.TerraNodeGroupSpec{Name: "front", SystemType: "Immutable"},
			build: build,
			want:  true,
		},
		"a group with no system type": {
			group: config.TerraNodeGroupSpec{Name: "worker"},
			build: build,
			want:  false,
		},
		"a group of another system type": {
			group: config.TerraNodeGroupSpec{Name: "worker", SystemType: "Classic"},
			build: build,
			want:  false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, payloadBuilderFor(tc.group, tc.build) != nil)
		})
	}
}

// The group decides on its own whether its machines boot from a document; nothing
// about the rest of the cluster - the master's kind least of all - may turn that off.
// A caller that brings no builder to an immutable group is broken, and it is
// better to stop it than to create machines that wait in the installer for good.
func TestAnImmutableGroupWithNoBuilderIsAProgrammerError(t *testing.T) {
	t.Parallel()

	front := config.TerraNodeGroupSpec{Name: "front", SystemType: "Immutable"}
	require.PanicsWithValue(t, `no payload builder for the immutable node group "front"`, func() {
		payloadBuilderFor(front, nil)
	})
}

// An immutable group never asks the cluster for its cloud config: what the
// cluster publishes for the group is a bashible script its machines cannot run,
// and each of them boots from a document of its own instead.
func TestAnImmutableGroupAsksTheClusterForNoCloudConfig(t *testing.T) {
	t.Parallel()

	build := func(context.Context, *client.KubernetesClient, string, string) (string, error) { return "", nil }

	cloudConfig, err := groupCloudConfig(t.Context(), nil, "front", build)
	require.NoError(t, err, "an immutable group must not ask the cluster for a cloud config")
	require.Empty(t, cloudConfig)
}

// The document is per node - it names the node it is for - so the group's config
// must never stand in for it.
func TestEachImmutableMachineGetsItsOwnDocument(t *testing.T) {
	t.Parallel()

	build := func(_ context.Context, _ *client.KubernetesClient, ng, node string) (string, error) {
		return ng + "/" + node, nil
	}

	got, err := nodeCloudConfig(t.Context(), nil, build, "front", "front-0", "the group config")
	require.NoError(t, err)
	require.Equal(t, "front/front-0", got)

	classic, err := nodeCloudConfig(t.Context(), nil, nil, "worker", "worker-0", "the group config")
	require.NoError(t, err)
	require.Equal(t, "the group config", classic)
}

// Both node paths have to honour the builder. The parallel one is the path a
// CloudPermanent group actually takes, and it once accepted the argument and
// dropped it: the group was then created with no cloud config at all, so its
// machines waited in the installer for good.
func TestBothNodePathsHonourTheBuilder(t *testing.T) {
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
			require.Truef(t, namesIdentifier(fn.Body, "build"),
				"%s takes a payload builder and never calls it: its machines are created with no document at all", fn.Name.Name)
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
