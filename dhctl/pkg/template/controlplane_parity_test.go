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

package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/manifests"
)

// TestControlPlaneRenderParity is the guard on the move of the control-plane templates into
// go_lib/controlplane/manifests.
//
// dhctl used to read the templates off disk and render them with its own engine; it now calls
// manifests.Render, which embeds them and uses sprig alone. The Secret those bytes end up in is
// what control-plane-manager checksums, so the two engines have to agree byte for byte or every
// control-plane node rolls.
//
// The context does not have to be a realistic cluster for the comparison to mean something: a
// key missing from the context renders the same way through both engines, so what the test pins
// is the engines, not the values. The version map is real, so the branches that key off the
// Kubernetes version are exercised rather than skipped.
func TestControlPlaneRenderParity(t *testing.T) {
	repoRoot := repoRootForTest(t)

	versionMap := map[string]any{}
	raw, err := os.ReadFile(filepath.Join(repoRoot, "candi", "version_map.yml"))
	require.NoError(t, err, "read candi/version_map.yml")
	require.NoError(t, yaml.Unmarshal(raw, &versionMap), "parse candi/version_map.yml")

	cases := []struct {
		name string
		data map[string]any
	}{
		{
			name: "cluster bootstrap",
			data: controlPlaneParityContext(versionMap, "ClusterBootstrap", map[string]any{}),
		},
		{
			name: "normal run",
			data: controlPlaneParityContext(versionMap, "Normal", map[string]any{}),
		},
		{
			name: "apiserver features",
			data: controlPlaneParityContext(versionMap, "Normal", map[string]any{
				"auditPolicy":          "YXBpVmVyc2lvbjogYXVkaXQuazhzLmlvL3Yx",
				"oidcIssuerURL":        "https://issuer.example.com",
				"authnWebhookURL":      "https://authn.example.com",
				"webhookURL":           "https://authz.example.com",
				"admissionPlugins":     []any{"AlwaysPullImages"},
				"secretEncryptionKey":  "c2VjcmV0",
				"etcdServers":          []any{"https://10.0.0.1:2379"},
				"bindToWildcard":       true,
				"authnWebhookCacheTTL": "5m",
			}),
		},
	}

	// The symlink at candi/control-plane is what keeps the published path alive; reading through
	// it here also proves the link points at the templates the package embeds.
	templatesDir := filepath.Join(repoRoot, "candi", "control-plane")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fromDisk, err := RenderTemplatesDir(t.Context(), templatesDir, cloneContext(tc.data), nil)
			require.NoError(t, err, "render templates off disk")
			require.NotEmpty(t, fromDisk, "the symlinked templates directory rendered nothing")

			bundle, err := manifests.Render(t.Context(), cloneContext(tc.data), manifests.NodeInput{
				NodeName: tc.data["nodeName"].(string),
				NodeIP:   tc.data["nodeIP"].(string),
			})
			require.NoError(t, err, "render templates from the package")
			require.Len(t, bundle, len(fromDisk), "the package rendered a different number of manifests")

			embedded := make(map[string]string, len(bundle))
			for _, artifact := range bundle {
				embedded[artifact.Name] = string(artifact.Content)
			}

			for _, rendered := range fromDisk {
				got, ok := embedded[rendered.FileName]
				require.Truef(t, ok, "the package did not render %s", rendered.FileName)
				require.Equalf(t, rendered.Content.String(), got, "%s differs between the two renderers", rendered.FileName)
			}
		})
	}
}

// TestControlPlaneBundleOrder pins the write order control-plane-manager relies on.
func TestControlPlaneBundleOrder(t *testing.T) {
	versionMap := map[string]any{}
	raw, err := os.ReadFile(filepath.Join(repoRootForTest(t), "candi", "version_map.yml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(raw, &versionMap))

	bundle, err := manifests.Render(
		t.Context(),
		controlPlaneParityContext(versionMap, "Normal", map[string]any{}),
		manifests.NodeInput{NodeName: "master-0", NodeIP: "10.0.0.1"},
	)
	require.NoError(t, err)
	require.NotEmpty(t, bundle)
	require.Equal(t, "etcd.yaml", bundle[0].Name, "etcd has to be written before anything that talks to it")
}

func controlPlaneParityContext(versionMap map[string]any, runType string, apiserver map[string]any) map[string]any {
	data := cloneContext(versionMap)
	data["runType"] = runType
	data["nodeName"] = "master-0"
	data["nodeIP"] = "10.0.0.1"
	data["nodesCount"] = 3
	data["registry"] = map[string]any{
		"address": "registry.deckhouse.io",
		"path":    "/deckhouse/ce",
	}
	data["images"] = map[string]any{
		"controlPlaneManager": map[string]any{
			"etcd":                  "sha256:1111",
			"kubeApiserver":         "sha256:2222",
			"kubeControllerManager": "sha256:3333",
			"kubeScheduler":         "sha256:4444",
		},
	}
	data["clusterConfiguration"] = map[string]any{
		"clusterDomain":           "cluster.local",
		"clusterType":             "Cloud",
		"kubernetesVersion":       "1.34",
		"podSubnetCIDR":           "10.111.0.0/16",
		"podSubnetNodeCIDRPrefix": "24",
		"serviceSubnetCIDR":       "10.222.0.0/16",
	}
	data["apiserver"] = apiserver
	data["scheduler"] = map[string]any{}
	data["settings"] = map[string]any{
		"resourcesRequests": map[string]any{"milliCPU": 1024, "memoryBytes": 536870912},
	}
	data["etcd"] = map[string]any{"existingCluster": runType == "Normal"}
	return data
}

// cloneContext copies the top level of the context. Both renderers write into the map they are
// given — dhctl's engine adds a Files key, the package adds the node placeholders — so each run
// gets its own.
func cloneContext(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func repoRootForTest(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "candi", "version_map.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "candi/version_map.yml not found in any parent directory")
		dir = parent
	}
}
