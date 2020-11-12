package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-kubernetes :: hooks :: migrate_domain_controllers ::", func() {
	const (
		stateStetefulSetWithProperLabel = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: application
  namespace: default
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.flant.com/production
                operator: In
                values:
                - ""
`
		stateStetefulSetWithProperNodeSelector = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: application
  namespace: default
spec:
  template:
    spec:
      nodeSelector:
        node-role.flant.com: postgresnode
      tolerations:
      - operator: Exists
        value: system
`
		stateStetefulSetWithProperTollerations = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: application
  namespace: default
spec:
  template:
    spec:
      tolerations:
      - key: dedicated.deckhouse.io
        value: system
        operator: Exists
`
		stateStetefulSetWithOldLabel = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: old-label-application
  namespace: default
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.flant.com/system
                operator: In
                values:
                - ""
`
		stateStetefulSetWithOldToleration = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: old-toleration-application
  namespace: default
spec:
  template:
    spec:
      tolerations:
      - key: dedicated.flant.com
        operator: Equal
        value: system
`
		stateDaemonSetWithProperLabel = `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: application
  namespace: default
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.flant.com/production
                operator: In
                values:
                - ""
`
		stateDaemonSetWithOldLabel = `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: old-label-application
  namespace: default
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.flant.com/system
                operator: In
                values:
                - ""
`
		stateDaemonSetWithOldToleration = `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: old-toleration-application
  namespace: default
spec:
  template:
    spec:
      tolerations:
      - key: dedicated.flant.com
        operator: Equal
        value: system
`

		stateDeploymentWithProperLabel = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: application
  namespace: default
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.flant.com/production
                operator: In
                values:
                - ""
`
		stateDeploymentWithOldLabel = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: old-label-application
  namespace: default
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.flant.com/system
                operator: In
                values:
                - ""
`
		stateDeploymentWithOldToleration = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: old-toleration-application
  namespace: default
spec:
  template:
    spec:
      tolerations:
      - key: dedicated.flant.com
        operator: Equal
        value: system
`
		stateDeploymentWithProperNodeSelector = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: proper-selector-application
  namespace: default
spec:
  template:
    spec:
      nodeSelector:
        node-role.flant.com/production: ""
`
		stateDeploymentWithOldNodeSelector = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: old-selector-application
  namespace: default
spec:
  template:
    spec:
      nodeSelector:
        node-role.flant.com/system: ""
`
		stateDeploymentWhichToleratesAll = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: all-toleration-application
  namespace: default
spec:
  template:
    spec:
      tolerations:
      - key: dedicated.flant.com
`
	)
	f := HookExecutionConfigInit(
		`{"monitoringKubernetes":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	// StatefulSet

	Context("Cluster containing StatefulSet with proper label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateStetefulSetWithProperLabel))
			f.RunHook()
		})

		It("Hook must not fail, node selector should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.statefulsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "StatefulSet",
  "name": "application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"production"
  ]
}
`))
		})
	})

	Context("Cluster containing StatefulSet with proper node selector", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateStetefulSetWithProperNodeSelector))
			f.RunHook()
		})

		It("Hook must not fail, node selector should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.statefulsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "StatefulSet",
  "name": "application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": []
}
`))
		})
	})

	Context("Cluster containing StatefulSet with proper tolerations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateStetefulSetWithProperTollerations))
			f.RunHook()
		})

		It("Hook must not fail, toleration should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.statefulsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "StatefulSet",
  "name": "application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": []
}
`))
		})
	})

	Context("Cluster with StatefulSet having old label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateStetefulSetWithOldLabel))
			f.RunHook()
		})

		It("Hook must not fail, node selector should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.statefulsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "StatefulSet",
  "name": "old-label-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
		})
	})

	Context("Cluster with StatefulSet having old toleration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateStetefulSetWithOldToleration))
			f.RunHook()
		})

		It("Hook must not fail, toleration should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.statefulsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "StatefulSet",
  "name": "old-toleration-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
		})
	})

	Context("Cluster with two StatefulSets one having old toleration and another having old label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateStetefulSetWithOldLabel + stateStetefulSetWithOldToleration))
			f.RunHook()
		})

		It("Hook must not fail, node selector and toleration should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.statefulsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "StatefulSet",
  "name": "old-label-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
			Expect(f.BindingContexts.Get("0.snapshots.statefulsets.1.filterResult").String()).To(MatchJSON(`
{
  "kind": "StatefulSet",
  "name": "old-toleration-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
		})
	})

	// DaemonSet

	Context("Cluster with proper DaemonSet", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDaemonSetWithProperLabel))
			f.RunHook()
		})

		It("Hook must not fail, node selector should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.daemonsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "DaemonSet",
  "name": "application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"production"
  ]
}
`))
		})
	})

	Context("Cluster with DaemonSet having old label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDaemonSetWithOldLabel))
			f.RunHook()
		})

		It("Hook must not fail, node selector should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.daemonsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "DaemonSet",
  "name": "old-label-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
		})
	})

	Context("Cluster with DaemonSet having old toleration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDaemonSetWithOldToleration))
			f.RunHook()
		})

		It("Hook must not fail, toleration should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.daemonsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "DaemonSet",
  "name": "old-toleration-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
		})
	})

	Context("Cluster with two DaemonSets one having old toleration and another having old label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDaemonSetWithOldLabel + stateDaemonSetWithOldToleration))
			f.RunHook()
		})

		It("Hook must not fail, node selector and toleration should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.daemonsets.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "DaemonSet",
  "name": "old-label-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
			Expect(f.BindingContexts.Get("0.snapshots.daemonsets.1.filterResult").String()).To(MatchJSON(`
{
  "kind": "DaemonSet",
  "name": "old-toleration-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
		})
	})

	// Deployment

	Context("Cluster with proper Deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeploymentWithProperLabel))
			f.RunHook()
		})

		It("Hook must not fail, node selector should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.deployments.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "Deployment",
  "name": "application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"production"
  ]
}
`))
		})
	})

	Context("Cluster with Deployment having old label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeploymentWithOldLabel))
			f.RunHook()
		})

		It("Hook must not fail, label should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.deployments.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "Deployment",
  "name": "old-label-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
		})
	})

	Context("Cluster with Deployment having old toleration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeploymentWithOldToleration))
			f.RunHook()
		})

		It("Hook must not fail, toleration should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.deployments.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "Deployment",
  "name": "old-toleration-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
		})
	})

	Context("Cluster with two Deployments one having old toleration and another having old label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeploymentWithOldLabel + stateDeploymentWithOldToleration))
			f.RunHook()
		})

		It("Hook must not fail, labels and tolerations should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.deployments.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "Deployment",
  "name": "old-label-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
			Expect(f.BindingContexts.Get("0.snapshots.deployments.1.filterResult").String()).To(MatchJSON(`
{
  "kind": "Deployment",
  "name": "old-toleration-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"system"
  ]
}
`))
		})
	})

	// Deployment with spec.nodeSelector

	Context("Cluster with proper Deployment spec.nodeSelector", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeploymentWithProperNodeSelector))
			f.RunHook()
		})

		It("Hook must not fail, node selector should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.deployments.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "Deployment",
  "name": "proper-selector-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"production"
  ]
}
`))
		})
	})

	Context("Cluster with Deployment having old nodeSelector", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeploymentWithOldNodeSelector))
			f.RunHook()
		})

		It("Hook must not fail, node selector should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.deployments.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "Deployment",
  "name": "old-selector-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
    "system"
  ]
}
`))
		})
	})

	Context("Cluster with Deployment which tolerates all", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeploymentWhichToleratesAll))
			f.RunHook()
		})

		It("Hook must not fail, all toleration should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.deployments.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "Deployment",
  "name": "all-toleration-application",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
    "_wildcard_"
  ]
}
`))
		})
	})

})
