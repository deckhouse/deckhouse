/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: geoproxy_ready ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"internal":{}}}`, "")

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set()
			f.RunHook()
		})

		It("hook must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

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
