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

package clusterprefix

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func scheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	gvk := ModuleConfigGVK()
	s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gvk.GroupVersion().WithKind("ModuleConfigList"), &unstructured.UnstructuredList{})
	return s
}

func globalMC(prefix string) *unstructured.Unstructured {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(ModuleConfigGVK())
	mc.SetName(GlobalModuleConfigName)
	if prefix != "" {
		_ = unstructured.SetNestedField(mc.Object, prefix, "spec", "settings", "prefix")
	}
	return mc
}

func ccSecret(cloudPrefix string) *corev1.Secret {
	y := "clusterType: Cloud\n"
	if cloudPrefix != "" {
		y += "cloud:\n  provider: Yandex\n  prefix: " + cloudPrefix + "\n"
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigSecretName, Namespace: clusterConfigSecretNamespace},
		Data:       map[string][]byte{clusterConfigSecretKey: []byte(y)},
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name string
		objs []client.Object
		want string
	}{
		{name: "global MC prefix wins over cloud.prefix", objs: []client.Object{globalMC("from-mc"), ccSecret("from-cloud")}, want: "from-mc"},
		{name: "falls back to cloud.prefix when MC prefix empty", objs: []client.Object{globalMC(""), ccSecret("from-cloud")}, want: "from-cloud"},
		{name: "falls back to cloud.prefix when MC absent", objs: []client.Object{ccSecret("from-cloud")}, want: "from-cloud"},
		{name: "empty when neither set", objs: []client.Object{globalMC(""), ccSecret("")}, want: ""},
		{name: "empty when nothing exists", objs: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(tt.objs...).Build()
			got, err := Resolve(context.Background(), cl)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Resolve = %q, want %q", got, tt.want)
			}
		})
	}
}
