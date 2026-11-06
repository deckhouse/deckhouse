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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// namespace.yaml creates the namespaces other modules keep their objects in. The labels of
// d8-monitoring are checked in module_test.go, the kube-rbac-proxy CA it publishes in
// kube_rbac_proxy_test.go.

var _ = Describe("Module :: deckhouse :: namespaces ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("keeps every namespace when Helm removes it from the release", func() {
		for _, name := range []string{"d8-monitoring", "d8-service-accounts", "d8-cloud-instance-manager"} {
			namespace := f.KubernetesGlobalResource("Namespace", name)
			Expect(namespace.Exists()).To(BeTrue(), "the %s namespace should be rendered", name)
			Expect(namespace.Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"),
				"deleting %s would delete what the other modules keep in it", name)
		}
	})

	It("collects the alerting rules and the extended monitoring of d8-service-accounts", func() {
		namespace := f.KubernetesGlobalResource("Namespace", "d8-service-accounts")
		Expect(namespace.Field(`metadata.labels.prometheus\.deckhouse\.io/rules-watcher-enabled`).String()).To(Equal("true"))
		Expect(namespace.Field(`metadata.labels.extended-monitoring\.deckhouse\.io/enabled`).Exists()).To(BeTrue())
	})

	It("checks the pods of d8-cloud-instance-manager against the restricted standard in warn mode", func() {
		namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
		Expect(namespace.Field(`metadata.labels.security\.deckhouse\.io/pod-policy`).String()).To(Equal("restricted"))
		Expect(namespace.Field(`metadata.labels.security\.deckhouse\.io/pod-policy-action`).String()).To(Equal("warn"))
		Expect(namespace.Field(`metadata.labels.security\.deckhouse\.io/enable-security-policy-check`).String()).To(Equal("true"))
		Expect(namespace.Field(`metadata.labels.prometheus\.deckhouse\.io/rules-watcher-enabled`).String()).To(Equal("true"))
		Expect(namespace.Field(`metadata.labels.extended-monitoring\.deckhouse\.io/enabled`).Exists()).To(BeTrue())
	})
})
