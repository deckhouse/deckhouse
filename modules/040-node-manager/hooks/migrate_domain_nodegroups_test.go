package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: migrate_domain_nodegroups ::", func() {
	const (
		properResources = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: system
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: frontend
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/frontend: ""
    taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: frontend-0
spec:
  nodeTemplate:
    labels:
      node-role.flant.com/production: ""
    taints:
    - effect: NoExecute
      key: dedicated.flant.com
      value: production
`
		resourcesWithOldLabels = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
      node-role.flant.com/system: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: frontend
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/frontend: ""
      node-role.flant.com/frontend: ""
`
		resourcesWithOldTaints = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeTemplate:
    taints:
    - effect: NoExecute
      key: dedicated.flant.com
      value: system
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: frontend
spec:
  nodeTemplate:
    taints:
    - effect: NoExecute
      key: dedicated.flant.com
      value: frontend
`
	)
	f := HookExecutionConfigInit(
		`{}`,
		`{}`,
	)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster containing proper resources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(properResources))
			f.RunHook()
		})

		It("Hook must not fail, no metrics should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.nodegroups.0.filterResult.labels").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with resources having old `nodeTemplate.labels`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithOldLabels))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.nodegroups.0.filterResult.labels").String()).To(MatchJSON(`{"name":"frontend"}`))
			Expect(f.BindingContexts.Get("0.snapshots.nodegroups.1.filterResult.labels").String()).To(MatchJSON(`{"name":"system"}`))
		})
	})

	Context("Cluster with resources having old `nodeTemplate.taints`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithOldTaints))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.nodegroups.0.filterResult.labels").String()).To(MatchJSON(`{"name":"frontend"}`))
			Expect(f.BindingContexts.Get("0.snapshots.nodegroups.1.filterResult.labels").String()).To(MatchJSON(`{"name":"system"}`))
		})
	})

})
