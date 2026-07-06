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

package bashiblecontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

// The always-present keys mirror the unconditional part of the helm define
// bashible_input_data; Build must emit exactly these plus whatever optional
// blocks apply, so bashible-apiserver never sees a partial context.
var mandatoryInputKeys = []string{
	"deckhouse", "podSubnetNodeCIDRPrefix", "clusterDomain", "clusterDNSAddress",
	"clusterUUID", "bootstrapTokens", "apiserverEndpoints", "clusterMasterEndpoints",
	"packagesProxy", "allowedBundles", "nodeGroups",
}

func TestBuild_MandatoryFieldsAlwaysPresent(t *testing.T) {
	// Empty cluster: only the mandatory (unconditional) keys must appear.
	s := newService(t)
	input := s.Build(context.Background(), Globals{}, nil)

	for _, k := range mandatoryInputKeys {
		assert.Contains(t, input, k, "mandatory input.yaml key %q must be present", k)
	}

	// Optional blocks gated off when their source is absent.
	assert.NotContains(t, input, "cloudProvider")
	assert.NotContains(t, input, "proxy")
	assert.NotContains(t, input, "apiserverProxyCerts")
	assert.NotContains(t, input, "kubernetesCA")
	assert.NotContains(t, input, "nodeStatusUpdateFrequency")
	assert.NotContains(t, input, "allowedKubeletFeatureGates")

	// Defaults mirror the template.
	deckhouse := input["deckhouse"].(map[string]interface{})
	assert.Equal(t, "unknown", deckhouse["channel"])
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", input["clusterUUID"])
	assert.Equal(t, allowedBundles, input["allowedBundles"])
}

func TestBuild_OptionalBlocksPopulated(t *testing.T) {
	s := newService(t,
		secret(kubeSystemNS, cloudProviderSecretName, map[string][]byte{"type": []byte(`"yandex"`)}),
		secret(kubeSystemNS, apiProxyCertSecretName, map[string][]byte{"crt": []byte("C"), "key": []byte("K")}),
		secret(kubeSystemNS, controlPlaneArgsSecretName, map[string][]byte{
			"arguments.json":    []byte(`{"nodeMonitorGracePeriod":40}`),
			"featureGates.json": []byte(`{"kubelet":["X"]}`),
		}),
		secret(cloudInstanceManagerNS, packagesProxyTokenSecretName, map[string][]byte{"token": []byte("tok")}),
	)

	globals := Globals{
		DeckhouseChannel: "stable",
		DeckhouseVersion: "v1.2.3",
		ClusterUUID:      "uuid-1",
		Proxy:            map[string]interface{}{"httpProxy": "http://p"},
	}
	blob := []map[string]interface{}{{"name": "worker", "nodeType": "CloudEphemeral"}}
	input := s.Build(context.Background(), globals, blob)

	assert.Equal(t, map[string]interface{}{"type": "yandex"}, input["cloudProvider"])
	assert.Equal(t, map[string]interface{}{"httpProxy": "http://p"}, input["proxy"])
	assert.Equal(t, map[string]interface{}{"crt": "C", "key": "K"}, input["apiserverProxyCerts"])
	assert.Equal(t, float64(10), input["nodeStatusUpdateFrequency"])
	assert.Equal(t, []string{"X"}, input["allowedKubeletFeatureGates"])
	assert.Equal(t, map[string]interface{}{"token": "tok"}, input["packagesProxy"])
	assert.Equal(t, blob, input["nodeGroups"])

	deckhouse := input["deckhouse"].(map[string]interface{})
	assert.Equal(t, "stable", deckhouse["channel"])
	assert.Equal(t, "v1.2.3", deckhouse["version"])
	assert.Equal(t, "uuid-1", input["clusterUUID"])
}

// Marshal must produce YAML that round-trips to the same value tree, confirming
// the payload is the sigs.k8s.io/yaml form bashible-apiserver unmarshals.
func TestMarshal_RoundTrips(t *testing.T) {
	s := newService(t)
	input := s.Build(context.Background(), Globals{ClusterUUID: "u"}, nil)

	raw, err := Marshal(input)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)
	assert.Contains(t, string(raw), "clusterUUID: u")
}

// WriteSecret upserts the bashible-apiserver-context Secret with the assembled
// input.yaml and the module labels, and re-writes cleanly on the second call
// (create then update path).
func TestWriteSecret_UpsertsInputYAML(t *testing.T) {
	s := newService(t,
		configMap(kubeSystemNS, clusterUUIDConfigMapName, map[string]string{clusterUUIDKey: "uuid-1"}),
		configMap(versionInfoCMNS, versionInfoCMName, map[string]string{
			"data.json": `{"channel":"stable","version":"v1.2.3","edition":"EE"}`,
		}),
	)
	blob := []map[string]interface{}{{"name": "worker", "nodeType": "CloudEphemeral"}}

	require.NoError(t, s.WriteSecret(context.Background(), blob))

	got := &corev1.Secret{}
	require.NoError(t, s.Client.Get(context.Background(),
		types.NamespacedName{Namespace: secretNamespace, Name: secretName}, got))

	assert.Equal(t, "deckhouse", got.Labels["heritage"])
	assert.Equal(t, "node-manager", got.Labels["module"])
	assert.Equal(t, "bashible-apiserver", got.Labels["app"])

	var parsed map[string]interface{}
	require.NoError(t, yaml.Unmarshal(got.Data[secretInputKey], &parsed))
	assert.Equal(t, "uuid-1", parsed["clusterUUID"])
	deckhouse := parsed["deckhouse"].(map[string]interface{})
	assert.Equal(t, "v1.2.3", deckhouse["version"])
	ngs := parsed["nodeGroups"].([]interface{})
	require.Len(t, ngs, 1)

	// Second write (update path) must succeed and overwrite the payload.
	cm := &corev1.ConfigMap{}
	require.NoError(t, s.Client.Get(context.Background(),
		types.NamespacedName{Namespace: kubeSystemNS, Name: clusterUUIDConfigMapName}, cm))
	cm.Data[clusterUUIDKey] = "uuid-2"
	require.NoError(t, s.Client.Update(context.Background(), cm))
	require.NoError(t, s.WriteSecret(context.Background(), nil))
	require.NoError(t, s.Client.Get(context.Background(),
		types.NamespacedName{Namespace: secretNamespace, Name: secretName}, got))
	require.NoError(t, yaml.Unmarshal(got.Data[secretInputKey], &parsed))
	assert.Equal(t, "uuid-2", parsed["clusterUUID"])
}
