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
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

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
// an Engine node without an OS image can never be updated, and a node that
// changed type would have two owners configuring it.
func TestNodeConfigSchemaTellsTheSystemTypesApart(t *testing.T) {
	c := nodeConfigAPIClient(t)

	t.Run("a Mutable document needs no OS image", func(t *testing.T) {
		require.NoError(t, c.Create(t.Context(), rawNodeConfig("mutable-0", map[string]any{"systemType": "Mutable"})))
	})

	t.Run("an Immutable document does", func(t *testing.T) {
		err := c.Create(t.Context(), rawNodeConfig("immutable-0", map[string]any{"systemType": "Immutable"}))
		require.ErrorContains(t, err, "spec.osImage must be set on an Immutable node")
	})

	t.Run("a document that names no type is an Immutable one", func(t *testing.T) {
		err := c.Create(t.Context(), rawNodeConfig("untyped-0", map[string]any{}))
		require.ErrorContains(t, err, "spec.osImage must be set on an Immutable node")
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

	t.Run("http and https addresses are kept", func(t *testing.T) {
		endpoints := []any{"http://127.0.0.1:5444", "https://10.0.0.1:4219"}
		document := rawNodeConfig("seed-0", map[string]any{"systemType": "Mutable", "registryPackagesProxyEndpoints": endpoints})
		require.NoError(t, c.Create(t.Context(), document))

		stored := &unstructured.Unstructured{}
		stored.SetGroupVersionKind(document.GroupVersionKind())
		require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: "seed-0"}, stored))
		kept, found, err := unstructured.NestedSlice(stored.Object, "spec", "registryPackagesProxyEndpoints")
		require.NoError(t, err)
		require.True(t, found, "the API server pruned the field")
		require.Equal(t, endpoints, kept)
	})

	for name, endpoint := range map[string]string{
		"another scheme": "ftp://10.0.0.1:21",
		"no scheme":      "127.0.0.1:5444",
		"no port":        "http://127.0.0.1",
	} {
		t.Run(name+" is refused", func(t *testing.T) {
			document := rawNodeConfig("seed-"+strings.ReplaceAll(name, " ", "-"), map[string]any{
				"systemType":                     "Mutable",
				"registryPackagesProxyEndpoints": []any{endpoint},
			})
			require.ErrorContains(t, c.Create(t.Context(), document), "spec.registryPackagesProxyEndpoints[0]")
		})
	}
}
