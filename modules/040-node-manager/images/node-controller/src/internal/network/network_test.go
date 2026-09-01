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

package network

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func scheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	gvk := ModuleConfigGVK()
	s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gvk.GroupVersion().WithKind("ModuleConfigList"), &unstructured.UnstructuredList{})
	return s
}

func moduleConfig(t *testing.T, network map[string]interface{}) *unstructured.Unstructured {
	t.Helper()
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(ModuleConfigGVK())
	mc.SetName(ModuleConfigName)
	if network != nil {
		if err := unstructured.SetNestedMap(mc.Object, network, "spec", "settings", "network"); err != nil {
			t.Fatalf("set network group: %v", err)
		}
	}
	return mc
}

func TestFromModuleConfig(t *testing.T) {
	tests := []struct {
		name string
		mc   *unstructured.Unstructured
		want Settings
	}{
		{
			name: "all three fields set",
			mc: moduleConfig(t, map[string]interface{}{
				"podSubnetCIDR":           "10.111.0.0/16",
				"serviceSubnetCIDR":       "10.222.0.0/16",
				"podSubnetNodeCIDRPrefix": "24",
			}),
			want: Settings{PodSubnetCIDR: "10.111.0.0/16", ServiceSubnetCIDR: "10.222.0.0/16", PodSubnetNodeCIDRPrefix: "24"},
		},
		{
			name: "network group absent",
			mc:   moduleConfig(t, nil),
			want: Settings{},
		},
		{
			name: "ModuleConfig absent",
			mc:   nil,
			want: Settings{},
		},
		{
			name: "non-string value dropped rather than coerced",
			mc:   moduleConfig(t, map[string]interface{}{"podSubnetNodeCIDRPrefix": int64(24)}),
			want: Settings{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme(t))
			if tt.mc != nil {
				builder = builder.WithObjects(tt.mc)
			}
			cl := builder.Build()

			got, err := FromModuleConfig(context.Background(), cl)
			if err != nil {
				t.Fatalf("FromModuleConfig: %v", err)
			}
			if got != tt.want {
				t.Fatalf("FromModuleConfig = %+v, want %+v", got, tt.want)
			}
		})
	}
}
