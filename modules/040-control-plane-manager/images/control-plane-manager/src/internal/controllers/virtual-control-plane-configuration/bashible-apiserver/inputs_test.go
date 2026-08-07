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

package bashibleapiserver

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// goldenParams are the facts both goldens are built from. The same values are hard-coded in the
// node-manager hook test, which asserts that the tenant reassembles context.golden.yaml out of
// external-inputs.golden.yaml plus what it reads from its own cluster. Change one side and that
// test fails, which is the point: the tenant must keep producing the document this controller
// used to write.
func goldenParams() ExternalInputsParams {
	return ExternalInputsParams{
		VCP: &controlplanev1alpha1.VirtualControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "golden", Namespace: "vcp-golden"},
			Spec:       controlplanev1alpha1.VirtualControlPlaneSpec{KubernetesVersion: "1.31"},
		},
		CA:           []byte("-----BEGIN CERTIFICATE-----\nVCP-CA\n-----END CERTIFICATE-----\n"),
		JoinToken:    "token",
		ClusterUUID:  "11111111-2222-3333-4444-555555555555",
		APIHost:      "api.golden.example.com",
		PackagesHost: "packages.golden.example.com",
		RPPToken:     "rpp-token",
		APIServerProxyCerts: ContextAPIServerProxyCerts{
			Crt: "-----BEGIN CERTIFICATE-----\nPROXY\n-----END CERTIFICATE-----\n",
			Key: "-----BEGIN RSA PRIVATE KEY-----\nPROXY\n-----END RSA PRIVATE KEY-----\n",
		},
	}
}

var updateGoldens = flag.Bool("update-goldens", false, "rewrite the goldens under testdata")

func requireGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name)
	if *updateGoldens {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}

	require.Equal(t, string(readGolden(t, name)), got)
}

// readGolden also serves context.golden.yaml, which has no generator any more: it is the last
// output of the writer this controller used to run, frozen on purpose. -update-goldens does not
// touch it, and nothing should — both this package and the node-manager hook check themselves
// against those bytes because they are what the fleet was bootstrapped with.
func readGolden(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", name)
	golden, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden %s: regenerate with go test ./internal/controllers/virtual-control-plane-configuration/bashible-apiserver/ -update-goldens", path)
	return golden
}

func TestBuildExternalInputsYAMLGolden(t *testing.T) {
	out, err := BuildExternalInputsYAML(goldenParams())
	require.NoError(t, err)
	requireGolden(t, "external-inputs.golden.yaml", out)
}

// TestExternalInputsMatchContextInput is the host half of the anti-drift guard. Every field the
// tenant is handed must carry exactly the value the removed writer put in the context, and the
// set of context fields the tenant is expected to compute for itself must stay the one
// documented in externalInputs. The tenant half — that those inputs really do reassemble into
// the same document — is TestBashibleContextVCPMatchesControlPlaneManager in node-manager.
func TestExternalInputsMatchContextInput(t *testing.T) {
	inputsYAML, err := BuildExternalInputsYAML(goldenParams())
	require.NoError(t, err)

	var contextDoc, inputsDoc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(readGolden(t, "context.golden.yaml"), &contextDoc))
	require.NoError(t, yaml.Unmarshal([]byte(inputsYAML), &inputsDoc))

	// Contract bookkeeping, not a context field.
	delete(inputsDoc, "version")

	for key, want := range inputsDoc {
		require.Contains(t, contextDoc, key, "inputs publish %q, which the context does not have", key)
		require.Equal(t, contextDoc[key], want, "inputs and context disagree on %q", key)
	}

	var tenantComputed []string
	for key := range contextDoc {
		if _, ok := inputsDoc[key]; !ok {
			tenantComputed = append(tenantComputed, key)
		}
	}
	// clusterDomain is in the tenant's own d8-cluster-configuration, written by
	// buildTargetTenantClusterConfigurationSecret from the same constant.
	require.ElementsMatch(t, []string{"clusterDomain"}, tenantComputed)
}
