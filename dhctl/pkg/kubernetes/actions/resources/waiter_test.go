// Copyright 2022 Flant JSC
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

package resources

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func TestResourcesToCheckers(t *testing.T) {
	const resourcesContentWithoutNg = `
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
spec:
  cores: 4
  memory: 8192
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: admin@admin.yoyo
  accessLevel: SuperAdmin
  portForwarding: true
---
`

	t.Run("without nodegroup", func(t *testing.T) {
		resources, err := template.ParseResourcesContent(resourcesContentWithoutNg, nil)
		require.NoError(t, err)
		require.Len(t, resources, 2)

		checkers, err := GetCheckers(nil, resources, nil)
		require.NoError(t, err)
		require.Len(t, checkers, 2) // ready resources for all

		require.Equal(t, checkers[0].Name(), "Waiting for the resource deckhouse.io/v1, Kind=YandexInstanceClass 'system' to become ready.")
		require.Equal(t, checkers[1].Name(), "Waiting for the resource deckhouse.io/v1, Kind=ClusterAuthorizationRule 'admin' to become ready.")
	})

	t.Run("with cloud static nodegroup", func(t *testing.T) {
		const content = resourcesContentWithoutNg + `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: node
spec:
  nodeType: Static
`
		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources, nil)
		require.NoError(t, err)
		require.Len(t, checkers, 3, "only readiness checks")

		require.Equal(t, checkers[0].Name(), "Waiting for the resource deckhouse.io/v1, Kind=YandexInstanceClass 'system' to become ready.")
		require.Equal(t, checkers[1].Name(), "Waiting for the resource deckhouse.io/v1, Kind=ClusterAuthorizationRule 'admin' to become ready.")
		require.Equal(t, checkers[2].Name(), "Waiting for the resource deckhouse.io/v1, Kind=NodeGroup 'node' to become ready.")
	})

	t.Run("with cloud ephemeral nodegroup, but min and max per zone not set", func(t *testing.T) {
		const content = resourcesContentWithoutNg + `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: system
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: CloudEphemeral
`
		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources, nil)
		require.NoError(t, err)
		require.Len(t, checkers, 3, "only readiness checks")

		require.Equal(t, checkers[0].Name(), "Waiting for the resource deckhouse.io/v1, Kind=YandexInstanceClass 'system' to become ready.")
		require.Equal(t, checkers[1].Name(), "Waiting for the resource deckhouse.io/v1, Kind=ClusterAuthorizationRule 'admin' to become ready.")
		require.Equal(t, checkers[2].Name(), "Waiting for the resource deckhouse.io/v1, Kind=NodeGroup 'system' to become ready.")
	})

	ngTemplate := func(name string, min, max int) string {
		return fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: %s
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: system
    minPerZone: %d
    maxPerZone: %d
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: CloudEphemeral
---
`, name, min, max)
	}

	t.Run("with cloud ephemeral nodegroup, but min and max per zone is zero", func(t *testing.T) {
		content := resourcesContentWithoutNg + ngTemplate("system", 0, 0)

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources, nil)
		require.NoError(t, err)
		require.Len(t, checkers, 3, "only readiness checks")

		require.Equal(t, checkers[0].Name(), "Waiting for the resource deckhouse.io/v1, Kind=YandexInstanceClass 'system' to become ready.")
		require.Equal(t, checkers[1].Name(), "Waiting for the resource deckhouse.io/v1, Kind=ClusterAuthorizationRule 'admin' to become ready.")
		require.Equal(t, checkers[2].Name(), "Waiting for the resource deckhouse.io/v1, Kind=NodeGroup 'system' to become ready.")
	})

	t.Run("with cloud ephemeral nodegroup, but min = 0 and max not zero", func(t *testing.T) {
		content := resourcesContentWithoutNg + ngTemplate("system", 0, 2)

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources, nil)
		require.NoError(t, err)
		require.Len(t, checkers, 4, "readiness checks with cluster bootstrapped")

		require.Equal(t, checkers[0].Name(), "Waiting for the resource deckhouse.io/v1, Kind=YandexInstanceClass 'system' to become ready.")
		require.Equal(t, checkers[1].Name(), "Waiting for the resource deckhouse.io/v1, Kind=ClusterAuthorizationRule 'admin' to become ready.")
		require.Equal(t, checkers[2].Name(), "Waiting for the cluster to become bootstrapped.")
		require.Equal(t, checkers[3].Name(), "Waiting for the resource deckhouse.io/v1, Kind=NodeGroup 'system' to become ready.")
	})

	t.Run("with cloud ephemeral nodegroup, but min not zero and max not zero", func(t *testing.T) {
		content := resourcesContentWithoutNg + ngTemplate("system", 1, 2)

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources, nil)
		require.NoError(t, err)
		require.Len(t, checkers, 4, "readiness checks with cluster bootstrapped")

		require.Equal(t, checkers[0].Name(), "Waiting for the resource deckhouse.io/v1, Kind=YandexInstanceClass 'system' to become ready.")
		require.Equal(t, checkers[1].Name(), "Waiting for the resource deckhouse.io/v1, Kind=ClusterAuthorizationRule 'admin' to become ready.")
		require.Equal(t, checkers[2].Name(), "Waiting for the cluster to become bootstrapped.")
		require.Equal(t, checkers[3].Name(), "Waiting for the resource deckhouse.io/v1, Kind=NodeGroup 'system' to become ready.")
	})

	t.Run("with multiple cloud ephemeral nodegroup", func(t *testing.T) {
		content := resourcesContentWithoutNg +
			ngTemplate("system", 0, 2) +
			ngTemplate("node", 1, 2)

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 4)

		checkers, err := GetCheckers(nil, resources, nil)
		require.NoError(t, err)
		require.Len(t, checkers, 5, "readiness checks with only one cluster bootstrapped")

		require.Equal(t, checkers[0].Name(), "Waiting for the resource deckhouse.io/v1, Kind=YandexInstanceClass 'system' to become ready.")
		require.Equal(t, checkers[1].Name(), "Waiting for the resource deckhouse.io/v1, Kind=ClusterAuthorizationRule 'admin' to become ready.")
		require.Equal(t, checkers[2].Name(), "Waiting for the cluster to become bootstrapped.")
		require.Equal(t, checkers[3].Name(), "Waiting for the resource deckhouse.io/v1, Kind=NodeGroup 'system' to become ready.")
		require.Equal(t, checkers[4].Name(), "Waiting for the resource deckhouse.io/v1, Kind=NodeGroup 'node' to become ready.")
	})

	t.Run("with one terra node without replicas", func(t *testing.T) {
		content := resourcesContentWithoutNg

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 2)

		cnf := &config.MetaConfig{
			TerraNodeGroupSpecs: []config.TerraNodeGroupSpec{
				{Replicas: 0, Name: "terra"},
			},
		}

		checkers, err := GetCheckers(nil, resources, cnf)
		require.NoError(t, err)
		require.Len(t, checkers, 2) // ready resources for all

		require.Equal(t, checkers[0].Name(), "Waiting for the resource deckhouse.io/v1, Kind=YandexInstanceClass 'system' to become ready.")
		require.Equal(t, checkers[1].Name(), "Waiting for the resource deckhouse.io/v1, Kind=ClusterAuthorizationRule 'admin' to become ready.")
	})

	t.Run("with one terra node with replicas", func(t *testing.T) {
		cnf := &config.MetaConfig{
			TerraNodeGroupSpecs: []config.TerraNodeGroupSpec{
				{Replicas: 1, Name: "terra"},
			},
		}

		checkers, err := GetCheckers(nil, nil, cnf)
		require.NoError(t, err)
		require.Len(t, checkers, 1, "should get one check")

		require.Equal(t, checkers[0].Name(), "Waiting for the cluster to become bootstrapped.")
	})

	t.Run("with multiple terra node with replicas", func(t *testing.T) {
		cnf := &config.MetaConfig{
			TerraNodeGroupSpecs: []config.TerraNodeGroupSpec{
				{Replicas: 1, Name: "terra"},
				{Replicas: 1, Name: "terra-1"},
			},
		}

		checkers, err := GetCheckers(nil, nil, cnf)
		require.NoError(t, err)
		require.Len(t, checkers, 1, "should get one check")

		require.Equal(t, checkers[0].Name(), "Waiting for the cluster to become bootstrapped.")
	})

	t.Run("with one terra node with replicas an ephemeral node group", func(t *testing.T) {
		content := resourcesContentWithoutNg + ngTemplate("system", 0, 2)

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		cnf := &config.MetaConfig{
			TerraNodeGroupSpecs: []config.TerraNodeGroupSpec{
				{Replicas: 1, Name: "terra"},
			},
		}

		checkers, err := GetCheckers(nil, resources, cnf)
		require.NoError(t, err)

		require.Len(t, checkers, 4, "should get one check")

		require.Equal(t, checkers[0].Name(), "Waiting for the cluster to become bootstrapped.")
		require.Equal(t, checkers[1].Name(), "Waiting for the resource deckhouse.io/v1, Kind=YandexInstanceClass 'system' to become ready.")
		require.Equal(t, checkers[2].Name(), "Waiting for the resource deckhouse.io/v1, Kind=ClusterAuthorizationRule 'admin' to become ready.")
		require.Equal(t, checkers[3].Name(), "Waiting for the resource deckhouse.io/v1, Kind=NodeGroup 'system' to become ready.")
	})
}

type testChecker struct {
	returns bool
	single  bool
	err     error
	name    string
}

func newTestChecker(returns bool, err error, single bool, name string) *testChecker {
	return &testChecker{
		returns: returns,
		err:     err,
		single:  single,
		name:    name,
	}
}

func (n *testChecker) IsReady(_ context.Context) (bool, error) {
	return n.returns, n.err
}

func (n *testChecker) Name() string {
	return "Test checker"
}

func (n *testChecker) Single() bool {
	return n.single
}

func TestWaiterStep(t *testing.T) {
	ctx := context.Background()

	t.Run("without checks", func(t *testing.T) {
		w := NewWaiter(make([]Checker, 0))
		ready, err := w.ReadyAll(ctx)

		require.NoError(t, err)
		require.True(t, ready, "should ready")
	})

	t.Run("with one ready check", func(t *testing.T) {
		w := NewWaiter([]Checker{newTestChecker(true, nil, false, "Test 1")})
		ready, err := w.ReadyAll(ctx)

		require.NoError(t, err)
		require.True(t, ready, "should ready")
	})

	t.Run("with multiple ready checks", func(t *testing.T) {
		w := NewWaiter([]Checker{
			newTestChecker(true, nil, false, "Test 1"),
			newTestChecker(true, nil, false, "Test 2"),
			newTestChecker(true, nil, false, "Test 3"),
		})
		ready, err := w.ReadyAll(ctx)

		require.NoError(t, err)
		require.True(t, ready, "should ready")
	})

	t.Run("with multiple ready and one error checks", func(t *testing.T) {
		w := NewWaiter([]Checker{
			newTestChecker(true, nil, false, "Test 1"),
			newTestChecker(false, fmt.Errorf("error"), false, "Test 2"),
			newTestChecker(true, nil, false, "Test 3"),
		})
		ready, err := w.ReadyAll(ctx)

		require.Error(t, err, "should error")
		require.False(t, ready)
	})

	t.Run("with multiple ready and one not ready checks", func(t *testing.T) {
		w := NewWaiter([]Checker{
			newTestChecker(true, nil, false, "Test 1"),
			newTestChecker(false, nil, false, "Test 2"),
			newTestChecker(true, nil, false, "Test 3"),
		})
		ready, err := w.ReadyAll(ctx)

		require.NoError(t, err)
		require.False(t, ready, "should not ready")
	})

	t.Run("with multiple ready and one not ready checks", func(t *testing.T) {
		w := NewWaiter([]Checker{
			newTestChecker(true, nil, false, "Test 1"),
			newTestChecker(false, nil, false, "Test 2"),
			newTestChecker(true, nil, false, "Test 3"),
		})

		_, err := w.ReadyAll(ctx)

		require.NoError(t, err)
		require.Len(t, w.checkers, 1, "should remove ready checks")
	})
}
