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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cni-cilium :: hooks :: discover_vxlan_port ::", func() {
	f := HookExecutionConfigInit(`{"cniCilium":{"internal":{}}}`, "")

	Context("New cluster :: No ConfigMap :: Virtualization Off :: Nesting level is not set", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4298", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4298"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("New cluster :: No ConfigMap :: Virtualization On :: Nesting level is not set", func() {
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

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("New cluster :: No ConfigMap :: Virtualization Off :: Nesting level is 1 ", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.discovery.dvpNestingLevel", []byte(`1`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4297", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4297"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("New cluster :: No ConfigMap :: Virtualization On :: Nesting level is 1", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global", []byte(`
discovery:
  dvpNestingLevel: 1
enabledModules:
- virtualization`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4297", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4297"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap set to 4299 :: Virtualization Off :: Nesting level is not set", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("4299"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4299", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4299"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap set to 4298 :: Virtualization On :: Nesting level is not set", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("4298"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4298", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4298"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap set to 4299 :: Virtualization Off :: Nesting level is 1", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("4299"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.discovery.dvpNestingLevel", []byte(`1`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4299, There is an alert about non-standard port", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4299"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).Should(Equal(operation.ActionExpireMetrics))
			Expect(m[1].Name).Should(Equal("d8_cni_cilium_non_standard_vxlan_port"))
			Expect(m[1].Action).To(BeEquivalentTo(operation.ActionGaugeSet))
			Expect(m[1].Value).To(BeEquivalentTo(ptr.To(1.0)))
		})
	})

	Context("Existing cluster :: ConfigMap set to 4298 :: Virtualization On :: Nesting level is 1", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("4298"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global", []byte(`
discovery:
  dvpNestingLevel: 1
enabledModules:
- virtualization`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4298, There is an alert about non-standard port", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4298"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).Should(Equal(operation.ActionExpireMetrics))
			Expect(m[1].Name).Should(Equal("d8_cni_cilium_non_standard_vxlan_port"))
			Expect(m[1].Action).To(BeEquivalentTo(operation.ActionGaugeSet))
			Expect(m[1].Value).To(BeEquivalentTo(ptr.To(1.0)))
		})
	})

	Context("Existing cluster :: ConfigMap set to 4300 :: Virtualization Off :: Nesting level is 1", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("4300"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.discovery.dvpNestingLevel", []byte(`1`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4300, There is an alert about non-standard port", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4300"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).Should(Equal(operation.ActionExpireMetrics))
			Expect(m[1].Name).Should(Equal("d8_cni_cilium_non_standard_vxlan_port"))
			Expect(m[1].Action).To(BeEquivalentTo(operation.ActionGaugeSet))
			Expect(m[1].Value).To(BeEquivalentTo(ptr.To(1.0)))
		})
	})

	Context("Existing cluster :: ConfigMap set to 4299 :: Virtualization On :: Nesting level is 1", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("4299"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global", []byte(`
discovery:
  dvpNestingLevel: 1
enabledModules:
- virtualization`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4299, There is an alert about non-standard port", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4299"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).Should(Equal(operation.ActionExpireMetrics))
			Expect(m[1].Name).Should(Equal("d8_cni_cilium_non_standard_vxlan_port"))
			Expect(m[1].Action).To(BeEquivalentTo(operation.ActionGaugeSet))
			Expect(m[1].Value).To(BeEquivalentTo(ptr.To(1.0)))
		})
	})

	Context("Existing cluster :: ConfigMap empty :: Virtualization Off", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort(""))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8472", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8472"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap empty :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort(""))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8469", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8469"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap empty :: Virtualization On :: Nesting level is 1", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort(""))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global", []byte(`
discovery:
  dvpNestingLevel: 1
enabledModules:
- virtualization`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4297", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4297"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap empty :: Virtualization Off :: Nesting level is 5", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort(""))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.discovery.dvpNestingLevel", []byte(`5`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4293", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4293"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap set to 8472 :: Virtualization Off", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("8472"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8472", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8472"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap set to 8469 :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("8469"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8469", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8469"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap set to 8472 :: Virtualization On :: Nesting leveis is not set", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("8472"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4298", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4298"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap set to 8472 :: Virtualization On :: Nesting leveis is 1", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("8472"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global", []byte(`
discovery:
  dvpNestingLevel: 1
enabledModules:
- virtualization`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8472, There is an alert about non-standard port", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8472"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).Should(Equal(operation.ActionExpireMetrics))
			Expect(m[1].Name).Should(Equal("d8_cni_cilium_non_standard_vxlan_port"))
			Expect(m[1].Action).To(BeEquivalentTo(operation.ActionGaugeSet))
			Expect(m[1].Value).To(BeEquivalentTo(ptr.To(1.0)))
		})
	})

	Context("Existing cluster :: ConfigMap set to 5555 :: Virtualization On", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("5555"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 5555, There is an alert about non-standard port", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("5555"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).Should(Equal(operation.ActionExpireMetrics))
			Expect(m[1].Name).Should(Equal("d8_cni_cilium_non_standard_vxlan_port"))
			Expect(m[1].Action).To(BeEquivalentTo(operation.ActionGaugeSet))
			Expect(m[1].Value).To(BeEquivalentTo(ptr.To(1.0)))
		})
	})

	Context("Existing cluster :: ConfigMap has a faulty value :: Virtualization On :: Nesting level is not set", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("abc"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[virtualization]`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 8469", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("8469"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap set to 4298 :: Virtualization Off :: Nesting level is not set", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("4298"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4298", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4298"))

			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("Existing cluster :: ConfigMap set to 4299 :: Virtualization Off :: Nesting level is 1", func() {
		BeforeEach(func() {
			f.KubeStateSet(getCiliumConfigMapWithPort("4299"))
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
			)
			f.ValuesSetFromYaml("global.discovery.dvpNestingLevel", []byte(`1`))
			f.RunHook()
		})

		It("VXLAN Tunnel Port should be 4299, There is an alert about non-standard port", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.tunnelPortVXLAN").String()).To(Equal("4299"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).Should(Equal(operation.ActionExpireMetrics))
			Expect(m[1].Name).Should(Equal("d8_cni_cilium_non_standard_vxlan_port"))
			Expect(m[1].Action).To(BeEquivalentTo(operation.ActionGaugeSet))
			Expect(m[1].Value).To(BeEquivalentTo(ptr.To(1.0)))
		})
	})
})

func getCiliumConfigMapWithPort(port string) string {
	if len(port) > 0 {
		return fmt.Sprintf(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:
  tunnel-port: "%s"`, port)
	}

	return `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: d8-cni-cilium
data:`
}
