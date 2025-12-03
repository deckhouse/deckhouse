package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: geoproxy_ready ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"internal":{}}}`, "")

	Context("geoproxy not ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(statefulSetNotReady))
			f.RunHook()
		})

		It("sets geoproxyReady to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.geoproxyReady").Bool()).To(BeFalse())
		})
	})

	Context("geoproxy becomes ready once and stays sticky", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(statefulSetReady))
			f.RunHook()
		})

		It("keeps geoproxyReady true even if the next event is not ready", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.geoproxyReady").Bool()).To(BeTrue())

			f.BindingContexts.Set(f.KubeStateSet(statefulSetNotReady))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.geoproxyReady").Bool()).To(BeTrue())
		})
	})
})

const (
	statefulSetReady = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: geoproxy
  namespace: d8-ingress-nginx
  labels:
    app: geoproxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: geoproxy
  serviceName: geoproxy-headless
  template:
    metadata:
      labels:
        app: geoproxy
status:
  observedGeneration: 1
  readyReplicas: 1
  updatedReplicas: 1
`

	statefulSetNotReady = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: geoproxy
  namespace: d8-ingress-nginx
  labels:
    app: geoproxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: geoproxy
  serviceName: geoproxy-headless
  template:
    metadata:
      labels:
        app: geoproxy
status:
  observedGeneration: 1
  readyReplicas: 0
  updatedReplicas: 0
`
)
