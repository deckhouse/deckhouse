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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: disable_load_balancer ::", func() {
	const initValues = `
cloudProviderDvp:
  internal: {}
`

	const disableConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: dvp-cloud-controller-manager-disable-lb
  namespace: d8-cloud-provider-dvp
`

	Context("When the disable ConfigMap is absent", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should set loadBalancer.disabled to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.loadBalancer.disabled").Bool()).To(BeFalse())
		})
	})

	Context("When the disable ConfigMap is present", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(disableConfigMap))
			f.RunHook()
		})

		It("Should set loadBalancer.disabled to true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.loadBalancer.disabled").Bool()).To(BeTrue())
		})
	})
})
