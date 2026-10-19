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
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/testenv"
)

// Matched alone is a denominator with no numerator: kubectl get ner said how
// many nodes the request covers and never how many got it. The markers on the
// type and the shipped CRD are two copies of one declaration — this API group
// ships its CRD by hand — so both are pinned here.
func TestNERPrinterColumnsSayWhatTheNodesDidWithIt(t *testing.T) {
	markers := printerColumnMarkers(t, "nodeextensionrequest_types.go")
	require.Equal(t, [][2]string{
		{"Kind", ".spec.extension.kind"},
		{"Phase", ".status.phase"},
		{"Matched", ".status.matchedNodes"},
		{"Applied", ".status.appliedNodes"},
		{"Failed", ".status.failedNodes"},
		{"Age", ".metadata.creationTimestamp"},
	}, markers)

	paths := testenv.NodeManagerCRDPaths(testenv.NodeExtensionRequestCRDFile)
	require.Len(t, paths, 1)
	raw, err := os.ReadFile(paths[0])
	require.NoError(t, err, "the shipped NodeExtensionRequest CRD must be readable at %s", paths[0])

	var crd struct {
		Spec struct {
			Versions []struct {
				Columns []struct {
					Name     string `json:"name"`
					JSONPath string `json:"jsonPath"`
				} `json:"additionalPrinterColumns"`
			} `json:"versions"`
		} `json:"spec"`
	}
	require.NoError(t, sigsyaml.Unmarshal(raw, &crd))
	require.Len(t, crd.Spec.Versions, 1)

	shipped := make([][2]string, 0, len(crd.Spec.Versions[0].Columns))
	for _, column := range crd.Spec.Versions[0].Columns {
		shipped = append(shipped, [2]string{column.Name, column.JSONPath})
	}
	require.Equal(t, markers, shipped)
}

// The requester declares what the image is and nothing guesses it: the API server
// refuses a request without one of the two kinds, and the status keeps no second copy.
func TestShippedNERCRDRequiresTheExtensionKind(t *testing.T) {
	schema := testenv.ShippedCRDSchema(t, testenv.NodeExtensionRequestCRDFile)

	require.Contains(t, crdField(t, schema, "spec", "extension")["required"], "kind")
	require.Equal(t, []any{string(ExtensionKindPackage), string(ExtensionKindSystemExtension)},
		crdField(t, schema, "spec", "extension", "kind")["enum"])
	require.NotContains(t, crdField(t, schema, "status")["properties"], "kind")
}

// One request carries what a node needs to fetch and load the image, named as in
// NodeConfig: the proxy's repository key and path, the kernel modules next to the
// image they come with, and the nodes' refusal only in the Ready condition.
func TestShippedNERCRDCarriesTheFrozenShape(t *testing.T) {
	schema := testenv.ShippedCRDSchema(t, testenv.NodeExtensionRequestCRDFile)

	require.NotContains(t, crdField(t, schema, "spec")["properties"], "kernelModules")
	require.NotContains(t, crdField(t, schema, "spec", "extension")["properties"], "source")
	require.Equal(t, float64(1024), crdField(t, schema, "spec", "extension", "repository")["maxLength"])
	require.Equal(t, "^[^[:space:]]+$", crdField(t, schema, "spec", "extension", "repository")["pattern"])
	russian := testenv.ShippedCRDSchema(t, "doc-ru-"+testenv.NodeExtensionRequestCRDFile)
	require.Contains(t, crdField(t, russian, "spec", "extension")["properties"], "repository")
	require.NotContains(t, crdField(t, russian, "spec", "extension")["properties"], "source")
	require.Equal(t, []any{"name"}, crdField(t, schema, "spec", "extension", "kernelModules")["x-kubernetes-list-map-keys"])
	require.Equal(t, []any{"Pending", "Ready", "Degraded"}, crdField(t, schema, "status", "phase")["enum"])
}

const testExtensionDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func drbdExtension() map[string]any {
	return map[string]any{
		"name":       "drbd",
		"digest":     testExtensionDigest,
		"kind":       "SystemExtension",
		"repository": "registry.example.com/deckhouse/modules",
		"path":       "drbd/extensions",
		"kernelModules": []any{
			map[string]any{"name": "drbd", "params": []any{"usermode_helper=disabled"}},
			map[string]any{"name": "drbd_transport_tcp"},
		},
	}
}

