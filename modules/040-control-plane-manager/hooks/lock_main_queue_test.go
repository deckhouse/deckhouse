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
  name: d8-control-plane-manager-a
  namespace: kube-system
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'True'
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
  name: d8-control-plane-manager-b
  namespace: kube-system
status:
  conditions:
  - type: Ready
    status: 'False'
  phase: Failed
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
  name: d8-control-plane-manager-c
  namespace: kube-system
status:
  conditions:
  - type: Ready
    status: 'False'
  phase: Succeeded
`
		runningNotReadyPods = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
  name: d8-control-plane-manager-a
  namespace: kube-system
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'True'
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
  name: d8-control-plane-manager-b
  namespace: kube-system
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'False'
  phase: Running
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster having all cpm Pods being Ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningReadyPods))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster having not Ready cpm Pods", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningNotReadyPods))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})

})
