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

var _ = Describe("Module :: node-manager :: helm template :: softdog", func() {
	f := SetupHelmConfig(``)

	softdog := func(name string) string {
		return f.KubernetesGlobalResource("NodeGroupConfiguration", "enable-watchdog-for-nodegroup-"+name+".sh").Field("spec.content").String()
	}

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+fencingDataInternalValues)
		f.ValuesSetFromYaml("nodeManager.internal.fencing", `
nodeGroups:
  slow: {mode: Slow}
  normal: {mode: Normal}
  critical: {mode: Critical}
  disabled: {mode: Disabled}
  plain: {mode: Disabled}
`)
		f.ApplyOpenAPIDefaults()
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("loads softdog with soft_panic=1 and gives the device to uid 64535 on every fenced group", func() {
		for _, name := range []string{"slow", "normal", "critical"} {
			content := softdog(name)
			Expect(content).To(ContainSubstring(`KERNEL=="watchdog*", OWNER="64535", GROUP="root", MODE="0660"`), name)
			Expect(content).To(ContainSubstring("modprobe softdog soft_panic=1"), name)
			Expect(content).To(ContainSubstring("mknod"), name)
			Expect(content).To(ContainSubstring(`chown 64535:root "${WD}"`), name)
			Expect(content).To(ContainSubstring(`chmod 0660 "${WD}"`), name)
		}
	})

	It("reloads an idle softdog and keeps one that is in use", func() {
		content := softdog("slow")
		Expect(content).To(ContainSubstring(`refs=$(awk '$1=="softdog"{print $3}' /proc/modules)`))
		Expect(content).To(ContainSubstring(`if [ "${refs}" = "0" ]; then`))
		Expect(content).To(ContainSubstring("rmmod softdog ||"))
		Expect(content).To(ContainSubstring("softdog is in use, keeping the loaded module"))
	})

	It("ships no softdog script to the groups without fencing", func() {
		for _, name := range []string{"disabled", "plain"} {
			Expect(f.KubernetesGlobalResource("NodeGroupConfiguration", "enable-watchdog-for-nodegroup-"+name+".sh").Exists()).To(BeFalse(), name)
		}
	})
})
