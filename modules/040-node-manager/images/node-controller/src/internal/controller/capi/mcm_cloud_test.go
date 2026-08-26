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

package capi

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
	"github.com/deckhouse/node-controller/internal/testenv"
)

func TestInstanceClassSpot(t *testing.T) {
	t.Run("spot true", func(t *testing.T) {
		resolved := derived_status.ResolvedNodeGroup{InstanceClass: map[string]interface{}{"spot": true}}
		assert.True(t, instanceClassSpot(resolved))
	})
	t.Run("spot false", func(t *testing.T) {
		resolved := derived_status.ResolvedNodeGroup{InstanceClass: map[string]interface{}{"spot": false}}
		assert.False(t, instanceClassSpot(resolved))
	})
	t.Run("no spot key", func(t *testing.T) {
		resolved := derived_status.ResolvedNodeGroup{InstanceClass: map[string]interface{}{}}
		assert.False(t, instanceClassSpot(resolved))
	})
	t.Run("no instanceClass", func(t *testing.T) {
		assert.False(t, instanceClassSpot(derived_status.ResolvedNodeGroup{}))
	})
	t.Run("instanceClass resolved to null", func(t *testing.T) {
		assert.False(t, instanceClassSpot(derived_status.ResolvedNodeGroup{CloudProcessed: true, InstanceClass: nil}))
	})
}

func TestDecodeCloudProviderSecret(t *testing.T) {
	data := map[string][]byte{
		"type":             []byte(`"aws"`),
		"region":           []byte(`"eu-west-1"`),
		"machineClassKind": []byte(`"AWSMachineClass"`),
		"aws":              []byte(`{"keyName":"kn","instances":{"ami":"ami-1"}}`),
		"plainString":      []byte(`not-json`),
	}
	tree := decodeCloudProviderSecret(data)
	assert.Equal(t, "aws", tree["type"])
	assert.Equal(t, "eu-west-1", tree["region"])
	assert.Equal(t, "AWSMachineClass", tree["machineClassKind"])
	aws, ok := tree["aws"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "kn", aws["keyName"])
	// Non-JSON values fall back to the raw string, matching decodeSecretData.
	assert.Equal(t, "not-json", tree["plainString"])
}

func TestReconcileCloudMCMs_NoCloudInstances(t *testing.T) {
	r := &MachineDeploymentReconciler{}
	ng := &deckhousev1.NodeGroup{}
	assert.NoError(t, r.reconcileCloudMCMs(context.Background(), ng, derived_status.ResolvedNodeGroup{}, ""))
}

// The clusterUUID, the zone and the secret name are the fixture and the golden of
// the helm template test (template_tests/module_test.go:46,152,802), which asserts
// the machine-class Secret under exactly this name.
const (
	helmClusterUUID     = "f49dd1c3-a63a-4565-a06c-625e35587eab"
	helmZone            = "zonea"
	helmZoneSecretName  = "worker-02320933"
	yandexMCMConfigPath = "../../../../../../../030-cloud-provider-yandex/cloud-instance-manager/config-for-machine-controller-manager.yaml"
	yandexMachineClass  = "../../../../../../../030-cloud-provider-yandex/cloud-instance-manager/machine-class.yaml"
	yandexNetworksPath  = "../../../../../../../030-cloud-provider-yandex/candi/bashible/bootstrap-networks.sh.tpl"
)

// The Secret machine-controller-manager reads a machine's cloud-init and its cloud
// credentials from. Both halves are asserted: what the bashible render put in
// userData, and what the provider fragment put next to it.
func TestApplyMachineClassSecret(t *testing.T) {
	config, err := os.ReadFile(yandexMCMConfigPath)
	require.NoError(t, err, "provider config-for-machine-controller-manager.yaml must exist")
	r := machineClassSecretReconciler(t, config)
	resolved := derived_status.ResolvedNodeGroup{Name: "worker", NodeType: deckhousev1.NodeTypeCloudEphemeral}

	// The two calls reconcileCloudMCMs makes: the cloud-init once for the group, the
	// Secret once per zone.
	userData, err := r.machineClassUserData(t.Context(), resolved)
	require.NoError(t, err)
	require.NoError(t, r.applyMachineClassSecret(t.Context(),
		resolved.Name, "yandex", helmZoneSecretName, userData, yandexRenderContext()))

	secret := &corev1.Secret{}
	require.NoError(t, r.Client.Get(t.Context(), types.NamespacedName{
		Namespace: common.MachineNamespace, Name: helmZoneSecretName,
	}, secret))

	written := string(secret.Data["userData"])
	assert.True(t, strings.HasPrefix(written, "#cloud-config\n"), "userData is a cloud-init document")
	// machine-controller-manager substitutes a token of its own per machine
	// (pkg/util/provider/machinecontroller/userdata.go:29), so the placeholder must
	// reach the node file verbatim — a minted token here would expire in 4 hours.
	assert.Contains(t, written, "\n- path: /var/lib/bashible/bootstrap-token\n  content: <<BOOTSTRAP_TOKEN>>\n")
	// The UUID reaches the script only through a candi template
	// (01-bootstrap-prerequisites.sh.tpl:26), so it is proof the ConfigMap-delivered
	// templates were rendered rather than a literal being copied.
	assert.Contains(t, written, `PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID="`+helmClusterUUID+`"`)
	// ip_in_subnet is defined in the yandex bootstrap-networks.sh.tpl and nowhere else
	// in the repository. It reaches userData only when the cloud-provider Secret named
	// the provider (01-bootstrap-prerequisites.sh.tpl:40 skips the block otherwise) and
	// the script was found under the key templateKey maps its path to — the largest
	// block of a cloud node's script.
	assert.Contains(t, written, "function ip_in_subnet(){")
	// The prerequisites template strips the network script's own shebang before
	// inlining it, so a raw copy of the file would leave a second one here.
	assert.Equal(t, 1, strings.Count(written, "#!/bin/bash\n"), "the inlined network script keeps no shebang")

	// helm wrote the provider fragment under `data:`, whose values the apiserver
	// base64-decodes; the Go client encodes Data itself, so the b64enc of the
	// template has to be undone or MCM authenticates with a double-encoded key.
	assert.Equal(t, []byte("myfolder"), secret.Data["folderID"])
	assert.Equal(t, []byte(`{"id":"sa"}`), secret.Data["serviceAccountJSON"])

	// node-group is not one of helm's labels: it is what a prune of the Secrets left by
	// a removed zone would select on.
	assert.Equal(t, map[string]string{
		"heritage": "deckhouse", "module": "node-manager", "node-group": "worker",
	}, secret.Labels)
}

