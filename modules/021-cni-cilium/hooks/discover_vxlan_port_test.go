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

var _ = Describe("Modules :: cni-cilium :: hooks :: discover_vxlan_port ::", func() {
	f := HookExecutionConfigInit(`{"cniCilium":{"internal":{}}}`, "")

	Context("New cluster :: ConfigMap empty :: Virtualization Off", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4299", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4299"))
		})
	})

	Context("New cluster :: ConfigMap empty :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4298", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4298"))
		})
	})

	Context("New cluster :: ConfigMap set to 4299 :: Virtualization Off", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel-port: "4299"
---
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4299", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4299"))
		})
	})

	Context("New cluster :: ConfigMap set to 4298 :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel-port: "4298"
---
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4298", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4298"))
		})
	})

	Context("New cluster :: ConfigMap set to 4299 :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel-port: "4299"
---
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4298", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4298"))
		})
	})

	Context("Existing cluster :: ConfigMap empty :: Virtualization Off", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
---
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8472", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8472"))
		})
	})

	Context("Existing cluster :: ConfigMap empty :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
---
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8469", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8469"))
		})
	})

	Context("Existing cluster :: ConfigMap set to 8472 :: Virtualization Off", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel-port: "8472"
---
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8472", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8472"))
		})
	})

	Context("Existing cluster :: ConfigMap set to 8469 :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel-port: "8469"
---
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8469", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8469"))
		})
	})

	Context("Existing cluster :: ConfigMap set to 8472 :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel-port: "8472"
---
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4298", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4298"))
		})
	})

	Context("Existing cluster :: ConfigMap set to 5555 :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel-port: "5555"
---
`)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 5555, There is alert about non-standard port", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("5555"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).Should(Equal("expire"))
			Expect(m[1].Name).Should(Equal("d8_cni_cilium_non_standard_vxlan_port"))
		})
	})
})
