/*
Copyright 2021 Flant JSC

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

var _ = Describe("Prometheus hooks :: enable prompp ::", func() {
	f := HookExecutionConfigInit(``, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Module", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
			f.RunHook()
		})

		Context("No prompp module found", func() {
			It("Should not create prompp ModuleConfig", func() {
				Expect(f).To(ExecuteSuccessfully())

				mc := f.KubernetesGlobalResource("ModuleConfig", promppModuleName)
				Expect(mc.Exists()).To(BeFalse())
			})
		})
	})
	Context("Cluster with prompp module", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: prompp
spec: {}
`, 1))
			f.RunHook()
		})

		It("Should create prompp ModuleConfig", func() {
			Expect(f).To(ExecuteSuccessfully())

			mc := f.KubernetesGlobalResource("ModuleConfig", promppModuleName)
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
		})
	})
	Context("Cluster with prompp module being explicitly disabled in ModuleConfig", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: prompp
spec: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prompp
spec:
  enabled: false
`, 2))
			f.RunHook()
		})

		It("Should leave prompp ModuleConfig intact", func() {
			Expect(f).To(ExecuteSuccessfully())

			mc := f.KubernetesGlobalResource("ModuleConfig", promppModuleName)
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeFalse())
		})
	})
})
