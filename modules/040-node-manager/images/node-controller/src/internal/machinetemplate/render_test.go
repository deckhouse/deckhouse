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

package machinetemplate

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRenderContext() RenderContext {
	return RenderContext{
		InstanceClass: map[string]any{"flavorName": "m1.large", "rootDiskSize": float64(50)},
		Provider:      map[string]any{"instances": map[string]any{"sshKeyPairName": "deckhouse"}},
		Zone:          "ru-1a",
		NodeGroupName: "worker",
		ClusterUUID:   "cluster-uuid",
		PodSubnet:     "10.111.0.0/16",
	}
}

func renderTemplate(t *testing.T, body string) (map[string]any, error) {
	t.Helper()
	contract, err := ParseContract([]byte("version: v2\nrolloutFields: [flavorName]\ntemplate: |\n" + indent(body)))
	if err != nil {
		return nil, err
	}
	return Render(contract, testRenderContext())
}

func indent(body string) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestRenderExposesFiveRoots(t *testing.T) {
	obj, err := renderTemplate(t, `apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OpenStackMachineTemplate
spec:
  template:
    spec:
      flavor: {{ .instanceClass.flavorName }}
      sshKeyName: {{ .provider.instances.sshKeyPairName }}
      zone: {{ .zone }}
      nodeGroup: {{ .nodeGroup.name }}
      clusterUUID: {{ .cluster.uuid }}
      podSubnet: {{ .cluster.podSubnet }}`)
	require.NoError(t, err)

	spec := obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	assert.Equal(t, map[string]any{
		"flavor":      "m1.large",
		"sshKeyName":  "deckhouse",
		"zone":        "ru-1a",
		"nodeGroup":   "worker",
		"clusterUUID": "cluster-uuid",
		"podSubnet":   "10.111.0.0/16",
	}, spec)
}

// Under v1 a typo in a context path rendered "<no value>" into the object and reached the cloud.
func TestRenderFailsOnUnknownPath(t *testing.T) {
	_, err := renderTemplate(t, `apiVersion: v1
kind: X
spec:
  flavor: {{ .instanceClass.flavourName }}`)
	require.ErrorContains(t, err, "map has no entry for key \"flavourName\"")
}

func TestRenderRejectsMetadata(t *testing.T) {
	_, err := renderTemplate(t, `apiVersion: v1
kind: X
metadata:
  name: mine
spec: {}`)
	require.ErrorContains(t, err, "must not set metadata")
}

func TestRenderRequiresApiVersionKindSpec(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expError string
	}{
		{
			name:     "no apiVersion",
			body:     "kind: X\nspec: {}",
			expError: "no apiVersion",
		},
		{
			name:     "no kind",
			body:     "apiVersion: v1\nspec: {}",
			expError: "no kind",
		},
		{
			name:     "no spec",
			body:     "apiVersion: v1\nkind: X",
			expError: "no spec",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := renderTemplate(t, tc.body)
			require.ErrorContains(t, err, tc.expError)
		})
	}
}

// The rendered object is frozen for the life of its generation, so rendering twice must produce
// exactly the same thing — that is what makes "nothing changed" a decidable question.
func TestRenderIsDeterministic(t *testing.T) {
	body := `apiVersion: v1
kind: X
spec:
  tags:
  {{- range $k, $v := .instanceClass }}
  - {{ printf "%s=%v" $k $v | quote }}
  {{- end }}`

	first, err := renderTemplate(t, body)
	require.NoError(t, err)
	second, err := renderTemplate(t, body)
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestRenderSupportsRequiredAndFail(t *testing.T) {
	_, err := renderTemplate(t, `apiVersion: v1
kind: X
spec:
  image: {{ get .instanceClass "imageName" | required "no imageName in InstanceClass nor in provider configuration" }}`)
	require.ErrorContains(t, err, "no imageName in InstanceClass nor in provider configuration")

	_, err = renderTemplate(t, `apiVersion: v1
kind: X
spec:
  {{ fail "provider says no" }}`)
	require.ErrorContains(t, err, "provider says no")
}

// The YAML/JSON helpers are part of the contract (a template embeds a whole subtree with them), so
// they are exercised rather than assumed.
func TestRenderSupportsYamlAndJsonHelpers(t *testing.T) {
	obj, err := renderTemplate(t, `apiVersion: v1
kind: X
spec:
  fromYaml: {{ get (fromYaml (toYaml .instanceClass)) "flavorName" }}
  fromJson: {{ get (fromJson (toJson .instanceClass)) "flavorName" }}
  embedded:
    {{- toYaml .provider.instances | nindent 4 }}`)
	require.NoError(t, err)

	spec := obj["spec"].(map[string]any)
	assert.Equal(t, "m1.large", spec["fromYaml"])
	assert.Equal(t, "m1.large", spec["fromJson"])
	assert.Equal(t, map[string]any{"sshKeyPairName": "deckhouse"}, spec["embedded"])
}

func TestRenderReportsBrokenYamlAndJson(t *testing.T) {
	_, err := renderTemplate(t, `apiVersion: v1
kind: X
spec:
  broken: {{ fromJson "{oops" }}`)
	require.ErrorContains(t, err, "invalid character")

	_, err = renderTemplate(t, `apiVersion: v1
kind: X
spec:
  broken: {{ fromYaml "a: [1" }}`)
	require.ErrorContains(t, err, "error converting YAML to JSON")
}

// SandboxFuncNames is pinned to a golden file so that a sprig upgrade adding a function is a
// review decision ("is it deterministic?") rather than a silent widening of the contract.
func TestSandboxFunctionSurfaceIsPinned(t *testing.T) {
	golden, err := os.ReadFile("testdata/sandbox_functions.txt")
	require.NoError(t, err)

	expected := strings.Split(strings.TrimSpace(string(golden)), "\n")
	assert.Equal(t, expected, SandboxFuncNames(),
		"the sandbox function set changed: review every added function for determinism, then update testdata/sandbox_functions.txt")
}

func TestSandboxDropsNondeterministicFunctions(t *testing.T) {
	for _, name := range nondeterministicFuncs {
		t.Run(name, func(t *testing.T) {
			_, err := parseTemplate("{{ " + name + " }}")
			require.ErrorContains(t, err, "function \""+name+"\" not defined",
				"a template must not be able to reach %s: the rendered object is frozen for the life of a generation", name)
		})
	}
}

func TestSandboxDropsHelmOnlyFunctions(t *testing.T) {
	for _, name := range []string{"include", "tpl", "lookup"} {
		t.Run(name, func(t *testing.T) {
			_, err := parseTemplate("{{ " + name + " }}")
			require.ErrorContains(t, err, "not defined")
		})
	}
}
