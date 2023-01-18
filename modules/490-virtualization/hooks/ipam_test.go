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
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

var _ = Describe("Modules :: virtualization :: hooks :: ipam ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "VirtualMachineIPAddressLease", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "VirtualMachineIPAddressClaim", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(``),
			)
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("IPAM Normal", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm1
  namespace: ns1
#spec:
#	static: true
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm2
  namespace: ns1
spec:
  static: false
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressLease
metadata:
  name: ip-10-10-10-2
spec:
  claimRef:
    name: vm3
    namespace: ns1
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm3
  namespace: ns1
spec:
  static: false
  address: 10.10.10.2
  leaseName: ip-10-10-10-2
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressLease
metadata:
  name: ip-10-10-10-3
spec:
  claimRef:
    name: vm4
    namespace: ns1
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressLease
metadata:
  name: ip-10-10-10-4
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressLease
metadata:
  name: ip-10-10-10-5
spec:
  claimRef:
    name: vm6
    namespace: ns1
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm6
  namespace: ns1
spec:
  leaseName: ip-10-10-10-5
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressLease
metadata:
  name: ip-10-10-10-6
spec:
  claimRef:
    name: vm7
    namespace: ns1
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm7
  namespace: ns1
spec:
  static: false
  address: 10.10.10.7
  leaseName: ip-10-10-10-6
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm8
  namespace: ns1
spec:
  static: false
  address: 10.10.10.8
  leaseName: ip-10-10-10-8
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressLease
metadata:
  name: ip-10-10-10-8
spec:
  claimRef:
    name: vm8
    namespace: ns1
status:
  phase: Bound
`),
			)
			f.RunHook()
		})

		It("Manages VirtualMachineIPAddressLeases", func() {
			Expect(f).To(ExecuteSuccessfully())
			var lease object_store.KubeObject
			var claim object_store.KubeObject

			By("Should create and assign lease to static IP address claim")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-0")
			Expect(lease).To(Not(BeEmpty()))
			Expect(lease.Field(`spec.claimRef.name`).String()).To(Equal("vm1"))
			Expect(lease.Field(`spec.claimRef.namespace`).String()).To(Equal("ns1"))
			Expect(lease.Field(`status.phase`).String()).To(Equal("Bound"))
			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm1")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(Equal("ip-10-10-10-0"))
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.0"))
			Expect(claim.Field(`spec.static`).Bool()).To(BeTrue())
			Expect(claim.Field(`status.phase`).String()).To(Equal("Bound"))

			By("Should create and assign lease to non-static IP address claim")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-1")
			Expect(lease).To(Not(BeEmpty()))
			Expect(lease.Field(`spec.claimRef.name`).String()).To(Equal("vm2"))
			Expect(lease.Field(`spec.claimRef.namespace`).String()).To(Equal("ns1"))
			Expect(lease.Field(`status.phase`).String()).To(Equal("Bound"))
			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm2")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(Equal("ip-10-10-10-1"))
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.1"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Bound"))

			By("Should keep lease with existing IP address claim")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-2")
			Expect(lease).To(Not(BeEmpty()))
			Expect(lease.Field(`spec.claimRef.name`).String()).To(Equal("vm3"))
			Expect(lease.Field(`spec.claimRef.namespace`).String()).To(Equal("ns1"))
			Expect(lease.Field(`status.phase`).String()).To(Equal("Bound"))
			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm3")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Bound"))
			Expect(claim.Field(`spec.leaseName`).String()).To(Equal("ip-10-10-10-2"))
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.2"))

			By("Should remove lease without IP address claim with missing claimRef")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-4")
			Expect(lease).To(BeEmpty())

			By("Should fix missing claim fields")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-5")
			Expect(lease).To(Not(BeEmpty()))
			Expect(lease.Field(`spec.claimRef.name`).String()).To(Equal("vm6"))
			Expect(lease.Field(`spec.claimRef.namespace`).String()).To(Equal("ns1"))
			Expect(lease.Field(`status.phase`).String()).To(Equal("Bound"))
			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm6")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(Equal("ip-10-10-10-5"))
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.5"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Bound"))
			Expect(claim.Field(`spec.static`).Bool()).To(BeTrue())

			By("Should allocate a new lease and fix wrong claim fields with different address specified")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-6")
			Expect(lease).To(BeEmpty())
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-7")
			Expect(lease).To(Not(BeEmpty()))
			Expect(lease.Field(`spec.claimRef.name`).String()).To(Equal("vm7"))
			Expect(lease.Field(`spec.claimRef.namespace`).String()).To(Equal("ns1"))
			Expect(lease.Field(`status.phase`).String()).To(Equal("Bound"))
			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm7")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(Equal("ip-10-10-10-7"))
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.7"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Bound"))

			By("Should keep correct lease and claim without changes")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-8")
			Expect(lease).To(Not(BeEmpty()))
			Expect(lease.Field(`spec.claimRef.name`).String()).To(Equal("vm8"))
			Expect(lease.Field(`spec.claimRef.namespace`).String()).To(Equal("ns1"))
			Expect(lease.Field(`status.phase`).String()).To(Equal("Bound"))
			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm8")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(Equal("ip-10-10-10-8"))
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.8"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Bound"))
		})
	})

	Context("IPAM Wrong cases", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm1
  namespace: ns1
spec:
  static: true
  address: 10.10.10.0
  leaseName: ip-10-10-10-0
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm2
  namespace: ns1
spec:
  static: false
  address: 10.10.10.0
  leaseName: ip-10-10-10-0
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressLease
metadata:
  name: ip-10-10-10-0
spec:
  claimRef:
    name: vm1
    namespace: ns1
status:
  phase: Conflict
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm4
  namespace: ns1
spec:
  static: true
  address: 10.10.10.1
status:
  phase: Conflict
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressLease
metadata:
  name: ip-10-10-10-1
spec:
  claimRef:
    name: vm3
    namespace: ns1
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm5
  namespace: ns1
spec:
  static: true
  address: 10.10.10.3
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm6
  namespace: ns1
spec:
  static: true
  address: 10.10.10.3
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm7
  namespace: ns1
spec:
  static: true
  address: 192.168.1.1
`),
			)
			f.RunHook()
		})

		It("Manages VirtualMachineIPAddressLeases", func() {
			Expect(f).To(ExecuteSuccessfully())
			var lease object_store.KubeObject
			var claim object_store.KubeObject

			By("Should not bound conflicting to claim")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-0")
			Expect(lease).To(Not(BeEmpty()))
			Expect(lease.Field(`spec.claimRef.name`).String()).To(Equal("vm1"))
			Expect(lease.Field(`spec.claimRef.namespace`).String()).To(Equal("ns1"))
			Expect(lease.Field(`status.phase`).String()).To(Equal("Bound"))

			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm1")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(Equal("ip-10-10-10-0"))
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.0"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Bound"))

			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm2")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(BeEmpty())
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.0"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Conflict"))

			By("Transfers IP from removed claim")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-1")
			Expect(lease).To(Not(BeEmpty()))
			Expect(lease.Field(`spec.claimRef.name`).String()).To(Equal("vm4"))
			Expect(lease.Field(`spec.claimRef.namespace`).String()).To(Equal("ns1"))
			Expect(lease.Field(`status.phase`).String()).To(Equal("Bound"))

			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm4")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(Equal("ip-10-10-10-1"))
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.1"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Bound"))

			By("Should not process conflicting lease in one loop")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-10-10-10-3")
			Expect(lease).To(Not(BeEmpty()))
			Expect(lease.Field(`spec.claimRef.name`).String()).To(Equal("vm5"))
			Expect(lease.Field(`spec.claimRef.namespace`).String()).To(Equal("ns1"))
			Expect(lease.Field(`status.phase`).String()).To(Equal("Bound"))

			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm5")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(Equal("ip-10-10-10-3"))
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.3"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Bound"))

			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm6")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(BeEmpty())
			Expect(claim.Field(`spec.address`).String()).To(Equal("10.10.10.3"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("Conflict"))

			By("Should not allocate lease not in range")
			lease = f.KubernetesGlobalResource("VirtualMachineIPAddressLease", "ip-192-168-1-1")
			Expect(lease).To(BeEmpty())
			claim = f.KubernetesResource("VirtualMachineIPAddressClaim", "ns1", "vm7")
			Expect(claim).To(Not(BeEmpty()))
			Expect(claim.Field(`spec.leaseName`).String()).To(BeEmpty())
			Expect(claim.Field(`spec.address`).String()).To(Equal("192.168.1.1"))
			Expect(claim.Field(`status.phase`).String()).To(Equal("OutOfRange"))
			// TODO out of range
		})
	})

})
