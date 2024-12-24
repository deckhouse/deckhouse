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
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const observabilityMCTestManifest = `---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: null
  name: observability
spec:
  enabled: false
`

var _ = Describe("Modules :: prometheus :: hooks :: create_observability_module_config ::", func() {
	f := HookExecutionConfigInit(`{"global":{"enabledModules":[]}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Without enabled observability module", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Must not create observability module config", func() {
			Expect(f).To(ExecuteSuccessfully())
			observabilityModuleConfig := f.KubernetesResource("ModuleConfig", "", "observability")
			Expect(observabilityModuleConfig).Should(BeNil())
		})
	})

	Context("With enabled observability module", func() {
		BeforeEach(func() {
			f.ValuesSet("global.enabledModules", []string{"observability"})
			f.RunHook()
		})

		It("Must create observability module config", func() {
			Expect(f).To(ExecuteSuccessfully())
			observabilityModuleConfig := f.KubernetesResource("ModuleConfig", "", "observability")
			Expect(observabilityModuleConfig).ShouldNot(BeNil())
			Expect(observabilityModuleConfig.Field("spec.enabled").Bool()).Should(Equal(true))
		})
	})

	Context("Observability module is turned off", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(observabilityMCTestManifest))
			f.RunHook()
		})

		It("Must do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			observabilityModuleConfig := f.KubernetesResource("ModuleConfig", "", "observability")
			Expect(observabilityModuleConfig).ShouldNot(BeNil())
			Expect(observabilityModuleConfig.Field("spec.enabled").Bool()).Should(Equal(false))
		})
	})
})
