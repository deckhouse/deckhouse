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

package template_tests

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	reservedHostsConfigMapName   = "d8-reserved-public-hosts"
	reservedHostsIngressPolicy   = "reserved-public-hosts-ingress.deckhouse.io"
	reservedHostsHTTPRoutePolicy = "reserved-public-hosts-httproute.deckhouse.io"

	validatingAdmissionPolicyAPI        = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"
	validatingAdmissionPolicyBindingAPI = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding"
	httpRouteAPI                        = "gateway.networking.k8s.io/v1/HTTPRoute"
)

var _ = Describe("Module :: deckhouse :: reserved public hosts ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	render := func(publicDomainTemplate string, apiVersions ...string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		if publicDomainTemplate != "" {
			f.ValuesSet("global.modules.publicDomainTemplate", publicDomainTemplate)
		}
		f.HelmRender(WithAPIVersions(apiVersions...))
	}

	admissionAPIs := []string{validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI}

	Context("Ingress hosts are reserved when the platform publishes any", func() {
		BeforeEach(func() {
			render("%s.example.com", admissionAPIs...)
		})

		It("reserves the hostname of every service the platform publishes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			Expect(cm.Exists()).To(BeTrue())

			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).To(ContainElements(
				"api.example.com",
				"console.example.com",
				"dashboard.example.com",
				"dex.example.com",
				"grafana.example.com",
				"kubeconfig.example.com",
				"prometheus.example.com",
			))
			// The policies lowercase the claimed hostname before comparing, so a reserved one that
			// kept any uppercase would silently never match.
			for _, host := range hosts {
				Expect(host).To(Equal(strings.ToLower(host)))
			}
		})

		It("denies an Ingress claiming one of them", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy)
			Expect(vap.Exists()).To(BeTrue())
			Expect(vap.Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(vap.Field("spec.paramKind.kind").String()).To(Equal("ConfigMap"))
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.apiGroups").String()).To(MatchJSON(`["networking.k8s.io"]`))
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(MatchJSON(`["ingresses"]`))
			// DELETE is deliberately absent: an Ingress that predates the policy stays routable
			// until somebody rewrites it.
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.operations").String()).To(MatchJSON(`["CREATE","UPDATE"]`))
			Expect(vap.Field(`spec.variables.#(name=="claimedHosts").expression`).String()).To(ContainSubstring("object.spec.rules"))
			Expect(vap.Field("spec.validations.0.reason").String()).To(Equal("Forbidden"))
		})

		It("exempts the writers that create the platform's own objects", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy)
			Expect(vap.Field("spec.matchConditions").Array()).To(HaveLen(3))
			Expect(vap.Field(`spec.matchConditions.#(name=="exclude-system-serviceaccounts").expression`).String()).
				To(ContainSubstring("system:serviceaccount:d8-"))
		})

		It("binds the policy to the parameters and skips the platform's own namespaces", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsIngressPolicy)
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("spec.policyName").String()).To(Equal(reservedHostsIngressPolicy))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
			Expect(binding.Field("spec.paramRef.name").String()).To(Equal(reservedHostsConfigMapName))
			Expect(binding.Field("spec.paramRef.namespace").String()).To(Equal("d8-system"))
			// Without this, losing the ConfigMap would take every Ingress write in the cluster
			// down with it.
			Expect(binding.Field("spec.paramRef.parameterNotFoundAction").String()).To(Equal("Allow"))
			Expect(binding.Field("spec.matchResources.namespaceSelector.matchExpressions").String()).To(MatchJSON(`[
				{"key": "heritage", "operator": "NotIn", "values": ["deckhouse"]},
				{"key": "security.deckhouse.io/reserved-hosts-bypass", "operator": "NotIn", "values": ["true"]}
			]`))
		})

		It("leaves HTTPRoute alone while the Gateway API is not installed", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsHTTPRoutePolicy).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsHTTPRoutePolicy).Exists()).To(BeFalse())
		})
	})

	Context("The hostnames follow publicDomainTemplate, whatever shape it has", func() {
		BeforeEach(func() {
			render("%s-kube.example.com", admissionAPIs...)
		})

		It("reserves what the template actually renders, not a fixed subdomain", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).To(ContainElement("console-kube.example.com"))
			Expect(hosts).NotTo(ContainElement("console.example.com"))
		})
	})

	Context("The Gateway API is installed", func() {
		BeforeEach(func() {
			render("%s.example.com", append(admissionAPIs, httpRouteAPI)...)
		})

		It("reserves the same hostnames on HTTPRoute", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsHTTPRoutePolicy)
			Expect(vap.Exists()).To(BeTrue())
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.apiGroups").String()).To(MatchJSON(`["gateway.networking.k8s.io"]`))
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(MatchJSON(`["httproutes"]`))
			Expect(vap.Field(`spec.variables.#(name=="claimedHosts").expression`).String()).To(ContainSubstring("object.spec.hostnames"))

			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsHTTPRoutePolicy)
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("spec.paramRef.name").String()).To(Equal(reservedHostsConfigMapName))
		})
	})

	Context("The cluster has no ValidatingAdmissionPolicy", func() {
		BeforeEach(func() {
			render("%s.example.com")
		})

		It("renders nothing rather than a ConfigMap nobody reads", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsIngressPolicy).Exists()).To(BeFalse())
		})
	})

	Context("The platform publishes nothing", func() {
		BeforeEach(func() {
			render("", admissionAPIs...)
		})

		It("reserves nothing, there are no system hostnames to protect", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsIngressPolicy).Exists()).To(BeFalse())
		})
	})
})
