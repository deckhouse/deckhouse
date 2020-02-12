package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: update_cloud_instance_group_status ::", func() {
	const (
		stateCIG1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroups
metadata:
  name: cig1
spec:
  maxInstancesPerZone: 5
  minInstancesPerZone: 1
status:
  extra: thing
`
		stateCIG2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroups
metadata:
  name: cig2
spec:
  maxInstancesPerZone: 3
  minInstancesPerZone: 2
  zones: [a, b, c]
status: {}
`

		stateMDs = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-cig1
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig1
spec:
  replicas: 2
`
		stateMachines = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-cig1-aaa
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig1-nova
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-cig1-bbb
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig1-nova
`

		stateNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-cig1-aaa
  labels:
    cloud-instance-manager.deckhouse.io/cloud-instance-group: cig1
status:
  conditions:
  - some: thing
  - status: "False"
    type: Ready
  - some: thing
---
apiVersion: v1
kind: Node
metadata:
  name: node-cig1-bbb
  labels:
    cloud-instance-manager.deckhouse.io/cloud-instance-group: cig1
status:
  conditions:
  - some: thing
  - status: "True"
    type: Ready
`
		stateCloudProviderSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-instance-manager-cloud-provider
  namespace: kube-system
data:
  zones: WyJub3ZhIl0= # ["nova"]
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "CloudInstanceGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: can't find '.data.zones' in secret kube-system/d8-cloud-instance-manager-cloud-provider.`))
		})
	})

	Context("A CIG1 and zones Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIG1 + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Min and max must be filled", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "cig1").Field("status").String()).To(MatchJSON(`{"extra":"thing","max":5,"min":1,"desired":0,"machines":0,"nodes":0,"ready":0}`))
		})
	})

	Context("CIGs MD, Machines, Nodes and zones Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIG1 + stateCIG2 + stateMDs + stateMachines + stateNodes + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Min, max, desired, machines, nodes, ready must be filled", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "cig1").Field("status").String()).To(MatchJSON(`{"extra":"thing","max":5,"min":1,"desired":2,"machines":2,"nodes":2,"ready":1}`))
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "cig2").Field("status").String()).To(MatchJSON(`{"max":9,"min":6,"desired":0,"machines":0,"nodes":0,"ready":0}`))
		})
	})
})
