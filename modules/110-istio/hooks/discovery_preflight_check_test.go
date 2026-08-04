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

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: discovery_preflight_check ::", func() {
	initValues := `
istio:
  internal:
    istioToK8sCompatibilityMap:
      "1.27": ["1.32", "1.33", "1.34", "1.35", "1.36"]
`
	f := HookExecutionConfigInit(initValues, "")

	Context("kubernetesVersionIsDefault is true", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.kubernetesVersionIsDefault", true)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should publish compatibility map and automatic flag", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(true))

			compatibilityMap, exists := requirements.GetValue(istioToK8sCompatibilityMapKey)
			Expect(exists).To(BeTrue())
			Expect(compatibilityMap).To(BeEquivalentTo(map[string][]string{
				"1.27": {"1.32", "1.33", "1.34", "1.35", "1.36"},
			}))
		})
	})

	Context("kubernetesVersionIsDefault is false", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.kubernetesVersionIsDefault", false)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should not mark kube version as automatic", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(false))
		})
	})

	Context("kubernetesVersionIsDefault is unset", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should treat missing flag as not automatic", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(false))
		})
	})
})
