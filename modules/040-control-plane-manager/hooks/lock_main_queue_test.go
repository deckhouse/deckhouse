package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controler-plane-manager :: hooks :: lock_main_queue ::", func() {
	const (
		initValuesString       = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		initConfigValuesString = ``
		runningReadyPods       = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
    pod-template-generation: "105"
  name: d8-control-plane-manager-a
  namespace: kube-system
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
    pod-template-generation: "105"
  name: d8-control-plane-manager-b
  namespace: kube-system
status:
  conditions:
  - type: Ready
    status: 'False'
---
`
		runningNotReadyPods = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
    pod-template-generation: "105"
  name: d8-control-plane-manager-a
  namespace: kube-system
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
    pod-template-generation: "105"
  name: d8-control-plane-manager-b
  namespace: kube-system
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'False'
---
`
		properDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 105
  name: d8-control-plane-manager
  namespace: kube-system
---
`
		justRolledOutDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 106
  name: d8-control-plane-manager
  namespace: kube-system
---
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster having all cpm Pods being Ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningReadyPods + properDaemonSet))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster having all cpm Pods being Ready but no DS", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningReadyPods))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster having no cpm Pods but with DS", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(properDaemonSet))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster having all cpm Pods being Ready with just rolled new DS", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningReadyPods + justRolledOutDaemonSet))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster having not Ready cpm Pods", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningNotReadyPods + properDaemonSet))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})

})
