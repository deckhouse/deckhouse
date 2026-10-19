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

package nodeconfig

import (
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	sigsyaml "sigs.k8s.io/yaml"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// nodeConfigAPI is one API server holding the NodeConfig CRD alone, shared by the
// tests that ask what it stores or refuses; started on first use, stopped by TestMain.
var nodeConfigAPI struct {
	once   sync.Once
	env    *envtest.Environment
	client client.Client
	err    error
}

func nodeConfigAPIClient(t *testing.T) client.Client {
	t.Helper()
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` (or set KUBEBUILDER_ASSETS) to run this test")
	}
	nodeConfigAPI.once.Do(func() {
		scheme := k8sruntime.NewScheme()
		if nodeConfigAPI.err = internalv1alpha1.AddToScheme(scheme); nodeConfigAPI.err != nil {
			return
		}
		nodeConfigAPI.env, _, nodeConfigAPI.client, nodeConfigAPI.err = testenv.Start(scheme,
			testenv.NodeManagerCRDPaths(testenv.NodeConfigCRDFile)...)
	})
	require.NoError(t, nodeConfigAPI.err)
	return nodeConfigAPI.client
}

func TestMain(m *testing.M) {
	code := m.Run()
	if nodeConfigAPI.env != nil {
		if err := nodeConfigAPI.env.Stop(); err != nil {
			fmt.Fprintln(os.Stderr, "stop the NodeConfig API server:", err)
		}
	}
	os.Exit(code)
}

// One CRD serves both system types, and the API server is what keeps them apart:
// an Engine node without an OS image can never be updated, a node that changed type
// would have two owners configuring it, and a Mutable document carrying Engine
// sections is configuration bashible and the agent would both apply.
func TestNodeConfigSchemaTellsTheSystemTypesApart(t *testing.T) {
	c := nodeConfigAPIClient(t)

	t.Run("a Mutable document needs no OS image", func(t *testing.T) {
		require.NoError(t, c.Create(t.Context(), rawNodeConfig("mutable-0", map[string]any{"systemType": "Mutable"})))
	})

	t.Run("an Immutable document does", func(t *testing.T) {
		err := c.Create(t.Context(), rawNodeConfig("immutable-0", map[string]any{"systemType": "Immutable"}))
		require.ErrorContains(t, err, "spec.osImage must be set on an Immutable node")
	})

	// No default: a writer that names no type wrote a document for no node.
	t.Run("a document that names no type is refused", func(t *testing.T) {
		err := c.Create(t.Context(), rawNodeConfig("untyped-0", map[string]any{}))
		require.ErrorContains(t, err, "spec.systemType: Required value")
	})

	t.Run("the type cannot be changed", func(t *testing.T) {
		document := rawNodeConfig("mutable-1", map[string]any{"systemType": "Mutable"})
		require.NoError(t, c.Create(t.Context(), document))

		require.NoError(t, unstructured.SetNestedField(document.Object, "Immutable", "spec", "systemType"))
		require.NoError(t, unstructured.SetNestedMap(document.Object,
			map[string]any{"digest": "sha256:" + strings.Repeat("a", 64)}, "spec", "osImage"))
		require.ErrorContains(t, c.Update(t.Context(), document), "spec.systemType cannot be changed")
	})

	t.Run("a feature name is a DNS label", func(t *testing.T) {
		err := c.Create(t.Context(), rawNodeConfig("mutable-2", map[string]any{
			"systemType":   "Mutable",
			"featureGates": map[string]any{"Static_Pods": true},
		}))
		require.ErrorContains(t, err, "a feature name is a DNS label")
	})

	t.Run("a Mutable document takes only the agent's part", func(t *testing.T) {
		err := c.Create(t.Context(), rawNodeConfig("mutable-3", map[string]any{
			"systemType": "Mutable",
			"kubelet":    map[string]any{"maxPods": int64(110)},
		}))
		require.ErrorContains(t, err, "a Mutable node takes only nodeName, featureGates, apiServerEndpoints, registryPackagesProxy, extensions and staticPods")
	})
}

// A NodeConfig is named after its node, and the agent configures its hostname and
// PKI by spec.nodeName: the two cannot disagree, and the second cannot move.
func TestNodeConfigSchemaPinsTheNodeName(t *testing.T) {
	c := nodeConfigAPIClient(t)

	misnamed := rawNodeConfig("named-0", map[string]any{"systemType": "Mutable"})
	misnamed.SetName("named-1")
	require.ErrorContains(t, c.Create(t.Context(), misnamed), "a NodeConfig is named after its node")

	document := rawNodeConfig("named-2", map[string]any{"systemType": "Mutable"})
	require.NoError(t, c.Create(t.Context(), document))
	require.NoError(t, unstructured.SetNestedField(document.Object, "named-3", "spec", "nodeName"))
	require.ErrorContains(t, c.Update(t.Context(), document), "spec.nodeName cannot be changed")
}

// rawNodeConfig is a NodeConfig as plain fields, so the schema is exercised by
// what is sent, whatever the Go type knows.
func rawNodeConfig(name string, spec map[string]any) *unstructured.Unstructured {
	spec["nodeName"] = name
	document := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	document.SetAPIVersion(internalv1alpha1.GroupVersion.String())
	document.SetKind("NodeConfig")
	document.SetName(name)
	return document
}

// The first master's agent fetches its packages through the installer's proxy, named
// with its scheme and port; the API refuses an address the agent would refuse.
func TestNodeConfigSchemaTakesTheProxyEndpointsOfTheFirstMaster(t *testing.T) {
	c := nodeConfigAPIClient(t)

	t.Run("http and https addresses are kept, the token as bytes", func(t *testing.T) {
		endpoints := []any{"http://127.0.0.1:5444", "https://10.0.0.1:4219"}
		token := base64.StdEncoding.EncodeToString([]byte("proxy-token"))
		document := rawNodeConfig("seed-0", map[string]any{"systemType": "Mutable", "registryPackagesProxy": map[string]any{
			"endpoints": endpoints, "accessToken": token,
		}})
		require.NoError(t, c.Create(t.Context(), document))

		stored := &internalv1alpha1.NodeConfig{}
		require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: "seed-0"}, stored))
		require.Equal(t, []string{"http://127.0.0.1:5444", "https://10.0.0.1:4219"}, stored.Spec.RegistryPackagesProxy.Endpoints)
		require.Equal(t, []byte("proxy-token"), stored.Spec.RegistryPackagesProxy.AccessToken,
			"the field carries the raw token, base64 once on the wire")
	})

	for name, endpoint := range map[string]string{
		"another scheme": "ftp://10.0.0.1:21",
		"no scheme":      "127.0.0.1:5444",
		"no port":        "http://127.0.0.1",
	} {
		t.Run(name+" is refused", func(t *testing.T) {
			document := rawNodeConfig("seed-"+strings.ReplaceAll(name, " ", "-"), map[string]any{
				"systemType":            "Mutable",
				"registryPackagesProxy": map[string]any{"endpoints": []any{endpoint}},
			})
			require.ErrorContains(t, c.Create(t.Context(), document), "spec.registryPackagesProxy.endpoints[0]")
		})
	}

	// The API servers are reached over TLS only, and the list is a set.
	t.Run("an API server endpoint without https is refused", func(t *testing.T) {
		document := rawNodeConfig("endpoints-0", map[string]any{"systemType": "Mutable", "apiServerEndpoints": []any{"10.0.0.1:6443"}})
		require.ErrorContains(t, c.Create(t.Context(), document), "spec.apiServerEndpoints[0]")
	})
	t.Run("a repeated API server endpoint is refused", func(t *testing.T) {
		document := rawNodeConfig("endpoints-1", map[string]any{"systemType": "Mutable",
			"apiServerEndpoints": []any{"https://10.0.0.1:6443", "https://10.0.0.1:6443"}})
		require.ErrorContains(t, c.Create(t.Context(), document), "Duplicate value")
	})
}

// immutableNodeConfig is an Engine document with the sections a machine registers with.
func immutableNodeConfig(name string) *unstructured.Unstructured {
	return rawNodeConfig(name, map[string]any{
		"systemType": "Immutable",
		"osImage":    map[string]any{"digest": "sha256:" + strings.Repeat("a", 64)},
		"storage": map[string]any{
			"device":       "/dev/sda",
			"diskSelector": map[string]any{"size": ">=30Gi"},
			"wipe":         true,
			"mounts":       []any{map[string]any{"name": "data", "device": "/dev/sdb1"}},
		},
		"network": map[string]any{"interfaces": []any{map[string]any{
			"name": "eth0", "dhcp": false, "addresses": []any{"192.0.2.10/24"}, "gateway": "192.0.2.1",
		}}},
	})
}

// The disk a system was installed on is a fact about the machine: a changed device,
// selector or wipe flag would point the next boot at another disk, or at wiping this
// one. Mounts and the network are applied on the live node and may change. The rules
// compare a field the old and the new object both have.
func TestNodeConfigSchemaFreezesTheInstallDisk(t *testing.T) {
	c := nodeConfigAPIClient(t)

	for name, tc := range map[string]struct {
		path    []string
		value   any
		message string
	}{
		"device":        {path: []string{"device"}, value: "/dev/sdb", message: "spec.storage.device names the disk"},
		"wipe":          {path: []string{"wipe"}, value: false, message: "spec.storage.wipe names the disk"},
		"disk selector": {path: []string{"diskSelector", "size"}, value: ">=40Gi", message: "spec.storage.diskSelector names the disk"},
	} {
		t.Run("a changed "+name+" is refused", func(t *testing.T) {
			document := immutableNodeConfig("disk-" + strings.ReplaceAll(name, " ", "-"))
			require.NoError(t, c.Create(t.Context(), document))

			require.NoError(t, unstructured.SetNestedField(document.Object, tc.value, append([]string{"spec", "storage"}, tc.path...)...))
			require.ErrorContains(t, c.Update(t.Context(), document), tc.message)
		})
	}

	t.Run("mounts and the network change through the API", func(t *testing.T) {
		document := immutableNodeConfig("disk-mounts")
		require.NoError(t, c.Create(t.Context(), document))

		require.NoError(t, unstructured.SetNestedSlice(document.Object,
			[]any{map[string]any{"name": "logs", "device": "/dev/sdc1"}}, "spec", "storage", "mounts"))
		require.NoError(t, unstructured.SetNestedSlice(document.Object, []any{map[string]any{"name": "eth1", "dhcp": true}},
			"spec", "network", "interfaces"))
		require.NoError(t, c.Update(t.Context(), document))
	})
}

// An agent newer than the cluster's schema publishes fields the schema does not know
// yet; the API server keeps them instead of refusing or pruning the whole status.
func TestNodeConfigSchemaKeepsTheStatusFieldsItDoesNotKnow(t *testing.T) {
	c := nodeConfigAPIClient(t)

	document := rawNodeConfig("status-0", map[string]any{"systemType": "Mutable"})
	require.NoError(t, c.Create(t.Context(), document))
	require.NoError(t, unstructured.SetNestedField(document.Object, "Ready", "status", "phase"))
	require.NoError(t, unstructured.SetNestedField(document.Object, "from a newer agent", "status", "somethingNew"))
	require.NoError(t, unstructured.SetNestedField(document.Object, "v0.1.16", "status", "agent", "version"))
	require.NoError(t, c.Status().Update(t.Context(), document))

	stored := &unstructured.Unstructured{}
	stored.SetGroupVersionKind(document.GroupVersionKind())
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: "status-0"}, stored))
	kept, _, err := unstructured.NestedString(stored.Object, "status", "somethingNew")
	require.NoError(t, err)
	require.Equal(t, "from a newer agent", kept)
	version, _, err := unstructured.NestedString(stored.Object, "status", "agent", "version")
	require.NoError(t, err)
	require.Equal(t, "v0.1.16", version)

	// Known fields are still validated.
	require.NoError(t, unstructured.SetNestedField(stored.Object, "Unknown", "status", "phase"))
	require.ErrorContains(t, c.Status().Update(t.Context(), stored), "status.phase")
}

// kubectl get nc answers how the node is doing; -o wide adds which kind of node,
// which group and which generation it runs.
func TestNodeConfigPrinterColumns(t *testing.T) {
	raw, err := os.ReadFile(testenv.NodeManagerCRDPaths(testenv.NodeConfigCRDFile)[0])
	require.NoError(t, err)
	var crd struct {
		Spec struct {
			Versions []struct {
				Columns []struct {
					Name     string `json:"name"`
					JSONPath string `json:"jsonPath"`
					Priority int    `json:"priority"`
				} `json:"additionalPrinterColumns"`
			} `json:"versions"`
		} `json:"spec"`
	}
	require.NoError(t, sigsyaml.Unmarshal(raw, &crd))

	type column struct {
		name, jsonPath string
		priority       int
	}
	var columns []column
	for _, c := range crd.Spec.Versions[0].Columns {
		columns = append(columns, column{c.Name, c.JSONPath, c.Priority})
	}
	require.Equal(t, []column{
		{"SystemType", ".spec.systemType", 1},
		{"Group", `.metadata.labels.node\.deckhouse\.io/group`, 1},
		{"Phase", ".status.phase", 0},
		{"Applied", ".status.appliedGeneration", 1},
		{"Age", ".metadata.creationTimestamp", 0},
	}, columns)
}

// The shipped CRD is copied from nodelet (hack/sync-nodeconfig-crd.sh) while these Go
// types stay node-controller's own: a field one of them has and the other lacks is a
// field the API server prunes, or one this controller can neither read nor write.
func TestTheGoTypesAndTheShippedCRDAgree(t *testing.T) {
	schema := nodeConfigCRDSchema(t)
	fromCRD := map[string]bool{}
	schemaPaths(schema, "", fromCRD)
	fromGo := map[string]bool{}
	typePaths(reflect.TypeOf(internalv1alpha1.NodeConfig{}), "", fromGo)

	var onlyCRD, onlyGo []string
	for path := range fromCRD {
		if !fromGo[path] {
			onlyCRD = append(onlyCRD, path)
		}
	}
	for path := range fromGo {
		if !fromCRD[path] {
			onlyGo = append(onlyGo, path)
		}
	}
	sort.Strings(onlyCRD)
	sort.Strings(onlyGo)
	require.Empty(t, onlyCRD, "fields the CRD has and the Go types do not")
	require.Empty(t, onlyGo, "fields the Go types have and the CRD does not")
}

// An artifact is located by the proxy's repository key and a path within it, in the Go
// types and in the Russian description of the CRD alike.
func TestTheArtifactsAreLocatedByRepositoryAndPath(t *testing.T) {
	fromGo := map[string]bool{}
	typePaths(reflect.TypeOf(internalv1alpha1.NodeConfig{}), "", fromGo)
	russian := map[string]bool{}
	schemaPaths(testenv.ShippedCRDSchema(t, "doc-ru-"+testenv.NodeConfigCRDFile), "", russian)

	for where, fields := range map[string]map[string]bool{"the Go types": fromGo, "the Russian description": russian} {
		for _, artifact := range []string{".spec.osImage", ".spec.extensions[]"} {
			require.True(t, fields[artifact+".repository"], "%s lack %s.repository", where, artifact)
			require.True(t, fields[artifact+".path"], "%s lack %s.path", where, artifact)
		}
	}
}

// schemaPaths collects the property paths of a structural schema below spec and status.
func schemaPaths(node map[string]any, prefix string, out map[string]bool) {
	if properties, ok := node["properties"].(map[string]any); ok {
		for name, child := range properties {
			if prefix == "" && name != "spec" && name != "status" {
				continue
			}
			out[prefix+"."+name] = true
			schemaPaths(child.(map[string]any), prefix+"."+name, out)
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		schemaPaths(items, prefix+"[]", out)
	}
	if values, ok := node["additionalProperties"].(map[string]any); ok {
		schemaPaths(values, prefix+"{}", out)
	}
}

// typePaths collects the JSON paths of a Go type the way controller-gen renders them:
// inline fields flatten, pointers and slices step through, metav1 types are leaves.
func typePaths(typ reflect.Type, prefix string, out map[string]bool) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Slice:
		if typ.Elem().Kind() != reflect.Uint8 {
			typePaths(typ.Elem(), prefix+"[]", out)
		}
		return
	case reflect.Map:
		typePaths(typ.Elem(), prefix+"{}", out)
		return
	case reflect.Struct:
	default:
		return
	}
	if typ == reflect.TypeOf(metav1.Time{}) {
		return
	}
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if strings.Contains(options, "inline") {
			if field.Type != reflect.TypeOf(metav1.TypeMeta{}) {
				typePaths(field.Type, prefix, out)
			}
			continue
		}
		if prefix == "" && name != "spec" && name != "status" {
			continue
		}
		out[prefix+"."+name] = true
		typePaths(field.Type, prefix+"."+name, out)
	}
}
