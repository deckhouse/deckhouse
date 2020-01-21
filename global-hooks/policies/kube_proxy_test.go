/*

User-stories:
1. There is DaemonSet in ns kube-system — kube-proxy. It must work on every node in cluster despite node taints. Hook must add tolerations to kube-proxy.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: policy :: kube_proxy ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateDS = `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  template:
    spec:
      tolerations:
      - operator: GenericToleration
`
		stateDSChanged = `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  template:
    spec:
      tolerations:
      - operator: YetAnotherToleration
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("DS has wrong tolerations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDS))
			f.RunHook()
		})

		It(`.spec.template.spec.tolerations must become '[{"operator":"Exists"}]'`, func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("DaemonSet", "kube-system", "kube-proxy").Field("spec.template.spec.tolerations").String()).To(Equal(`[{"operator":"Exists"}]`))
		})

		Context("DS changed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateDSChanged))
				f.RunHook()
			})

			It(`.spec.template.spec.tolerations must become '[{"operator":"Exists"}]'`, func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("DaemonSet", "kube-system", "kube-proxy").Field("spec.template.spec.tolerations").String()).To(Equal(`[{"operator":"Exists"}]`))
			})
		})
	})

	Context("Cluster has not kube-proxy DS", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It(`Hook must not fail`, func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})
