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

const legacy = `
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: legacy
spec:
  deschedulerPolicy:
    strategies:
      removePodsViolatingInterPodAntiAffinity:
        enabled: true
      removePodsViolatingNodeAffinity:
        enabled: true
status:
  ready: true
`

var _ = Describe("Descheduler migration :: delete deprecated legacy CR", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Descheduler", false)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Legacy CR exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(legacy))
			f.RunHook()
		})
		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Descheduler", "legacy").Exists()).To(BeFalse())
		})
	})
})
