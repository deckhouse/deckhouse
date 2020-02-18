/*

User-stories:
1. There are special nodes for kube-dns in cluster — hook must fit kube-dns deployment to this nodes and masters. If there is kube-dns-autoscaler in cluster then hook must keep replicas.
2. There aren't dedicated dns-nodes, but there are special system-nodes in cluster — hook must fit kube-dns deployment to this nodes and masters. If there is kube-dns-autoscaler in cluster then hook must keep replicas.
3. There aren't special nodes — hook must fit kube-dns deployment to this nodes. Replicas must be counted by formula: ([([2,<count_master_nodes>,<original_replicas>] | max), ([2, '<count_master_nodes + count_nonspecific_nodes>'] | max)] | min).
4. kube-dns deployment should aim to fit pods to different nodes.
5. If there are empty fields in affinity then hook must delete them.

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

var _ = Describe("Global hooks :: policies/kube_dns ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateKubeDnsDeployment = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
spec:
  replicas: 42
  template:
    spec:
      tolerations:
      - some: toleration
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions: [{"a": "b"}]
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 1
              preference:
                matchExpressions: [{"a": "b"}]
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - some: preferredantiaffinity
          requiredDuringSchedulingIgnoredDuringExecution:
          - some: requiredantiaffinity
`

		stateKubeDnsAutoscalerDeployment = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-kube-dns-autoscaler
  namespace: kube-system
  labels:
    k8s-app: kube-dns-autoscaler
`

		stateMaster = `
---
apiVersion: v1
kind: Node
metadata:
  name: master
  labels:
    node-role.kubernetes.io/master: ""
`
		stateNonSpecificNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: slacker-1
---
apiVersion: v1
kind: Node
metadata:
  name: slacker-2
---
apiVersion: v1
kind: Node
metadata:
  name: slacker-3
---
apiVersion: v1
kind: Node
metadata:
  name: slacker-4
---
apiVersion: v1
kind: Node
metadata:
  name: slacker-5

`

		stateThreeSystemNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: system-1
  labels:
    node-role.deckhouse.io/system: ""
---
apiVersion: v1
kind: Node
metadata:
  name: system-2
  labels:
    node-role.kubernetes.io/system: ""
---
apiVersion: v1
kind: Node
metadata:
  name: system-3
  labels:
    node-role.flant.com/system: ""
`
		stateFourKubeDnsNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: dns-1
  labels:
    node-role.deckhouse.io/kube-dns: ""
---
apiVersion: v1
kind: Node
metadata:
  name: dns-2
  labels:
    node-role.kubernetes.io/kube-dns: ""
---
apiVersion: v1
kind: Node
metadata:
  name: dns-3
  labels:
    node-role.flant.com/kube-dns: ""
---
apiVersion: v1
kind: Node
metadata:
  name: dns-4
  labels:
    node-role.flant.com/kube-dns: ""
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("There is only master in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMaster))
			f.RunHook()
		})

		It("expectations — snapshots: [1,0,0]", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(len(f.BindingContexts.Get("0.snapshots.node_roles").Array())).To(Equal(1))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns").Array())).To(Equal(0))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns_autoscaler").Array())).To(Equal(0))

			Expect(f.Session.Err).Should(gbytes.Say("WARNING: Can't find kube-dns deployment."))
		})
	})

	Context("There is only master and kube-dns Deployment in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMaster + stateKubeDnsDeployment))
			f.RunHook()
		})

		It("expectations — snapshots: [1,1,0], replicas: 2, tolerations: keep original, empty affinity: wipe, nodeAffinity: wipe, podAntiAffinity: fit different nodes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(len(f.BindingContexts.Get("0.snapshots.node_roles").Array())).To(Equal(1))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns").Array())).To(Equal(1))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns_autoscaler").Array())).To(Equal(0))

			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.replicas").String()).To(Equal("2"))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.tolerations").String()).To(MatchJSON(`[{"key":"node-role.kubernetes.io/master"},{"key":"node-role/system"},{"key":"dedicated.flant.com","operator":"Equal","value":"kube-dns"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"},{"some":"toleration"}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution").String()).To(MatchJSON(`[{"weight":1,"preference":{"matchExpressions":[{"key":"node-role.kubernetes.io/master","operator":"Exists"}]}}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution").String()).To(MatchJSON(`[{"weight":1,"podAffinityTerm":{"labelSelector":{"matchLabels":{"k8s-app":"kube-dns"}},"topologyKey":"kubernetes.io/hostname"}}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
		})
	})

	Context("There is only master, kube-dns Deployment and five non-specific nodes in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMaster + stateKubeDnsDeployment + stateNonSpecificNode))
			f.RunHook()
		})

		It("expectations — snapshots: [6,1,0], replicas: 5, tolerations: keep original, empty affinity: wipe, nodeAffinity: wipe, podAntiAffinity: fit different nodes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(len(f.BindingContexts.Get("0.snapshots.node_roles").Array())).To(Equal(6))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns").Array())).To(Equal(1))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns_autoscaler").Array())).To(Equal(0))

			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.replicas").String()).To(Equal("6"))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.tolerations").String()).To(MatchJSON(`[{"key":"node-role.kubernetes.io/master"},{"key":"node-role/system"},{"key":"dedicated.flant.com","operator":"Equal","value":"kube-dns"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"},{"some":"toleration"}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution").String()).To(MatchJSON(`[{"weight":1,"preference":{"matchExpressions":[{"key":"node-role.kubernetes.io/master","operator":"Exists"}]}}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution").String()).To(MatchJSON(`[{"weight":1,"podAffinityTerm":{"labelSelector":{"matchLabels":{"k8s-app":"kube-dns"}},"topologyKey":"kubernetes.io/hostname"}}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
		})
	})

	Context("There is master, kube-dns Deployment and three system-nodes in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMaster + stateKubeDnsDeployment + stateThreeSystemNodes))
			f.RunHook()
		})

		It("expectations — snapshots: [4,1,0], replicas: 3, tolerations: original + tolerate d8-specific nodes, empty affinity: wipe, nodeAffinity: schedule to system-nodes, podAntiAffinity: fit different nodes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(len(f.BindingContexts.Get("0.snapshots.node_roles").Array())).To(Equal(4))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns").Array())).To(Equal(1))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns_autoscaler").Array())).To(Equal(0))

			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.replicas").String()).To(Equal("3"))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.tolerations").String()).To(MatchJSON(`[{"key":"node-role.kubernetes.io/master"},{"key":"node-role/system"},{"key":"dedicated.flant.com","operator":"Equal","value":"kube-dns"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"},{"some":"toleration"}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.emptyStuff").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms").String()).To(MatchJSON(`[{"matchExpressions":[{"key":"node-role.kubernetes.io/master","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.flant.com/system","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.deckhouse.io/system","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.kubernetes.io/system","operator":"Exists"}]}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution").String()).To(MatchJSON(`[{"labelSelector":{"matchLabels":{"k8s-app":"kube-dns"}},"topologyKey":"kubernetes.io/hostname"}]`))
		})
	})

	Context("There is master, kube-dns Deployment, three system-nodes and four dns-nodes in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMaster + stateKubeDnsDeployment + stateThreeSystemNodes + stateFourKubeDnsNodes))
			f.RunHook()
		})

		It("expectations — snapshots: [8,1,0], replicas: 4, tolerations: original + tolerate d8-specific nodes, empty affinity: wipe, nodeAffinity: schedule to dns-nodes, podAntiAffinity: fit different nodes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(len(f.BindingContexts.Get("0.snapshots.node_roles").Array())).To(Equal(8))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns").Array())).To(Equal(1))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns_autoscaler").Array())).To(Equal(0))

			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.replicas").String()).To(Equal("5"))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.tolerations").String()).To(MatchJSON(`[{"key":"node-role.kubernetes.io/master"},{"key":"node-role/system"},{"key":"dedicated.flant.com","operator":"Equal","value":"kube-dns"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"},{"some":"toleration"}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.emptyStuff").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms").String()).To(MatchJSON(`[{"matchExpressions":[{"key":"node-role.kubernetes.io/master","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.flant.com/kube-dns","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.deckhouse.io/kube-dns","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.kubernetes.io/kube-dns","operator":"Exists"}]}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution").String()).To(MatchJSON(`[{"labelSelector":{"matchLabels":{"k8s-app":"kube-dns"}},"topologyKey":"kubernetes.io/hostname"}]`))
		})
	})

	Context("There is master, kube-dns Deployment, three system-nodes, four dns-nodes and kube-dns-autoscaler in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMaster + stateKubeDnsDeployment + stateThreeSystemNodes + stateFourKubeDnsNodes + stateKubeDnsAutoscalerDeployment))
			f.RunHook()
		})

		It("expectations — snapshots: [8,1,0], replicas: keep original '42', tolerations: original + tolerate d8-specific nodes, empty affinity: wipe, nodeAffinity: schedule to dns-nodes, podAntiAffinity: fit different nodes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(len(f.BindingContexts.Get("0.snapshots.node_roles").Array())).To(Equal(8))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns").Array())).To(Equal(1))
			Expect(len(f.BindingContexts.Get("0.snapshots.kube_dns_autoscaler").Array())).To(Equal(1))

			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.replicas").String()).To(Equal("42"))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.tolerations").String()).To(MatchJSON(`[{"key":"node-role.kubernetes.io/master"},{"key":"node-role/system"},{"key":"dedicated.flant.com","operator":"Equal","value":"kube-dns"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"},{"some":"toleration"}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms").String()).To(MatchJSON(`[{"matchExpressions":[{"key":"node-role.kubernetes.io/master","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.flant.com/kube-dns","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.deckhouse.io/kube-dns","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.kubernetes.io/kube-dns","operator":"Exists"}]}]`))
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "kube-system", "my-kube-dns").Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution").String()).To(MatchJSON(`[{"labelSelector":{"matchLabels":{"k8s-app":"kube-dns"}},"topologyKey":"kubernetes.io/hostname"}]`))
		})
	})
})
