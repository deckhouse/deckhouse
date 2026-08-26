/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func bindingByName(t *testing.T, name string) (executesOnEvents bool) {
	t.Helper()

	for _, binding := range orderBootstrapTokenConfig.Kubernetes {
		if binding.Name != name {
			continue
		}
		require.NotNil(t, binding.ExecuteHookOnEvents, "binding %q leaves ExecuteHookOnEvents unset", name)
		return *binding.ExecuteHookOnEvents
	}

	t.Fatalf("no binding named %q", name)
	return false
}

// A NodeGroup that appears with no token of its own gets one only when this hook
// runs. Asleep on NodeGroup events, it runs when something unrelated re-renders
// the module or when the hourly schedule comes round - and a CloudEphemeral
// machine created meanwhile sits at WaitingForBootstrapScript until then. Measured
// on a live cluster: the NodeGroup and its machine appeared at 20:28:4x, the token
// at 20:33:11.
func TestANewNodeGroupWakesTheTokenHook(t *testing.T) {
	require.True(t, bindingByName(t, "ngs"),
		"the hook sleeps through a new NodeGroup: its machines wait for the hourly schedule to issue a token")
}

// The other half of the same decision: waking on events is only affordable while
// the filter ignores everything that churns. A NodeGroup's status is rewritten
// constantly, and shell-operator drops an event whose filter result is unchanged
// (resource_informer.go: "Do not fire Added or Modified if object is in cache and
// its checksum is equal") - so the filter must answer the same for both.
func TestTheNodeGroupFilterIgnoresStatusChurn(t *testing.T) {
	nodeGroup := func(readyNodes int64) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"metadata":   map[string]any{"name": "worker"},
			"spec":       map[string]any{"nodeType": "CloudEphemeral"},
			"status":     map[string]any{"ready": readyNodes},
		}}
	}

	before, err := bootstrapTokenFilterNodeGroup(nodeGroup(0))
	require.NoError(t, err)

	after, err := bootstrapTokenFilterNodeGroup(nodeGroup(3))
	require.NoError(t, err)

	require.Equal(t, before, after,
		"the filter reads the group's status, so every status write would re-run the hook and re-render the module")
}
