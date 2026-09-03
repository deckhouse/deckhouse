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

package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The goldens come from the helm render of the node-group templates, on the values kept
// verbatim in testdata/helm-values.yaml. Taking them from our own render would be
// pointless: such a golden guards nothing but its own drift — and the templates are gone,
// so there is no oracle to regenerate from either. Every Input below mirrors the values
// its golden was rendered from.
func TestRenderMatchesHelmGoldens(t *testing.T) {
	files := frozenFiles(t)

	cases := []struct {
		name   string
		golden string
		render func(Input) ([]byte, error)
		in     Input
	}{
		{
			name:   "static node cloud-config",
			golden: "static-instances-cloud-config.txt",
			render: RenderCloudConfig,
			in:     staticInput(files),
		},
		{
			name:   "static node bootstrap.sh",
			golden: "static-instances-bootstrap-sh.txt",
			render: RenderStaticScript,
			in:     staticInput(files),
		},
		{
			name:   "node without staticInstances gets no tail-log",
			golden: "cloud-permanent-bootstrap-sh.txt",
			render: RenderStaticScript,
			in:     cloudPermanentInput(files),
		},
		{
			name:   "capi cloud-config",
			golden: "capi-yandex-value.txt",
			render: RenderCAPICloudConfig,
			in:     capiInput(files),
		},
		{
			name:   "mcm userData carries the token placeholder",
			golden: "mcm-aws-userData.txt",
			render: RenderCloudConfig,
			in:     mcmInput(files),
		},
		{
			name:   "azure userData mounts the ephemeral disk",
			golden: "azure-userData.txt",
			render: RenderCloudConfig,
			in:     azureInput(files),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", "golden", tc.golden))
			require.NoError(t, err)

			got, err := tc.render(tc.in)
			require.NoError(t, err)

			assert.Equal(t, string(want), string(got))
		})
	}
}

func TestRenderScriptWithoutBashibleLibrary(t *testing.T) {
	in := staticInput(&Files{text: map[string]string{}})

	_, err := RenderScript(in)

	require.ErrorContains(t, err, "candi/bashible/lib.sh.tpl")
}

// frozenFiles loads the templates the goldens were rendered from, under the keys
// the ConfigMap uses, so the golden compares render to render, not delivery to
// delivery.
//
// They are copies under testdata/inputs, not the live files at the repository
// root, because the goldens embed their rendered content — static-instances
// inlines the whole of lib.sh. Reading the live files would turn an edit to
// candi/bashible into a red test in this package, with the helm oracle deleted
// and no honest way to regenerate. These are fixtures, not mirrors: drift from
// the live templates is expected and is not what this test is about. What it
// proves stays exact — on these inputs, this renderer emits helm's bytes.
//
// bb_node_ip.sh.tpl is here although no golden contains it: the prerequisites
// template pulls it in only under runType ClusterBootstrap, and runType is a key
// helm never sets. Offering the file proves that branch stays shut anyway.
func frozenFiles(t *testing.T) *Files {
	t.Helper()
	text := map[string]string{}
	for _, key := range []string{
		"lib.sh.tpl",
		"01-bootstrap-prerequisites.sh.tpl",
		"bb_node_ip.sh.tpl",
		"bootstrap-networks-aws.sh.tpl",
		"bootstrap-networks-yandex.sh.tpl",
	} {
		data, err := os.ReadFile(filepath.Join("testdata", "inputs", key))
		if err != nil {
			t.Fatalf("read %s: %v", key, err)
		}
		text[key] = string(data)
	}
	return &Files{text: text}
}

func staticInput(files *Files) Input {
	in := baseInput(files)
	in.NodeGroup = map[string]any{
		"name":     "worker",
		"nodeType": "Static",
		"staticInstances": map[string]any{
			"labelSelector": map[string]any{"matchLabels": map[string]any{"node-group": "worker"}},
		},
	}
	in.BootstrapToken = "myworker"
	return in
}

func cloudPermanentInput(files *Files) Input {
	in := baseInput(files)
	in.NodeGroup = map[string]any{"name": "worker", "nodeType": "CloudPermanent"}
	in.BootstrapToken = "myworker"
	return in
}

func capiInput(files *Files) Input {
	in := baseInput(files)
	in.NodeGroup = map[string]any{"name": "worker", "nodeType": "CloudEphemeral"}
	in.BootstrapToken = "myworker"
	in.SSHPublicKey = "ssh-rsa AAAA"
	in.Provider = "yandex"
	return in
}

// MCM substitutes the real token into userData itself, so the secret carries the
// placeholder literally (machine-controller-manager pkg/util/provider/
// machinecontroller/userdata.go).
func mcmInput(files *Files) Input {
	in := baseInput(files)
	in.NodeGroup = map[string]any{"name": "worker", "nodeType": "CloudEphemeral"}
	in.BootstrapToken = "<<BOOTSTRAP_TOKEN>>"
	in.Provider = "aws"
	return in
}

func azureInput(files *Files) Input {
	in := mcmInput(files)
	in.Provider = "azure"
	return in
}

func baseInput(files *Files) Input {
	return Input{
		APIServerEndpoints: []string{"10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"},
		ClusterMasterEndpoints: []map[string]any{
			{"address": "10.0.0.1", "kubeApiPort": int64(6443), "rppServerPort": int64(4219), "rppBootstrapServerPort": int64(4220)},
		},
		ClusterUUID: "deadbeef-dead-beef-dead-beefdeadbeef",
		Images: map[string]any{"registrypackages": map[string]any{
			"jq171": "sha256:jq", "d8Curl891": "sha256:curl", "tailLog": "sha256:tail", "rppGet": "sha256:rpp",
			"ec2DescribeTagsV001Flant3": "sha256:ec2",
		}},
		PackagesProxy: map[string]any{"token": "mytoken"},
		MingetB64:     "bWluZ2V0",
		KubernetesCA:  "myclusterca",
		Files:         files,
	}
}
