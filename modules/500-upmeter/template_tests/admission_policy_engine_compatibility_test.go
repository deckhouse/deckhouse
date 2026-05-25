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

const upmeterGlobalValuesNoAPE = `
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  cloud:
    prefix: myprefix
    provider: OpenStack
  clusterDomain: cluster.local
  clusterType: "Cloud"
  defaultCRI: Containerd
  kind: ClusterConfiguration
  kubernetesVersion: "1.31"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
enabledModules: ["vertical-pod-autoscaler", "upmeter"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  kubernetesVersion: 1.16.15
`

const upmeterGlobalValuesWithAPE = `
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  cloud:
    prefix: myprefix
    provider: OpenStack
  clusterDomain: cluster.local
  clusterType: "Cloud"
  defaultCRI: Containerd
  kind: ClusterConfiguration
  kubernetesVersion: "1.31"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
enabledModules: ["vertical-pod-autoscaler", "upmeter", "admission-policy-engine", "admission-policy-engine-crd"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  kubernetesVersion: 1.16.15
  apiVersions:
    - deckhouse.io/v1alpha1/SecurityPolicyException
`

var _ = Describe("Module :: upmeter :: admission-policy-engine compatibility", func() {
	f := SetupHelmConfig(``)

	Context("without admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", upmeterGlobalValuesNoAPE)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("upmeter", customCertificatePresent)
			f.HelmRender()
		})

		It("must render restricted namespace label and skip security-check label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-upmeter")
			Expect(namespace.Exists()).To(BeTrue())
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/pod-policy").String()).To(Equal("restricted"))
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/enable-security-policy-check").Exists()).To(BeFalse())
		})

		It("must not render SPE and exception labels", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			agentDaemonSet := f.KubernetesResource("DaemonSet", "d8-upmeter", "upmeter-agent")
			Expect(agentDaemonSet.Exists()).To(BeTrue())
			Expect(agentDaemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-upmeter", "upmeter-agent").Exists()).To(BeFalse())
		})
	})

	Context("with admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", upmeterGlobalValuesWithAPE)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("upmeter", customCertificatePresent)
			f.HelmRender()
		})

		It("must render restricted namespace label and security-check label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-upmeter")
			Expect(namespace.Exists()).To(BeTrue())
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/pod-policy").String()).To(Equal("restricted"))
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/enable-security-policy-check").String()).To(Equal("true"))
		})

		It("must render SPE and exception label only for upmeter-agent", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			agentDaemonSet := f.KubernetesResource("DaemonSet", "d8-upmeter", "upmeter-agent")
			Expect(agentDaemonSet.Exists()).To(BeTrue())
			Expect(agentDaemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("upmeter-agent"))
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-upmeter", "upmeter-agent").Exists()).To(BeTrue())

			upmeterStatefulSet := f.KubernetesResource("StatefulSet", "d8-upmeter", "upmeter")
			Expect(upmeterStatefulSet.Exists()).To(BeTrue())
			Expect(upmeterStatefulSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-upmeter", "upmeter").Exists()).To(BeFalse())
		})
	})
})
