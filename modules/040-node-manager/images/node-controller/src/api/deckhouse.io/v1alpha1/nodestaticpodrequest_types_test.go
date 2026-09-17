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

package v1alpha1

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/testenv"
)

// The control plane's own manifests are written by the node agent. A static pod
// of one of those names would give one file two writers, and the loser is
// whichever of them ran last.
func TestIsReservedStaticPodName(t *testing.T) {
	for _, name := range []string{"etcd", "kube-apiserver", "kube-controller-manager", "kube-scheduler"} {
		require.True(t, IsReservedStaticPodName(name), "%s is the node agent's own manifest", name)
	}
	require.False(t, IsReservedStaticPodName("registry-agent"))
	require.False(t, IsReservedStaticPodName("etcd-backup"))
}

// The manifest is checked where it is written, not on every node that reads it:
// a node refusing a document it cannot decode has already taken the rollout slot,
// and it refuses the whole NodeConfig with it. One question — is this a valid Pod
// — answered by the decoder rather than by a second opinion about kinds.
func TestValidateStaticPodManifest(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		wantPod  string
		wantErr  string
	}{
		{
			// The pod key comes back so the caller does not parse the document a
			// second time to find out which pod two objects are fighting over.
			name:     "a Pod with a name and a namespace",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n  namespace: d8-system\nspec:\n  hostNetwork: true\n  containers:\n  - name: agent\n    image: deckhouse.local/images:registry-agent\n",
			wantPod:  "d8-system/registry-agent",
		},
		{
			// The object's own name is the file name on the node and nothing more:
			// kubelet takes the mirror pod's name from the document, never from the
			// file, so the two have no reason to agree.
			name:     "the pod is named nothing like its object",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: something-else\n  namespace: kube-system\n",
			wantPod:  "kube-system/something-else",
		},
		{
			// Loose parsing, so a field this binary's k8s.io/api does not know is
			// accepted: the document is executed by a kubelet of its own version,
			// and refusing it here would refuse a Pod field newer than the vendored
			// types on every node the document reaches.
			name:     "a field the vendored Pod type does not know",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n  namespace: d8-system\nspec:\n  hostNetwrk: true\n",
			wantPod:  "d8-system/registry-agent",
		},
		{
			name:     "not a document at all",
			manifest: "\tnot: yaml",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			// Loose is not blind: a field of the right name and the wrong type is
			// still a manifest kubelet cannot run, and the decoder says so.
			name:     "a field of the right name and the wrong type",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n  namespace: d8-system\nspec:\n  hostNetwork: \"yes, please\"\n",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			// The case the nil "into" exists for: handed a &corev1.Pod{} the decoder
			// would take the missing kind from the destination type and accept this
			// as a Pod. With nil it answers "Object 'Kind' is missing in ...".
			name:     "no kind at all",
			manifest: "apiVersion: v1\nmetadata:\n  name: registry-agent\n  namespace: d8-system\n",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			name:     "no apiVersion and no kind",
			manifest: "metadata:\n  name: registry-agent\n  namespace: d8-system\n",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			// A group this scheme never registered: the decoder refuses it before
			// anything here has to reason about kinds.
			name:     "a Deployment is not a static pod",
			manifest: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: registry-agent\n  namespace: d8-system\n",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			// A core/v1 kind that is not a Pod decodes cleanly, so the type
			// assertion is the one that has to catch it — and it names the kind the
			// decoder did find rather than the Go type it built.
			name:     "a ConfigMap is not a static pod either",
			manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: registry-agent\n  namespace: d8-system\n",
			wantErr:  "unexpected kind /v1, Kind=ConfigMap",
		},
		{
			name:     "no name",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  namespace: d8-system\n",
			wantErr:  "metadata.name is empty",
		},
		{
			name:     "no namespace",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n",
			wantErr:  "metadata.namespace is empty",
		},
		{
			name:    "nothing at all",
			wantErr: "manifest is not a valid Pod",
		},
		{
			// Pinning what the decoder does rather than asking for it: it reads
			// one document and the rest of the stream is not looked at, so a
			// second pod hidden behind a --- is keyed by the first one's name.
			name:     "two documents in one manifest",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: first\n  namespace: ns-a\n---\napiVersion: v1\nkind: Pod\nmetadata:\n  name: second\n  namespace: ns-b\n",
			wantPod:  "ns-a/first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod, err := ValidateStaticPodManifest(tt.manifest)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.wantPod, pod)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// The markers on the type and the shipped CRD are two copies of one declaration
// and nothing regenerates one from the other: this API group ships its CRD by
// hand. This test is what keeps them equal.
func TestShippedCRDMatchesTheGoTypes(t *testing.T) {
	paths := testenv.NodeManagerCRDPaths(testenv.NodeStaticPodRequestCRDFile)
	require.Len(t, paths, 1)
	raw, err := os.ReadFile(paths[0])
	require.NoError(t, err, "the shipped NodeStaticPodRequest CRD must be readable at %s", paths[0])

	var crd struct {
		Spec struct {
			Scope string `json:"scope"`
			Names struct {
				ShortNames []string `json:"shortNames"`
			} `json:"names"`
			Versions []struct {
				Subresources map[string]any `json:"subresources"`
				Columns      []struct {
					Name     string `json:"name"`
					JSONPath string `json:"jsonPath"`
				} `json:"additionalPrinterColumns"`
				Schema struct {
					OpenAPIV3Schema map[string]any `json:"openAPIV3Schema"`
				} `json:"schema"`
			} `json:"versions"`
		} `json:"spec"`
	}
	require.NoError(t, sigsyaml.Unmarshal(raw, &crd))
	require.Len(t, crd.Spec.Versions, 1)
	version := crd.Spec.Versions[0]

	require.Equal(t, "Cluster", crd.Spec.Scope)
	require.Equal(t, []string{"nspr"}, crd.Spec.Names.ShortNames)
	require.Contains(t, version.Subresources, "status")

	shipped := make([][2]string, 0, len(version.Columns))
	for _, column := range version.Columns {
		shipped = append(shipped, [2]string{column.Name, column.JSONPath})
	}
	require.Equal(t, printerColumnMarkers(t), shipped)

	schema := version.Schema.OpenAPIV3Schema
	require.Equal(t, []any{"manifest"}, crdField(t, schema, "spec")["required"])

	manifest := crdField(t, schema, "spec", "manifest")
	require.Equal(t, float64(1), manifest["minLength"])
	// Counted in runes by the API server, and nodelet's loader counts runes too.
	require.Equal(t, float64(32768), manifest["maxLength"])

	require.Equal(t, []any{"Ready", "Degraded"}, crdField(t, schema, "status", "phase")["enum"])
}

var printerColumnMarker = regexp.MustCompile(`\+kubebuilder:printcolumn:name=([^,]+),jsonPath=([^,]+),`)

// printerColumnMarkers returns the name and jsonPath of every printcolumn marker
// on the type, in the order they are declared.
func printerColumnMarkers(t *testing.T) [][2]string {
	t.Helper()

	raw, err := os.ReadFile("nodestaticpodrequest_types.go")
	require.NoError(t, err)

	var columns [][2]string
	for _, match := range printerColumnMarker.FindAllStringSubmatch(string(raw), -1) {
		columns = append(columns, [2]string{match[1], match[2]})
	}
	return columns
}

// crdField walks the schema down a property path to one field.
func crdField(t *testing.T, schema map[string]any, path ...string) map[string]any {
	t.Helper()

	node := schema
	for _, name := range path {
		properties, ok := node["properties"].(map[string]any)
		require.True(t, ok, "nothing above %s has properties", name)
		node, ok = properties[name].(map[string]any)
		require.True(t, ok, "the CRD has no %s", name)
	}
	return node
}
