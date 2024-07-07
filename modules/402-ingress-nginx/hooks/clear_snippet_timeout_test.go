/*
Copyright 2024 Flant JSC

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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: ingress-nginx :: hooks :: clear_snippet_timeout ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.9", "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	Context("Ingress exists", func() {
		It("Should comment", func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressYAML(
				"test",
				"default",
				"proxy_connect_timeout 60s\nproxy_read_timeout 30s\nproxy_write_timeout 30s")))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesResource("Ingress", "default", "test")
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io\\/auth-snippet").Str).
				To(Equal("#proxy_connect_timeout 60s\nproxy_read_timeout 30s\nproxy_write_timeout 30s"))
			Expect(ingress.Field("metadata.annotations.other").Str).To(Equal("someotherannotations"))
		})
		It("Should do nothing", func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressYAML(
				"test",
				"default",
				"proxy_read_timeout 30s\nproxy_write_timeout 30s")))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesResource("Ingress", "default", "test")
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io\\/auth-snippet").Str).
				To(Equal("proxy_read_timeout 30s\nproxy_write_timeout 30s"))
			Expect(ingress.Field("metadata.annotations.other").Str).To(Equal("someotherannotations"))
		})
	})
})

func ingressYAML(name, namespace, authSnippet string) string {
	ing := netv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-snippet": authSnippet,
				"other": "someotherannotations",
			},
		},
		Spec: netv1.IngressSpec{
			IngressClassName: pointer.String("nginx"),
			Rules:            []netv1.IngressRule{},
		},
	}
	marshaled, _ := yaml.Marshal(ing)
	return string(marshaled)
}
