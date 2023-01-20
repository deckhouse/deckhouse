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

var _ = Describe("Modules :: virtualization :: hooks :: detect_cilium_tunnel ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Detect cilium tunnel :: ConfigMap is missing", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(``),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("routeLocal in values should be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("virtualization.internal.routeLocal").Bool()).To(BeFalse())
		})
	})

	Context("Detect cilium tunnel :: ConfigMap does not contain data", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
			`), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("routeLocal in values should be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("virtualization.internal.routeLocal").Bool()).To(BeFalse())
		})
	})

	Context("Detect cilium tunnel :: tunnel mode is enabled in ConfigMap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel: vxlan
			`), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("routeLocal in values should be true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("virtualization.internal.routeLocal").Bool()).To(BeTrue())
		})
	})

	Context("Detect cilium tunnel :: tunnel mode is not set in ConfigMap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
			`), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("routeLocal in values should be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("virtualization.internal.routeLocal").Bool()).To(BeFalse())
		})
	})

	Context("Detect cilium tunnel :: tunnel mode is disabled in ConfigMap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel: disabled
			`), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("routeLocal in values should be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("virtualization.internal.routeLocal").Bool()).To(BeFalse())
		})
	})
})
