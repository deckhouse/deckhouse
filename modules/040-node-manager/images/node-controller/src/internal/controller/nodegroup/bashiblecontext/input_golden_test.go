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
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
)

var updateGoldens = flag.Bool("update-goldens", false, "rewrite the input.yaml golden under testdata")

const instanceClassKind = "D8TestInstanceClass"

func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, discoveryv1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	gv := schema.GroupVersion{Group: "deckhouse.io", Version: "v1alpha1"}
	scheme.AddKnownTypeWithName(gv.WithKind(instanceClassKind), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gv.WithKind(instanceClassKind+"List"), &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(ngcommon.MCMMachineDeploymentGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(ngcommon.MCMMachineDeploymentGVK.GroupVersion().WithKind("MachineDeploymentList"), &unstructured.UnstructuredList{})

	return scheme
}

func goldenInstanceClass(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: instanceClassKind})
	obj.SetName(name)
	obj.Object["spec"] = map[string]interface{}{
		"platformID":   "standard-v3",
		"cores":        int64(4),
		"memory":       int64(8192),
		"coreFraction": int64(100),
		"diskType":     "network-ssd",
		"imageID":      "img-abc",
	}
	return obj
}

func goldenNodeGroups() []*v1.NodeGroup {
	return []*v1.NodeGroup{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "static-worker"},
			Spec: v1.NodeGroupSpec{
				NodeType: v1.NodeTypeStatic,
				CRI:      &v1.CRISpec{Type: v1.CRITypeContainerd},
				NodeTemplate: &v1.NodeTemplate{
					Labels: map[string]string{"role": "worker"},
					Taints: []corev1.Taint{{Key: "dedicated", Value: "worker", Effect: corev1.TaintEffectNoExecute}},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "cloud-worker",
				Annotations: map[string]string{"manual-rollout-id": "rollout-1"},
			},
			Spec: v1.NodeGroupSpec{
				NodeType: v1.NodeTypeCloudEphemeral,
				CloudInstances: &v1.CloudInstancesSpec{
					ClassReference: v1.ClassReference{Kind: instanceClassKind, Name: "worker"},
					MinPerZone:     1,
					MaxPerZone:     3,
					Zones:          []string{"ru-central1-a"},
				},
				NodeDrainTimeoutSecond: ptr.To(300),
			},
		},
	}
}

func newGoldenReconciler(t *testing.T) *Reconciler {
	t.Helper()

	objs := []runtime.Object{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-dns",
				Namespace: kubeSystemNS,
				Labels:    map[string]string{dnsAppLabel: "kube-dns"},
			},
			Spec: corev1.ServiceSpec{ClusterIP: "10.222.0.10"},
		},
		endpointSlice([]string{"10.0.0.1", "10.0.0.2"}, "https", 6443),
		configMap(kubeSystemNS, clusterUUIDConfigMapName, map[string]string{clusterUUIDKey: "3d9a3d3f-0000-4000-8000-000000000001"}),
		configMap(versionInfoCMNS, versionInfoCMName, map[string]string{
			"data.json": `{"channel":"stable","version":"v1.70.0","edition":"EE"}`,
		}),
		secret(kubeSystemNS, clusterConfigSecretName, map[string][]byte{
			clusterConfigKey: []byte("kubernetesVersion: \"1.32\"\ndefaultCRI: Containerd\npodSubnetNodeCIDRPrefix: \"24\"\nclusterDomain: cluster.local\nproxy:\n  httpProxy: http://proxy.example.com\n  noProxy:\n  - 10.0.0.0/8\n"),
		}),
		secret(kubeSystemNS, "d8-static-cluster-configuration", map[string][]byte{
			"static-cluster-configuration.yaml": []byte("internalNetworkCIDRs:\n- 172.18.200.0/24\n"),
		}),
		secret(kubeSystemNS, cloudProviderSecretName, map[string][]byte{
			"type":              []byte(`"yandex"`),
			"instanceClassKind": []byte(`"` + instanceClassKind + `"`),
			"machineClassKind":  []byte(`"YandexMachineClass"`),
			"region":            []byte(`"ru-central1"`),
			"zones":             []byte(`["ru-central1-a","ru-central1-b"]`),
		}),
		secret(cloudInstanceManagerNS, packagesProxyTokenSecretName, map[string][]byte{"token": []byte("packages-proxy-token")}),
		secret(kubeSystemNS, apiProxyCertSecretName, map[string][]byte{"crt": []byte("PROXY-CERT"), "key": []byte("PROXY-KEY")}),
		secret(kubeSystemNS, controlPlaneArgsSecretName, map[string][]byte{
			"arguments.json":    []byte(`{"nodeMonitorGracePeriod":40}`),
			"featureGates.json": []byte(`{"kubelet":["RotateKubeletServerCertificate"]}`),
		}),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: kubeSystemNS,
				Name:      "bootstrap-token-abcdef",
				Labels:    map[string]string{bootstrapTokenNGLabel: "cloud-worker"},
			},
			Type: corev1.SecretTypeBootstrapToken,
			Data: map[string][]byte{"token-id": []byte("abcdef"), "token-secret": []byte("0123456789abcdef")},
		},
		goldenInstanceClass("worker"),
	}
	for _, ng := range goldenNodeGroups() {
		objs = append(objs, ng)
	}

	c := fake.NewClientBuilder().WithScheme(goldenScheme(t)).WithRuntimeObjects(objs...).Build()
	return &Reconciler{
		Client:        c,
		Context:       &Service{Client: c, RootCAFile: filepath.Join("testdata", "ca.crt")},
		DerivedStatus: &derived_status.Service{Client: c},
	}
}

// TestAssemble_InputYAMLGolden pins the whole published document, not just the NodeGroup
// elements: bashible-apiserver hashes the parsed input.yaml into every node's configuration
// checksum, so a key that appears or disappears rebuilds every bashible step in the cluster.
// updateEpoch is the one value that legitimately moves (it is a function of wall time) and is
// masked.
func TestAssemble_InputYAMLGolden(t *testing.T) {
	r := newGoldenReconciler(t)
	require.NoError(t, r.Assemble(t.Context()))

	published := &corev1.Secret{}
	require.NoError(t, r.Client.Get(t.Context(),
		types.NamespacedName{Namespace: secretNamespace, Name: secretName}, published))

	var got map[string]interface{}
	require.NoError(t, yaml.Unmarshal(published.Data[secretInputKey], &got))
	maskUpdateEpoch(t, got)

	path := filepath.Join("testdata", "input.golden.yaml")
	if *updateGoldens {
		raw, err := yaml.Marshal(got)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, raw, 0o644))
	}

	golden, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden %s: regenerate with go test ./internal/controller/nodegroup/bashiblecontext/ -run InputYAMLGolden -update-goldens", path)

	var want map[string]interface{}
	require.NoError(t, yaml.Unmarshal(golden, &want))

	assert.Equal(t, want, got)
}

func maskUpdateEpoch(t *testing.T, input map[string]interface{}) {
	t.Helper()

	nodeGroups, ok := input["nodeGroups"].([]interface{})
	require.True(t, ok, "input.yaml must carry nodeGroups")

	for _, item := range nodeGroups {
		element, ok := item.(map[string]interface{})
		require.True(t, ok)
		epoch, ok := element["updateEpoch"].(string)
		require.True(t, ok, "every element must carry updateEpoch")
		require.NotEmpty(t, epoch)
		element["updateEpoch"] = "<epoch>"
	}
}