// The API server answers for what no node could install, so a bad request is
// refused at the door instead of failing on every node it selects.
func TestNERSchemaRefusesWhatNoNodeCouldInstall(t *testing.T) {
	c := requestsAPIClient(t)

	t.Run("a system extension with its repository and kernel modules is stored as sent", func(t *testing.T) {
		request := rawRequest("NodeExtensionRequest", "drbd", map[string]any{"extension": drbdExtension()})
		require.NoError(t, c.Create(t.Context(), request))

		stored := &unstructured.Unstructured{}
		stored.SetGroupVersionKind(request.GroupVersionKind())
		require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: "drbd"}, stored))
		extension, _, err := unstructured.NestedMap(stored.Object, "spec", "extension")
		require.NoError(t, err)
		require.Equal(t, drbdExtension(), extension)
	})

	t.Run("the documented examples are accepted", func(t *testing.T) {
		for _, example := range docExamples(t, testenv.NodeExtensionRequestCRDFile) {
			example.SetName("example-" + example.GetName())
			require.NoError(t, c.Create(t.Context(), example), example.GetName())
		}
	})

	t.Run("kernel modules on a Package are refused", func(t *testing.T) {
		extension := drbdExtension()
		extension["kind"] = "Package"
		err := c.Create(t.Context(), rawRequest("NodeExtensionRequest", "drbd-package", map[string]any{"extension": extension}))
		require.ErrorContains(t, err, "kernel modules load from a SystemExtension only")
	})

	t.Run("the kind cannot be changed", func(t *testing.T) {
		extension := drbdExtension()
		delete(extension, "kernelModules")
		request := rawRequest("NodeExtensionRequest", "drbd-kind", map[string]any{"extension": extension})
		require.NoError(t, c.Create(t.Context(), request))

		require.NoError(t, unstructured.SetNestedField(request.Object, "Package", "spec", "extension", "kind"))
		require.ErrorContains(t, c.Update(t.Context(), request), "kind cannot be changed: delete the request and create it again")
	})

	t.Run("the status takes the Pending phase", func(t *testing.T) {
		extension := drbdExtension()
		delete(extension, "kernelModules")
		request := rawRequest("NodeExtensionRequest", "drbd-status", map[string]any{"extension": extension})
		require.NoError(t, c.Create(t.Context(), request))

		require.NoError(t, unstructured.SetNestedField(request.Object, "Pending", "status", "phase"))
		require.NoError(t, c.Status().Update(t.Context(), request))
	})

	tooMany := func(count int, item func(int) any) []any {
		items := make([]any, 0, count)
		for i := range count {
			items = append(items, item(i))
		}
		return items
	}
	labels := func(count int) map[string]any {
		items := make(map[string]any, count)
		for i := range count {
			items[fmt.Sprintf("example.com/label-%d", i)] = "true"
		}
		return items
	}
	refused := []struct {
		name   string
		change func(spec, extension map[string]any)
		field  string
	}{
		{
			name: "one kernel module listed twice",
			change: func(_, extension map[string]any) {
				extension["kernelModules"] = []any{map[string]any{"name": "drbd"}, map[string]any{"name": "drbd"}}
			},
			field: "spec.extension.kernelModules[1]",
		},
		{
			name: "33 kernel modules",
			change: func(_, extension map[string]any) {
				extension["kernelModules"] = tooMany(33, func(i int) any { return map[string]any{"name": fmt.Sprintf("module%d", i)} })
			},
			field: "spec.extension.kernelModules",
		},
		{
			name: "a kernel module name longer than 64 characters",
			change: func(_, extension map[string]any) {
				extension["kernelModules"] = []any{map[string]any{"name": strings.Repeat("m", 65)}}
			},
			field: "spec.extension.kernelModules[0].name",
		},
		{
			name: "33 parameters of one kernel module",
			change: func(_, extension map[string]any) {
				extension["kernelModules"] = []any{map[string]any{"name": "drbd", "params": tooMany(33, func(i int) any { return fmt.Sprintf("p%d=1", i) })}}
			},
			field: "spec.extension.kernelModules[0].params",
		},
		{
			name: "a parameter longer than 256 characters",
			change: func(_, extension map[string]any) {
				extension["kernelModules"] = []any{map[string]any{"name": "drbd", "params": []any{"p=" + strings.Repeat("1", 255)}}}
			},
			field: "spec.extension.kernelModules[0].params[0]",
		},
		{
			name:   "a repository with a space",
			change: func(_, extension map[string]any) { extension["repository"] = "registry.example.com/deckhouse modules" },
			field:  "spec.extension.repository",
		},
		{
			name:   "a repository longer than 1024 characters",
			change: func(_, extension map[string]any) { extension["repository"] = strings.Repeat("r", 1025) },
			field:  "spec.extension.repository",
		},
		{
			name:   "a path longer than 512 characters",
			change: func(_, extension map[string]any) { extension["path"] = strings.Repeat("p", 513) },
			field:  "spec.extension.path",
		},
		{
			name:   "17 node labels",
			change: func(spec, _ map[string]any) { spec["nodeSelector"] = map[string]any{"matchLabels": labels(17)} },
			field:  "spec.nodeSelector.matchLabels",
		},
		{
			name: "a label value longer than 63 characters",
			change: func(spec, _ map[string]any) {
				spec["nodeSelector"] = map[string]any{"matchLabels": map[string]any{"tier": strings.Repeat("v", 64)}}
			},
			field: "spec.nodeSelector.matchLabels.tier",
		},
	}
	for i, tt := range refused {
		t.Run(tt.name+" is refused", func(t *testing.T) {
			spec := map[string]any{"extension": drbdExtension()}
			tt.change(spec, spec["extension"].(map[string]any))
			err := c.Create(t.Context(), rawRequest("NodeExtensionRequest", fmt.Sprintf("refused-%d", i), spec))
			require.True(t, apierrors.IsInvalid(err), "want Invalid, got %v", err)
			require.ErrorContains(t, err, tt.field)
		})
	}

	requireGroupSelectorBounded(t, c, "NodeExtensionRequest", map[string]any{"extension": map[string]any{
		"name": "selected", "digest": testExtensionDigest, "kind": "Package",
	}})
}

