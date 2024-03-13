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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: ingress-nginx :: hooks :: update_ingress_address ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.1", "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	Context("Service with non LoadBalancer type", func() {
		BeforeEach(func() {
			resources := []string{
				ingressNginxControllerYAML("test"),
				serviceYAML("test", "HostPort", "ip", "hostname"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should not set anything", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesGlobalResource("IngressNginxController", "test")
			Expect(ingress.Field("status.loadBalancer.hostname").Str).To(BeEmpty())
			Expect(ingress.Field("status.loadBalancer.ip").Str).To(BeEmpty())
		})
	})

	Context("Service with LoadBalancer type and with hostname", func() {
		BeforeEach(func() {
			resources := []string{
				ingressNginxControllerYAML("test2"),
				serviceYAML("test2", "LoadBalancer", "", "hostname"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should set hostname", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesGlobalResource("IngressNginxController", "test2")
			Expect(ingress.Field("status.loadBalancer.hostname").Str).To(Equal("hostname"))
			Expect(ingress.Field("status.loadBalancer.ip").Str).To(BeEmpty())
		})
	})
	Context("Service with LoadBalancer type and with ip", func() {
		BeforeEach(func() {
			resources := []string{
				ingressNginxControllerYAML("test3"),
				serviceYAML("test3", "LoadBalancer", "ip", ""),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should set ip", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesGlobalResource("IngressNginxController", "test3")
			Expect(ingress.Field("status.loadBalancer.hostname").Str).To(BeEmpty())
			Expect(ingress.Field("status.loadBalancer.ip").Str).To(Equal("ip"))
		})
	})

	Context("Service with LoadBalancer type and with ip and hostname", func() {
		BeforeEach(func() {
			resources := []string{
				ingressNginxControllerYAML("test4"),
				serviceYAML("test4", "LoadBalancer", "ip", "hostname"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should set ip", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesGlobalResource("IngressNginxController", "test4")
			Expect(ingress.Field("status.loadBalancer.hostname").Str).To(Equal("hostname"))
			Expect(ingress.Field("status.loadBalancer.ip").Str).To(Equal("ip"))
		})
	})
})

func serviceYAML(name, inlet, ip, hostname string) string {
	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-load-balancer", name),
			Namespace: "d8-ingress-nginx",
			Labels: map[string]string{
				"name": name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
		},
	}
	if inlet == "LoadBalancer" {
		svc.Labels["deckhouse-service-type"] = "provider-managed"
		var loadBalancerIngress corev1.LoadBalancerIngress
		if ip != "" {
			loadBalancerIngress.IP = ip
		}
		if hostname != "" {
			loadBalancerIngress.Hostname = hostname
		}
		svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, loadBalancerIngress)
	}
	marshaled, _ := yaml.Marshal(svc)
	return string(marshaled)
}
