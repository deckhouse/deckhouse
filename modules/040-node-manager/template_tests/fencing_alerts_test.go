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
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

var _ = Describe("Module :: node-manager :: helm template :: alerts of fencing", func() {
	f := SetupHelmConfig(``)

	render := func(fencing string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.enabledModules", []string{"vertical-pod-autoscaler", "operator-prometheus-crd"})
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		f.ValuesSetFromYaml("nodeManager.internal.fencing", fencing)
		f.ApplyOpenAPIDefaults()
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	rule := func(name string) object_store.KubeObject {
		return f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-"+name)
	}

	Context("every node group is Disabled", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Disabled}}`)
		})

		It("ships the alert of deprecated settings", func() {
			Expect(rule("node-group-deprecate").Field("spec.groups.#.rules.#.alert").String()).To(ContainSubstring("D8NodeGroupFencingDeprecatedSettings"))
		})

		It("does not load the rules of the agents and of the controller", func() {
			Expect(rule("fencing").Exists()).To(BeFalse())
			Expect(rule("fencing-controller").Exists()).To(BeFalse())
		})
	})

	Context("a node group runs an agent", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Normal}}`)
		})

		It("loads the rules of the agents and the alerts of incidents", func() {
			Expect(rule("fencing").Exists()).To(BeTrue())
			Expect(rule("fencing-controller").Field("spec.groups.#.rules.#.alert").String()).To(ContainSubstring("D8FencingNodeFailed"))
		})

		It("names both reasons of the taint in D8FencingNodeFailed", func() {
			description := rule("fencing-controller").Field(`spec.groups.0.rules.#(alert=="D8FencingNodeFailed").annotations.description`).String()
			Expect(description).To(ContainSubstring("witness"))
			Expect(description).To(ContainSubstring("heartbeat"))
		})
	})
})