// The console reads the state of each node from the request, without NodeConfigs. Both shipped
// CRDs keep status.nodes as node-controller writes it, up to MaxStatusNodes entries, and describe
// it in both languages.
func TestRequestStatusListsAtMostMaxStatusNodes(t *testing.T) {
	for _, file := range []testenv.NodeManagerCRDFile{testenv.NodeExtensionRequestCRDFile, testenv.NodeStaticPodRequestCRDFile} {
		require.Equal(t, float64(MaxStatusNodes), crdField(t, testenv.ShippedCRDSchema(t, file), "status", "nodes")["maxItems"], file)
		require.NotEmpty(t, crdField(t, testenv.ShippedCRDSchema(t, "doc-ru-"+file), "status", "nodes")["description"], file)
	}

	c := requestsAPIClient(t)
	listed := func(count int, state string) []any {
		nodes := make([]any, 0, count)
		for i := range count {
			nodes = append(nodes, map[string]any{"name": fmt.Sprintf("worker-%02d", i), "state": state, "message": "the node has no NodeConfig yet"})
		}
		return nodes
	}
	for _, request := range []*unstructured.Unstructured{
		rawRequest("NodeExtensionRequest", "listed", map[string]any{"extension": map[string]any{
			"name": "listed", "digest": testExtensionDigest, "kind": "Package",
		}}),
		rawRequest("NodeStaticPodRequest", "listed", map[string]any{
			"manifest": "apiVersion: v1\nkind: Pod\nmetadata:\n  name: agent\n  namespace: kube-system\n",
		}),
	} {
		t.Run(request.GetKind(), func(t *testing.T) {
			require.NoError(t, c.Create(t.Context(), request))

			require.NoError(t, unstructured.SetNestedSlice(request.Object, listed(MaxStatusNodes, "Failed"), "status", "nodes"))
			require.NoError(t, c.Status().Update(t.Context(), request))
			stored, _, err := unstructured.NestedSlice(request.Object, "status", "nodes")
			require.NoError(t, err)
			require.Equal(t, listed(MaxStatusNodes, "Failed"), stored)

			for _, nodes := range [][]any{listed(MaxStatusNodes+1, "Pending"), listed(1, "Applied")} {
				require.NoError(t, unstructured.SetNestedSlice(request.Object, nodes, "status", "nodes"))
				err := c.Status().Update(t.Context(), request)
				require.True(t, apierrors.IsInvalid(err), "want Invalid, got %v", err)
				require.ErrorContains(t, err, "status.nodes")
			}
		})
	}
}

