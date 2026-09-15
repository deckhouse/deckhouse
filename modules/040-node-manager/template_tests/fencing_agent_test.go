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

var _ = Describe("Module :: node-manager :: helm template :: fencing agent", func() {
	f := SetupHelmConfig(``)

	Context("Two NGs with fencing", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager.internal.fencingNodeGroups", `
- name: ng-notify
  mode: Notify
  watchdogTimeout: 60
- name: ng-watchdog
  mode: Watchdog
  watchdogTimeout: 45
`)
			f.ValuesSetFromYaml("nodeManager.internal.capiControllerManagerWebhookCert", `{ca: string, crt: string, key: string}`)
			f.ValuesSetFromYaml("nodeManager.internal.capsControllerManagerWebhookCert", `{ca: string, crt: string, key: string}`)
			f.ValuesSetFromYaml("nodeManager.internal.nodeControllerWebhookCert", `{ca: string, crt: string, key: string}`)
			f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{"master":1}`)
			f.ValuesSetFromYaml("global.clusterConfiguration", `apiVersion: deckhouse.io/v1
cloud:
  prefix: sandbox
  provider: vSphere
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "1.32"
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender(WithAPIVersions(
				"autoscaling.k8s.io/v1/VerticalPodAutoscaler",
				"deckhouse.io/v1alpha1/SecurityPolicyException",
			))
		})

		// nodeManager.internal.nodeGroups is left unset on purpose: the fencing
		// templates must read only their own narrow key.
		It("renders one agent per fencing NodeGroup", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ds := f.KubernetesResource("DaemonSet", "d8-cloud-instance-manager", "fencing-agent-ng-watchdog")
			Expect(ds.Exists()).To(BeTrue())
			Expect(ds.Field("spec.template.spec.containers.0.env.0.name").String()).To(Equal("LOG_LEVEL"))
			Expect(ds.Field("spec.template.spec.containers.0.env.1.value").String()).To(Equal("Watchdog"))
			Expect(ds.Field("spec.template.spec.containers.0.env.3.value").String()).To(Equal("45"))

			dsNotify := f.KubernetesResource("DaemonSet", "d8-cloud-instance-manager", "fencing-agent-ng-notify")
			Expect(dsNotify.Exists()).To(BeTrue())
			Expect(dsNotify.Field("spec.template.spec.containers.0.env.1.value").String()).To(Equal("Notify"))
			Expect(dsNotify.Field("spec.template.spec.containers.0.env.3.value").String()).To(Equal("60"))

			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-instance-manager", "fencing-agent-ng-watchdog").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-cloud-instance-manager", "fencing-agent-ng-watchdog").Exists()).To(BeTrue())

			ngc := f.KubernetesGlobalResource("NodeGroupConfiguration", "enable-watchdog-for-nodegroup-ng-watchdog.sh")
			Expect(ngc.Exists()).To(BeTrue())
			Expect(ngc.Field("spec.content").String()).To(ContainSubstring("soft_margin=45"))
		})
	})
})
