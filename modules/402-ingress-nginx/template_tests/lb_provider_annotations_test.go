/*
Copyright 2025 Flant JSC

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

package template_tests

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

const (
	providerControllerName = "provider-lb"
	providerControllerIP   = "10.0.0.5"
)

func providerControllerSpec(annotations map[string]string) string {
	spec := map[string]interface{}{
		"name": providerControllerName,
		"spec": map[string]interface{}{
			"inlet":        "LoadBalancer",
			"ingressClass": "nginx",
			"loadBalancer": map[string]interface{}{
				"loadBalancerIP": providerControllerIP,
			},
		},
	}

	if len(annotations) > 0 {
		spec["spec"].(map[string]interface{})["loadBalancer"].(map[string]interface{})["annotations"] = annotations
	}

	serialized, _ := yaml.Marshal(spec)
	return string(serialized)
}

var baseEnabledModules = []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus", "control-plane-manager"}

var _ = Describe("Module :: ingress-nginx :: helm template :: load balancer provider annotations", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.30.14")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.deckhouse.io/deckhouse/fe")
		hec.ValuesSet("global.enabledModules", baseEnabledModules)
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("ingressNginx.defaultControllerVersion", "1.10")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.ca", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.cert", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.key", "test")
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.namespaces", json.RawMessage("[]"))
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.ingresses", json.RawMessage("[]"))
		hec.ValuesSet("ingressNginx.internal.geoproxyReady", true)
		hec.ValuesSetFromYaml("ingressNginx.internal.nginxAuthTLS", fmt.Sprintf(`
- controllerName: %s
  ingressClass: nginx
  data:
    cert: teststring
    key: teststring
`, providerControllerName))
	})

	renderService := func(enabledModules []string, controllerSpec string) object_store.KubeObject {
		hec.ValuesSet("global.enabledModules", enabledModules)
		hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllers.0", controllerSpec)
		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		return hec.KubernetesResource("Service", "d8-ingress-nginx", providerControllerName+"-load-balancer")
	}

	moduleList := func(extras ...string) []string {
		modules := append([]string{}, baseEnabledModules...)
		return append(modules, extras...)
	}

	It("adds openstack annotations when cloud-provider-openstack is enabled", func() {
		service := renderService(moduleList("cloud-provider-openstack"), providerControllerSpec(nil))
		Expect(service.Exists()).To(BeTrue())
		Expect(service.Field(`metadata.annotations.loadbalancer\.openstack\.org/keep-floatingip`).String()).To(Equal("true"))
		Expect(service.Field(`metadata.annotations.loadbalancer\.openstack\.org/load-balancer-address`).String()).To(Equal(providerControllerIP))
	})

	It("adds yandex annotation when cloud-provider-yandex is enabled", func() {
		service := renderService(moduleList("cloud-provider-yandex"), providerControllerSpec(nil))
		Expect(service.Exists()).To(BeTrue())
		Expect(service.Field(`metadata.annotations.yandex\.cpi\.flant\.com/listener-address-ipv4`).String()).To(Equal(providerControllerIP))
	})

	It("does not emit provider annotations when cloud-provider-aws is enabled", func() {
		service := renderService(moduleList("cloud-provider-aws"), providerControllerSpec(nil))
		Expect(service.Exists()).To(BeTrue())
		Expect(service.Field(`metadata.annotations.loadbalancer\.openstack\.org/keep-floatingip`).Exists()).To(BeFalse())
		Expect(service.Field(`metadata.annotations.yandex\.cpi\.flant\.com/listener-address-ipv4`).Exists()).To(BeFalse())
	})

	It("does not emit provider annotations when no cloud provider module is enabled", func() {
		service := renderService(moduleList(), providerControllerSpec(nil))
		Expect(service.Exists()).To(BeTrue())
		Expect(service.Field(`metadata.annotations.loadbalancer\.openstack\.org/keep-floatingip`).Exists()).To(BeFalse())
		Expect(service.Field(`metadata.annotations.yandex\.cpi\.flant\.com/listener-address-ipv4`).Exists()).To(BeFalse())
	})

	It("preserves user annotations alongside openstack annotations", func() {
		custom := map[string]string{
			"user.provided": "value",
		}
		service := renderService(moduleList("cloud-provider-openstack"), providerControllerSpec(custom))
		Expect(service.Exists()).To(BeTrue())
		Expect(service.Field(`metadata.annotations.loadbalancer\.openstack\.org/keep-floatingip`).String()).To(Equal("true"))
		Expect(service.Field(`metadata.annotations.loadbalancer\.openstack\.org/load-balancer-address`).String()).To(Equal(providerControllerIP))
		Expect(service.Field(`metadata.annotations.user\.provided`).String()).To(Equal("value"))
	})

	It("keeps user annotations when no provider is enabled", func() {
		custom := map[string]string{
			"user.only": "solo",
		}
		service := renderService(moduleList(), providerControllerSpec(custom))
		Expect(service.Exists()).To(BeTrue())
		Expect(service.Field(`metadata.annotations.user\.only`).String()).To(Equal("solo"))
		Expect(service.Field(`metadata.annotations.loadbalancer\.openstack\.org/keep-floatingip`).Exists()).To(BeFalse())
		Expect(service.Field(`metadata.annotations.yandex\.cpi\.flant\.com/listener-address-ipv4`).Exists()).To(BeFalse())
	})
})
