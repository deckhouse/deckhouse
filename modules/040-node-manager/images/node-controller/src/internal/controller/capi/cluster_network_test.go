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
	"testing"

	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/network"
	"github.com/deckhouse/node-controller/internal/register"
)

func networkScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	gvk := network.ModuleConfigGVK()
	s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gvk.GroupVersion().WithKind("ModuleConfigList"), &unstructured.UnstructuredList{})
	return s
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

// clusterNetworkReconciler builds a ClusterReconciler with both Client and APIReader backed by the
// same fake client: readClusterConfiguration reads the Secret via APIReader and ModuleConfig via
// the cached Client, and a real cluster always agrees between the two.
func clusterNetworkReconciler(t *testing.T, objs ...client.Object) *ClusterReconciler {
	t.Helper()
	cl := fake.NewClientBuilder().WithScheme(networkScheme(t)).WithObjects(objs...).Build()
	return &ClusterReconciler{BaseWithReader: BaseWithReader{Base: register.Base{Client: cl}, APIReader: cl}}
}

// The two CIDRs are being migrated to ModuleConfig control-plane-manager, and the CAPI Cluster's
// clusterNetwork must agree with whichever value the control plane actually runs with — a stale
// read here would misconfigure the CNI's view of the cluster's own subnets.
func TestReadClusterConfiguration_NetworkPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		mc          *unstructured.Unstructured
		cc          string
		wantPod     string
		wantService string
	}{
		{
			name:        "ClusterConfiguration only",
			cc:          "podSubnetCIDR: 10.111.0.0/16\nserviceSubnetCIDR: 10.222.0.0/16\n",
			wantPod:     "10.111.0.0/16",
			wantService: "10.222.0.0/16",
		},
		{
			name:        "ModuleConfig only",
			mc:          controlPlaneManagerMC(t, "10.111.0.0/16", "10.222.0.0/16"),
			cc:          "clusterDomain: cluster.local\n",
			wantPod:     "10.111.0.0/16",
			wantService: "10.222.0.0/16",
		},
		{
			name:        "ModuleConfig wins over ClusterConfiguration",
			mc:          controlPlaneManagerMC(t, "10.111.0.0/16", "10.222.0.0/16"),
			cc:          "podSubnetCIDR: 10.99.0.0/16\nserviceSubnetCIDR: 10.88.0.0/16\n",
			wantPod:     "10.111.0.0/16",
			wantService: "10.222.0.0/16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []client.Object{clusterConfigSecret(tt.cc)}
			if tt.mc != nil {
				objs = append(objs, tt.mc)
			}
			r := clusterNetworkReconciler(t, objs...)

			got, err := r.readClusterConfiguration(t.Context())
			require.NoError(t, err)
			require.Equal(t, tt.wantPod, got.PodSubnetCIDR)
			require.Equal(t, tt.wantService, got.ServiceSubnetCIDR)
		})
	}
}

// readPodSubnet backs the MCM cloud provider template's podSubnet, which must match the CAPI
// Cluster's clusterNetwork.pods above — the same subnet resolved the same way.
func TestReadPodSubnet_NetworkPrecedence(t *testing.T) {
	tests := []struct {
		name string
		mc   *unstructured.Unstructured
		cc   string
		want string
	}{
		{name: "ClusterConfiguration only", cc: "podSubnetCIDR: 10.111.0.0/16\n", want: "10.111.0.0/16"},
		{
			name: "ModuleConfig wins over ClusterConfiguration",
			mc:   controlPlaneManagerMC(t, "10.111.0.0/16", ""),
			cc:   "podSubnetCIDR: 10.99.0.0/16\n",
			want: "10.111.0.0/16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []client.Object{clusterConfigSecret(tt.cc)}
			if tt.mc != nil {
				objs = append(objs, tt.mc)
			}
			cl := fake.NewClientBuilder().WithScheme(networkScheme(t)).WithObjects(objs...).Build()
			r := &MachineDeploymentReconciler{BaseWithReader: BaseWithReader{Base: register.Base{Client: cl}}}

			got, err := r.readPodSubnet(t.Context())
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
