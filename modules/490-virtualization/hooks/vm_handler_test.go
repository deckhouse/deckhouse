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

var _ = Describe("Modules :: virtualization :: hooks :: vm_handler ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "VirtualMachineDisk", true)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "VirtualMachine", true)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "VirtualMachineIPAddressClaim", true)
	f.RegisterCRD("kubevirt.io", "v1", "VirtualMachine", true)

	// Set Kind for binding.
	vmHandlerHookConfig.Kubernetes[0].Kind = "VirtualMachine"
	vmHandlerHookConfig.Kubernetes[0].ApiVersion = "kubevirt.io/v1"

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

	Context("VMS creation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: mysql
  namespace: default
spec:
  static: false
  address: 10.10.10.2
  leaseName: ip-10-10-10-2
status:
  phase: Bound
  vmName: vm1
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm1
  namespace: default
  labels:
    aaa: bbb
  annotations:
    some: value
spec:
  nodeSelector:
    disktype: ssd
  tolerations:
  - key: "key1"
    operator: "Equal"
    value: "value1"
    effect: "NoSchedule"
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  ipAddressClaimName: mysql
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-20.04
    size: 10Gi
    storageClassName: linstor-slow
    autoDelete: true
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineDisk
metadata:
  name: mydata
  namespace: ns1
spec:
  source:
    kind: ClusterVirtualMachineImage
    name: centos-7
  storageClassName: linstor-slow
  size: 10Gi
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm2
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-20.04
    size: 10Gi
    storageClassName: linstor-slow
    autoDelete: false
  cloudInit:
    userData: |-
      chpasswd: { expire: False }
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm2
  namespace: default
spec:
  static: false
  address: 10.10.10.3
  leaseName: ip-10-10-10-3
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineDisk
metadata:
  name: vm2-boot
  namespace: default
spec:
  source:
    kind: ClusterVirtualMachineImage
    name: ubuntu-20.04
  storageClassName: linstor-slow
  size: 10Gi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: disk-vm2-boot
  namespace: default
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineDisk
metadata:
  name: vm3-boot
  namespace: default
spec:
  source:
    kind: ClusterVirtualMachineImage
    name: ubuntu-20.04
  storageClassName: linstor-slow
  size: 10Gi
status:
  ephemeral: false
  vmName: vm3
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: disk-vm3-boot
  namespace: default
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm4
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-20.04
    size: 12Gi
    storageClassName: linstor-slow
  cloudInit:
    userData: |-
      chpasswd: { expire: False }
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineDisk
metadata:
  name: vm4-boot
  namespace: default
spec:
  source:
    kind: ClusterVirtualMachineImage
    name: ubuntu-20.04
  storageClassName: linstor-slow
  size: 11Gi
status:
  ephemeral: false
  vmName: vm4
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: disk-vm4-boot
  namespace: default
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm4a
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-20.04
    size: 10Gi
    storageClassName: linstor-slow
  cloudInit:
    userData: |-
      chpasswd: { expire: False }
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineDisk
metadata:
  name: vm4a-boot
  namespace: default
spec:
  source:
    kind: ClusterVirtualMachineImage
    name: ubuntu-20.04
  storageClassName: linstor-slow
  size: 11Gi
status:
  ephemeral: false
  vmName: vm4a
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: disk-vm4a-boot
  namespace: default
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm5
  namespace: default
spec:
  address: 10.10.10.10
  leaseName: ip-10-10-10-10
  static: true
status:
  phase: Bound
  vmName: vm5
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineDisk
metadata:
  name: vm5-boot
  namespace: default
spec:
  size: 10Gi
  source:
    kind: ClusterVirtualMachineImage
    name: ubuntu-22.04
  storageClassName: linstor-fast
status:
  ephemeral: false
  vmName: vm5
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm6
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    name: foo
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineDisk
metadata:
  name: foo
  namespace: default
spec:
  size: 10Gi
  source:
    kind: ClusterVirtualMachineImage
    name: ubuntu-22.04
  storageClassName: linstor-fast
status:
  ephemeral: false
  vmName: vm6
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm7
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    name: foo
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm8
  namespace: default
spec:
  static: false
  address: 10.10.10.8
  leaseName: ip-10-10-10-8
status:
  phase: Bound
---
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm9
  namespace: default
spec:
  static: true
  address: 10.10.10.9
  leaseName: ip-10-10-10-9
status:
  phase: Bound
`),
			)
			f.RunHook()
		})

		It("Manages VirtualMachine and boot Disk", func() {
			Expect(f).To(ExecuteSuccessfully())
			disk := f.KubernetesResource("VirtualMachineDisk", "default", "vm1-boot")
			Expect(disk).To(Not(BeEmpty()))
			Expect(disk.Field(`status.vmName`).String()).To(Equal("vm1"))
			Expect(disk.Field(`status.ephemeral`).Bool()).To(BeTrue())
			vm := f.KubernetesResource("virtualmachines.kubevirt.io", "default", "vm1")
			Expect(vm).To(Not(BeEmpty()))
			Expect(vm.Field(`apiVersion`).String()).To(Equal("kubevirt.io/v1"))
			Expect(vm.Field(`spec.template.metadata.labels.aaa`).String()).To(Equal("bbb"))
			Expect(vm.Field(`spec.template.metadata.annotations.some`).String()).To(Equal("value"))
			Expect(vm.Field(`spec.template.spec.nodeSelector.disktype`).String()).To(Equal("ssd"))
			Expect(vm.Field(`spec.template.spec.tolerations`).Array()).To(HaveLen(1))
			Expect(vm.Field(`spec.template.spec.volumes`).Array()).To(HaveLen(2))

			d8vm := f.KubernetesResource("VirtualMachine", "default", "vm1")
			Expect(d8vm).To(Not(BeEmpty()))
			Expect(d8vm.Field(`apiVersion`).String()).To(Equal("deckhouse.io/v1alpha1"))
			Expect(d8vm.Field(`status.ipAddress`).String()).To(Equal("10.10.10.2"))

			ipClaim := f.KubernetesResource("VirtualMachineIPAddressClaim", "default", "mysql")
			Expect(ipClaim).To(Not(BeEmpty()))

			By("should update fields for existing disk")
			disk2 := f.KubernetesResource("VirtualMachineDisk", "default", "vm2-boot")
			Expect(disk2).To(Not(BeEmpty()))
			Expect(disk2.Field(`status.vmName`).String()).To(Equal("vm2"))
			Expect(disk2.Field(`status.ephemeral`).Bool()).To(BeFalse())

			By("should keep ephemeral disks")
			disk3 := f.KubernetesResource("VirtualMachineDisk", "default", "vm3-boot")
			Expect(disk3).To(Not(BeEmpty()))
			Expect(disk3.Field(`status.vmName`).String()).To(Equal(""))

			By("should resize existing disk")
			disk4 := f.KubernetesResource("VirtualMachineDisk", "default", "vm4-boot")
			Expect(disk4).To(Not(BeEmpty()))
			Expect(disk4.Field(`spec.size`).String()).To(Equal("12Gi"))

			By("should not resize existing disk")
			disk4a := f.KubernetesResource("VirtualMachineDisk", "default", "vm4a-boot")
			Expect(disk4a).To(Not(BeEmpty()))
			Expect(disk4a.Field(`spec.size`).String()).To(Equal("11Gi"))

			By("Should release static VirtualMachineIPAddressClaim")
			ipClaim5 := f.KubernetesResource("VirtualMachineIPAddressClaim", "default", "vm5")
			Expect(ipClaim5).To(Not(BeEmpty()))
			Expect(ipClaim5.Field(`status.vmName`).String()).To(BeEmpty())

			By("Should release non ephemeral VirtualMachineDisk")
			disk5 := f.KubernetesResource("VirtualMachineDisk", "default", "vm5-boot")
			Expect(disk5).To(Not(BeEmpty()))
			Expect(disk5.Field(`status.vmName`).String()).To(BeEmpty())

			By("Should keep VirtualMachine and non ephemeral VirtualMachineDisk")
			d8vm6 := f.KubernetesResource("VirtualMachine", "default", "vm6")
			Expect(d8vm6).To(Not(BeEmpty()))

			disk6 := f.KubernetesResource("VirtualMachineDisk", "default", "foo")
			Expect(disk6).To(Not(BeEmpty()))
			Expect(disk6.Field(`status.vmName`).String()).To(Equal("vm6"))

			By("Should not allow to run VirtualMachine with VirtualMachineDisk attached to other VirtualMachine")
			vm7 := f.KubernetesResource("virtualmachines.kubevirt.io", "default", "vm7")
			Expect(vm7).To(BeEmpty())

			// TODO: should we remove non-static ip automatically?
			// By("Should remove non static VirtualMachineIPAddressClaim without assigned VirtualMachine")
			// d8vm8 := f.KubernetesResource("VirtualMachineIPAddressClaim", "default", "vm8")
			// Expect(d8vm8).To(BeEmpty())

			By("Should keep static VirtualMachineIPAddressClaim without assigned VirtualMachine")
			d8vm9 := f.KubernetesResource("VirtualMachineIPAddressClaim", "default", "vm9")
			Expect(d8vm9).To(Not(BeEmpty()))
		})
	})

	// TODO ownerReferrences tests

})
