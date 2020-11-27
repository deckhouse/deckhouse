/*

User-stories:
1. If there is other ready nodes in addition to master-nodes, we can assume that the cluster has been bootstrapped.

*/

package hooks

import (
	"testing"

	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	initValuesString       = `{"global": {}}`
	initConfigValuesString = `{}`
)

const (
	stateMasterOnly = `
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
status:
  conditions:
  - status: "True"
    type: Ready
`
	stateMasterAndNotReadyNode = `
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-worker-1
spec:
taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/worker
status:
  conditions:
  - status: "False"
    type: Ready
`

	stateMasterAndReadyNode = `
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-worker-1
spec:
taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/worker
status:
  conditions:
  - status: "True"
    type: Ready
`

	stateMasterAndCM = `
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-is-bootstraped
  namespace: kube-system
`
)

var _ = Describe("Global hooks :: cluster_is_bootstraped ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster has no nodes except master", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterOnly))
			f.RunHook()
		})

		It("filterResult must be 'false'; `global.clusterIsBootstrapped` must not exist", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.clusterIsBootstrapped").Exists()).To(BeFalse())
		})

		Context("Worker node with status NotReady added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMasterAndNotReadyNode))
				f.RunHook()
			})

			It("'filterResult' must be false; `global.clusterIsBootstrapped` must not exist", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.clusterIsBootstrapped").Exists()).To(BeFalse())
			})

			Context("State of additional node changed to Ready", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateMasterAndReadyNode))
					f.RunHook()
				})

				It("filterResult must be 'true'; `global.clusterIsBootstrapped` must be 'true'; CM `d8-cluster-is-bootstraped` must be created", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.clusterIsBootstrapped").Bool()).To(BeTrue())
					Expect(f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-is-bootstraped").Exists()).To(BeTrue())
				})
			})
		})

		Context("Someone created cm kube-system/d8-cluster-is-bootstraped", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMasterAndCM))
				f.RunHook()
			})

			It("`global.clusterIsBootstrapped` must be 'true'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.clusterIsBootstrapped").Bool()).To(BeTrue())
			})
		})
	})

	Context("Cluster has master and additional nodes in NotReady state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndNotReadyNode))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
		})

		It("filterResult must be 'false'; `global.clusterIsBootstrapped` must not exist", func() {
			Expect(f.ValuesGet("global.clusterIsBootstrapped").Exists()).To(BeFalse())
		})
	})

	Context("Cluster has master and additional nodes in Ready state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndReadyNode))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
		})

		It("BINDING_CONTEXT must have Synchronization event with two objects with filterResult 'false' and 'true'; `global.clusterIsBootstrapped` must be 'true'", func() {
			Expect(f.ValuesGet("global.clusterIsBootstrapped").Bool()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-is-bootstraped").Exists()).To(BeTrue())
		})
	})

	Context("Cluster has cm kube-system/d8-cluster-is-bootstraped", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndCM))
			f.RunHook()
		})

		It("`global.clusterIsBootstrapped` must be 'true'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.clusterIsBootstrapped").Bool()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-is-bootstraped").Exists()).To(BeTrue())
		})

		Context("CM kube-system/d8-cluster-is-bootstraped deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMasterOnly))
				f.RunHook()
			})

			It("Hook must fail", func() {
				Expect(f).To(Not(ExecuteSuccessfully()))
				Expect(f.Session.Err).Should(gbytes.Say("ERROR: CM kube-system/d8-cluster-is-bootstraped was deleted. Don't know what to do."))
			})
		})
	})
})
