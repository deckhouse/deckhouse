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

package derived_status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func serviceWithMC(t *testing.T, objs ...client.Object) *Service {
	t.Helper()
	scheme := newTestScheme(t)
	scheme.AddKnownTypeWithName(moduleConfigGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(moduleConfigGVK.GroupVersion().WithKind("ModuleConfigList"), &unstructured.UnstructuredList{})
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Service{Client: c}
}

func nodeManagerMC(defaultCRI string) *unstructured.Unstructured {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(moduleConfigGVK)
	mc.SetName(nodeManagerModuleConfigName)
	if defaultCRI != "" {
		_ = unstructured.SetNestedField(mc.Object, defaultCRI, "spec", "settings", "defaultCRI")
	}
	return mc
}

func ccSecretWithCRI(defaultCRI string) *corev1.Secret {
	y := "kind: ClusterConfiguration\n"
	if defaultCRI != "" {
		y += "defaultCRI: " + defaultCRI + "\n"
	}
	return testSecret(clusterConfigSecretNamespace, clusterConfigSecretName, map[string][]byte{
		"cluster-configuration.yaml": []byte(y),
	})
}

func TestReadDefaultCRIFromModuleConfig(t *testing.T) {
	tests := []struct {
		name string
		objs []client.Object
		want string
	}{
		{name: "reads spec.settings.defaultCRI", objs: []client.Object{nodeManagerMC("NotManaged")}, want: "NotManaged"},
		{name: "empty when field absent", objs: []client.Object{nodeManagerMC("")}, want: ""},
		{name: "empty when ModuleConfig absent", objs: nil, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serviceWithMC(t, tt.objs...)
			assert.Equal(t, tt.want, s.readDefaultCRIFromModuleConfig(context.Background()))
		})
	}
}

// The node-manager ModuleConfig is the new home for defaultCRI; it must take
// precedence over the deprecated ClusterConfiguration.defaultCRI, matching the
// webhook so admission and rendering resolve the same CRI.
func TestCompute_DefaultCRIPrecedence(t *testing.T) {
	ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic}}
	ng.Name = "ng"

	tests := []struct {
		name string
		objs []client.Object
		want string
	}{
		{
			name: "explicit ModuleConfig value wins over ClusterConfiguration",
			objs: []client.Object{ccSecretWithCRI("Containerd"), nodeManagerMC("NotManaged")},
			want: "NotManaged",
		},
		{
			name: "explicit ModuleConfig Containerd wins over ClusterConfiguration NotManaged",
			objs: []client.Object{ccSecretWithCRI("NotManaged"), nodeManagerMC("Containerd")},
			want: "Containerd",
		},
		{
			name: "no ModuleConfig field falls back to ClusterConfiguration",
			objs: []client.Object{ccSecretWithCRI("NotManaged"), nodeManagerMC("")},
			want: "NotManaged",
		},
		{
			name: "no ModuleConfig at all uses ClusterConfiguration",
			objs: []client.Object{ccSecretWithCRI("NotManaged")},
			want: "NotManaged",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serviceWithMC(t, tt.objs...)
			res, err := s.compute(context.Background(), ng, map[string]interface{}{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, res.CRIType)
		})
	}
}
