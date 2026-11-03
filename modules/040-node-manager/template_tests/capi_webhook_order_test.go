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
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: node-manager :: helm template :: capi webhook order", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "operator-prometheus-crd"]`)
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
		f.ValuesSet("nodeManager.internal.capiControllerManagerEnabled", true)
		setBashibleAPIServerTLSValues(f)
	})

	assertCapiWebhookOrder := func(webhookState, deploymentTracking string) {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		deployment := f.KubernetesResource("Deployment", nodeManagerNamespace, "capi-controller-manager")
		Expect(deployment.Field(`metadata.annotations.werf\.io/track-termination-mode`).String()).To(Equal(deploymentTracking))

		for kind, name := range map[string]string{
			"MutatingWebhookConfiguration":   "capi-mutating-webhook-configuration",
			"ValidatingWebhookConfiguration": "capi-validating-webhook-configuration",
		} {
			webhook := f.KubernetesGlobalResource(kind, name)
			Expect(webhook.Exists()).To(BeTrue(), kind)
			Expect(webhook.Field(`metadata.annotations.werf\.io/deploy-dependency-deployment`).String()).To(HavePrefix("state="+webhookState+","), kind)
		}
	}

	// The EE chart replaces templates/monitoring.yaml, so the CE render does not cover it.
	It("installs the capi alerts in the EE edition too", func() {
		content, err := os.ReadFile("../../../ee/modules/040-node-manager/templates/monitoring.yaml")
		if os.IsNotExist(err) {
			Skip("EE sources are not present")
		}

		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("monitoring/prometheus-rules/capi-controller-manager.yaml"))
	})

	Context("While the cluster is bootstrapped for the first time", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.HelmRender()
		})

		It("creates the capi webhooks only after every capi replica is ready", func() {
			assertCapiWebhookOrder("ready", "WaitUntilResourceReady")
		})
	})

	Context("On a bootstrapped cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.HelmRender()
		})

		It("does not wait for every capi replica, so a missing replica does not hold the release", func() {
			assertCapiWebhookOrder("present", "")
		})

		It("alerts on missing capi replicas instead", func() {
			rules := f.KubernetesResource("PrometheusRule", nodeManagerNamespace, "node-manager-capi-controller-manager")
			Expect(rules.Exists()).To(BeTrue())
			Expect(rules.Field("spec.groups.0.rules.#.alert").String()).To(MatchJSON(`["D8CAPIControllerManagerNotAvailable","D8CAPIControllerManagerReplicasUnavailable"]`))
		})
	})
})
