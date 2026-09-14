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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/network"
)

func networkTestClient(t *testing.T, objects ...runtime.Object) *fake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	gvk := network.ModuleConfigGVK()
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind("ModuleConfigList"), &unstructured.UnstructuredList{})
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...)
}

func controlPlaneManagerMC(t *testing.T, podSubnetCIDR, serviceSubnetCIDR string) *unstructured.Unstructured {
	t.Helper()
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(network.ModuleConfigGVK())
	mc.SetName(network.ModuleConfigName)
	group := map[string]interface{}{}
	if podSubnetCIDR != "" {
		group["podSubnetCIDR"] = podSubnetCIDR
	}
	if serviceSubnetCIDR != "" {
		group["serviceSubnetCIDR"] = serviceSubnetCIDR
	}
	if len(group) > 0 {
		require.NoError(t, unstructured.SetNestedMap(mc.Object, group, "spec", "settings", "network"))
	}
	return mc
}

func networkTestSecret(configuration string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ClusterConfigSecretName, Namespace: ClusterConfigSecretNamespace},
		Data:       map[string][]byte{clusterConfigSecretKey: []byte(configuration)},
	}
}

// The two CIDRs are being migrated to ModuleConfig control-plane-manager. Every reader of the
// subnets goes through ReadClusterConfiguration, so the precedence is checked once, here: the CAPI
// Cluster's clusterNetwork, the MCM machine class and the provider templates must all describe the
// network the control plane actually runs with.
func TestReadClusterConfiguration_NetworkPrecedence(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		moduleConfig  *unstructured.Unstructured
		configuration string
		wantPod       string
		wantService   string
	}{
		{
			name:          "ClusterConfiguration only",
			configuration: "podSubnetCIDR: 10.111.0.0/16\nserviceSubnetCIDR: 10.222.0.0/16\n",
			wantPod:       "10.111.0.0/16",
			wantService:   "10.222.0.0/16",
		},
		{
			name:          "ModuleConfig only",
			moduleConfig:  controlPlaneManagerMC(t, "10.111.0.0/16", "10.222.0.0/16"),
			configuration: "clusterDomain: cluster.local\n",
			wantPod:       "10.111.0.0/16",
			wantService:   "10.222.0.0/16",
		},
		{
			name:          "ModuleConfig wins over ClusterConfiguration",
			moduleConfig:  controlPlaneManagerMC(t, "10.111.0.0/16", "10.222.0.0/16"),
			configuration: "podSubnetCIDR: 10.99.0.0/16\nserviceSubnetCIDR: 10.88.0.0/16\n",
			wantPod:       "10.111.0.0/16",
			wantService:   "10.222.0.0/16",
		},
		{
			name:          "ModuleConfig sets one field only",
			moduleConfig:  controlPlaneManagerMC(t, "10.111.0.0/16", ""),
			configuration: "podSubnetCIDR: 10.99.0.0/16\nserviceSubnetCIDR: 10.88.0.0/16\n",
			wantPod:       "10.111.0.0/16",
			wantService:   "10.88.0.0/16",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			objects := []runtime.Object{networkTestSecret(testCase.configuration)}
			if testCase.moduleConfig != nil {
				objects = append(objects, testCase.moduleConfig)
			}

			got, err := ReadClusterConfiguration(t.Context(), networkTestClient(t, objects...).Build())
			require.NoError(t, err)
			require.Equal(t, testCase.wantPod, got.PodSubnetCIDR)
			require.Equal(t, testCase.wantService, got.ServiceSubnetCIDR)
		})
	}
}
