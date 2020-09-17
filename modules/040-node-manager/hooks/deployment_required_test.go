package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: deployment_required ::", func() {
	const (
		nodeGroupHybrid = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Hybrid
status: {}
`
		nodeGroupCloud = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Cloud
status: {}
`
		machineDeployment = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: aaa
  labels:
    heritage: deckhouse
  name: machine-deployment-name
  namespace: d8-cloud-instance-manager
`
		machineSet = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineSet
metadata:
  annotations:
    zone: aaa
  name: machine-set-name
  namespace: d8-cloud-instance-manager
`
		machine = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-name
  namespace: d8-cloud-instance-manager
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.16.15", "kubernetesVersions":["1.16.15"]},"clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineSet", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail; flag must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with Hybrid NG only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupHybrid))
			f.RunHook()
		})

		It("Hook must not fail; flag must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with Cloud NG only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupCloud))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").String()).To(Equal("true"))
		})
	})

	Context("Cluster with MDs, MSs and Ms", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machineDeployment + machineSet + machine))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").String()).To(Equal("true"))
		})
	})

	Context("Cluster with MDs only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machineDeployment))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").String()).To(Equal("true"))
		})
	})

	Context("Cluster with MDs and MSs only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machineDeployment + machineSet))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").String()).To(Equal("true"))
		})
	})

})
