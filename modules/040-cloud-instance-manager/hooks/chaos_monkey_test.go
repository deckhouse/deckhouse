package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: chaos_monkey ::", func() {
	const (
		stateCIGSmall = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: toosmall
spec:
status:
  desired: 1
  ready: 1
`
		stateCIGLarge = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: largecig
spec:
  chaos:
    period: 5m
status:
  desired: 3
  ready: 3
`
		stateCIGLargeBroken = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: largecig
spec:
  chaos:
    period: 5m
status:
  desired: 3
  ready: 2
`

		stateNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    cloud-instance-manager.deckhouse.io/cloud-instance-group: largecig
---
apiVersion: v1
kind: Node
metadata:
  name: node2
  labels:
    cloud-instance-manager.deckhouse.io/cloud-instance-group: largecig
---
apiVersion: v1
kind: Node
metadata:
  name: node3
  labels:
    cloud-instance-manager.deckhouse.io/cloud-instance-group: largecig
---
apiVersion: v1
kind: Node
metadata:
  name: smallnode1
  labels:
    cloud-instance-manager.deckhouse.io/cloud-instance-group: toosmall
`
		stateMachines = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: node1
  namespace: d8-cloud-instance-manager
  labels:
    node: node1
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: node2
  namespace: d8-cloud-instance-manager
  labels:
    node: node2
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: node3
  namespace: d8-cloud-instance-manager
  labels:
    node: node3
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: smallnode1
  namespace: d8-cloud-instance-manager
  labels:
    node: smallnode1
`
		stateMachineVictim = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: victimnode
  namespace: d8-cloud-instance-manager
  labels:
    cloud-instance-manager.deckhouse.io/cloud-instance-group: somecig
    cloud-instance-manager.deckhouse.io/chaos-monkey-victim: ""
    node: victimnode
`
	)

	f := HookExecutionConfigInit(`{"cloudInstanceManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "CloudInstanceGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with cigs ready for chaos", func() {
		BeforeEach(func() {
			f.KubeStateSet(stateCIGSmall + stateCIGLarge + stateNodes + stateMachines)
			f.BindingContexts.Set(f.RunSchedule("* * * * *"))
			f.AddHookEnv("RANDOM_SEED=7")
			f.RunHook()
		})

		It("Hook is lucky to run monkey. One machine must be deleted.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node3").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "smallnode1").Exists()).To(BeTrue())
		})
	})

	Context("Cluster with broken large cig", func() {
		BeforeEach(func() {
			f.KubeStateSet(stateCIGSmall + stateCIGLargeBroken + stateNodes + stateMachines)
			f.BindingContexts.Set(f.RunSchedule("* * * * *"))
			f.AddHookEnv("RANDOM_SEED=7")
			f.RunHook()
		})

		It("Hook is lucky to run monkey. All machines must survive.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node3").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "smallnode1").Exists()).To(BeTrue())
		})
	})

	Context("Cluster with large ready cig and victim machine", func() {
		BeforeEach(func() {
			f.KubeStateSet(stateCIGSmall + stateCIGLarge + stateNodes + stateMachines + stateMachineVictim)
			f.BindingContexts.Set(f.RunSchedule("* * * * *"))
			f.AddHookEnv("RANDOM_SEED=7")
			f.RunHook()
		})

		It("Hook is lucky to run monkey. All machines must survive.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node3").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "smallnode1").Exists()).To(BeTrue())
		})
	})

	Context("Hook isn't lucky to run monkey. All machines must survive.", func() {
		BeforeEach(func() {
			f.KubeStateSet(stateCIGSmall + stateCIGLarge + stateNodes + stateMachines)
			f.BindingContexts.Set(f.RunSchedule("* * * * *"))
			f.AddHookEnv("RANDOM_SEED=0")
			f.RunHook()
		})

		It("", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node3").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "smallnode1").Exists()).To(BeTrue())
		})
	})
})
