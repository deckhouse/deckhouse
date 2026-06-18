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

package migrate

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: migrate :: remove obsolete monitoring-deckhouse ModuleConfig", func() {
	const obsoleteMC = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-deckhouse
spec:
  enabled: true
`
	const otherMC = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
`

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("When the obsolete monitoring-deckhouse ModuleConfig exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(obsoleteMC + otherMC))
			f.RunHook()
		})

		It("Deletes only the obsolete ModuleConfig", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ModuleConfig", "monitoring-deckhouse").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ModuleConfig", "deckhouse").Exists()).To(BeTrue())
		})
	})

	Context("When the obsolete monitoring-deckhouse ModuleConfig is absent", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(otherMC))
			f.RunHook()
		})

		It("Does nothing and keeps other ModuleConfigs", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ModuleConfig", "deckhouse").Exists()).To(BeTrue())
		})
	})
})