// The Secret name is a contract with every provider's machine-class.yaml: a
// MachineClass finds its cloud-init through secretRef, and the two names are
// computed twice — in Go by sha256Hash, in the template by sprig's sha256sum.
func TestMachineClassSecretNameMatchesSecretRef(t *testing.T) {
	tmpl, err := os.ReadFile(yandexMachineClass)
	require.NoError(t, err, "provider machine-class.yaml must exist")

	rendered, err := machineclass.RenderMachineClass(tmpl, yandexRenderContext())
	require.NoError(t, err)
	mc := map[string]interface{}{}
	require.NoError(t, sigsyaml.Unmarshal(rendered, &mc))

	secretRef, found, err := unstructured.NestedString(mc, "spec", "secretRef", "name")
	require.NoError(t, err)
	require.True(t, found, "MachineClass must reference its secret")

	assert.Equal(t, fmt.Sprintf("worker-%s", sha256Hash(helmClusterUUID+helmZone)), secretRef)
	assert.Equal(t, helmZoneSecretName, secretRef, "and it is the name helm computed for these inputs")
}

// yandexRenderContext is the context reconcileCloudMCMs builds for the zone loop,
// filled for yandex: the provider reads the credentials from .Values and the
// MachineClass fields from .nodeGroup and .zoneName.
func yandexRenderContext() map[string]interface{} {
	return map[string]interface{}{
		"Values": map[string]interface{}{
			"global": map[string]interface{}{
				"discovery": map[string]interface{}{"clusterUUID": helmClusterUUID, "podSubnet": "10.111.0.0/16"},
			},
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": map[string]interface{}{
						"type": "yandex",
						"yandex": map[string]interface{}{
							"folderID":                    "myfolder",
							"serviceAccountJSON":          `{"id":"sa"}`,
							"region":                      "ru-central1",
							"sshKey":                      "ssh-ed25519 AAAA",
							"nodeNetworkCIDR":             "10.222.0.0/16",
							"shouldAssignPublicIPAddress": false,
							"instanceClassDefaults":       map[string]interface{}{"imageID": "fd8default"},
							"zoneToSubnetIdMap":           map[string]interface{}{helmZone: "e9bsubnet"},
						},
					},
				},
			},
		},
		"nodeGroup": map[string]interface{}{
			"name":          "worker",
			"nodeType":      "CloudEphemeral",
			"instanceClass": map[string]interface{}{"cores": 4, "memory": 8192},
		},
		"zoneName": helmZone,
	}
}

// machineClassSecretReconciler builds a reconciler over a fake cluster carrying what
// the machine-class Secret is rendered from: the candi templates plus the yandex
// network script, the image digests, the cluster UUID, the apiserver endpoints, the
// cloud-provider discovery Secret and the provider's MCM template Secret.
func machineClassSecretReconciler(t *testing.T, providerConfig []byte) *MachineDeploymentReconciler {
	t.Helper()

	templates := testenv.BootstrapTemplatesConfigMapFor(t)
	// Left out of the shared fixture because it lives next to its cloud-provider module;
	// the chart globs it in under this key (bootstrap-templates-cm.yaml:16-18).
	networks, err := os.ReadFile(yandexNetworksPath)
	require.NoError(t, err, "provider bootstrap-networks.sh.tpl must exist")
	templates.Data["bootstrap-networks-yandex.sh.tpl"] = string(networks)

	return fakeReconciler(t,
		templates,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: clusterUUIDConfigMapNS, Name: clusterUUIDConfigMapName},
			Data:       map[string]string{"cluster-uuid": helmClusterUUID},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: common.MachineNamespace, Name: "bashible-apiserver-files"},
			Data: map[string]string{"images_digests.json": `{"registrypackages":{"jq171":"sha256:jq",` +
				`"d8Curl891":"sha256:curl","tailLog":"sha256:tail","rppGet":"sha256:rpp"}}`},
		},
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"},
			Endpoints:  []discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}}},
			Ports:      []discoveryv1.EndpointPort{{Name: ptr("https"), Port: ptr(int32(6443))}},
		},
		// Where Input.Provider comes from: without it the render drops the provider
		// network block entirely.
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: cloudProviderSecretNamespace, Name: cloudProviderSecretName},
			Data: map[string][]byte{
				"type":         []byte(`"yandex"`),
				"sshPublicKey": []byte(`"ssh-ed25519 AAAA"`),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: providerTemplateSecretNamespace, Name: "d8-cloud-provider-yandex-mcm"},
			Data:       map[string][]byte{"config-for-machine-controller-manager.yaml": providerConfig},
		},
	)
}
