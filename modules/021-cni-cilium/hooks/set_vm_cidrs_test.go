/*
Copyright 2023 Flant JSC

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

var _ = Describe("Modules :: cni-cilium :: hooks :: set_vm_cidrs ::", func() {
	f := HookExecutionConfigInit(`{"cniCilium":{"internal":{}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Set VM CIDRs :: ModuleConfig is missing", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("vmCIDRs in values should be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.vmCIDRs").Array()).To(HaveLen(0))
		})
	})

	Context("Set VM CIDRs :: ModuleConfig does not contain data", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
 name: virtualization
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("vmCIDRs in values should be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.vmCIDRs").Array()).To(HaveLen(0))
		})
	})

	Context("Set VM CIDRs :: vmCIDRs are set in ModuleConfig", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  settings:
    virtualMachineCIDRs:
    - 10.10.10.0/24
    - 10.9.8.0/24
  version: 1
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("vmCIDRs in values should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.vmCIDRs").Array()).To(HaveLen(2))
			Expect(f.ValuesGet("cniCilium.internal.vmCIDRs").String()).To(Equal(`["10.10.10.0/24","10.9.8.0/24"]`))
		})

	})

})