// requireGroupSelectorBounded checks the selector NodeExtensionRequest and
// NodeStaticPodRequest share. An absent selector, one without names and an empty
// list all select every group.
func requireGroupSelectorBounded(t *testing.T, c client.Client, kind string, spec map[string]any) {
	t.Helper()

	names := func(count int) []any {
		items := make([]any, 0, count)
		for i := range count {
			items = append(items, fmt.Sprintf("group-%d", i))
		}
		return items
	}
	cases := []struct {
		name     string
		selector map[string]any
		field    string
	}{
		{name: "no selector"},
		{name: "a selector without names", selector: map[string]any{}},
		{name: "64 group names", selector: map[string]any{"matchNames": names(64)}},
		{name: "an empty list of group names", selector: map[string]any{"matchNames": []any{}}},
		{name: "65 group names", selector: map[string]any{"matchNames": names(65)}, field: "spec.nodeGroupSelector.matchNames"},
		{name: "one group name listed twice", selector: map[string]any{"matchNames": []any{"worker", "worker"}}, field: "spec.nodeGroupSelector.matchNames[1]"},
		{name: "a group name no NodeGroup can have", selector: map[string]any{"matchNames": []any{"Worker"}}, field: "spec.nodeGroupSelector.matchNames[0]"},
	}
	for i, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			full := maps.Clone(spec)
			if tt.selector != nil {
				full["nodeGroupSelector"] = tt.selector
			}
			err := c.Create(t.Context(), rawRequest(kind, fmt.Sprintf("selector-%d", i), full))
			if tt.field == "" {
				require.NoError(t, err)
				return
			}
			require.True(t, apierrors.IsInvalid(err), "want Invalid, got %v", err)
			require.ErrorContains(t, err, tt.field)
		})
	}
}

// requestsAPIClient starts an API server with the NodeExtensionRequest and
// NodeStaticPodRequest CRDs as shipped, stopped when the test ends.
func requestsAPIClient(t *testing.T) client.Client {
	t.Helper()
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run this test")
	}

	env, _, c, err := testenv.Start(runtime.NewScheme(),
		testenv.NodeManagerCRDPaths(testenv.NodeExtensionRequestCRDFile, testenv.NodeStaticPodRequestCRDFile)...)
	t.Cleanup(func() {
		if env == nil {
			return
		}
		if err := env.Stop(); err != nil {
			t.Errorf("stop the API server: %v", err)
		}
	})
	require.NoError(t, err)
	return c
}

func rawRequest(kind, name string, spec map[string]any) *unstructured.Unstructured {
	request := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	request.SetAPIVersion(GroupVersion.String())
	request.SetKind(kind)
	request.SetName(name)
	return request
}

// docExamples returns the objects of the x-doc-examples of a shipped CRD.
func docExamples(t *testing.T, file testenv.NodeManagerCRDFile) []*unstructured.Unstructured {
	t.Helper()

	examples, ok := testenv.ShippedCRDSchema(t, file)["x-doc-examples"].([]any)
	require.True(t, ok, "%s has no x-doc-examples", file)

	var objects []*unstructured.Unstructured
	for _, example := range examples {
		text, ok := example.(string)
		require.True(t, ok)
		text = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(text), "```yaml"), "```")
		decoder := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(text), 4096)
		for {
			object := &unstructured.Unstructured{}
			err := decoder.Decode(&object.Object)
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
			objects = append(objects, object)
		}
	}
	require.NotEmpty(t, objects)
	return objects
}
