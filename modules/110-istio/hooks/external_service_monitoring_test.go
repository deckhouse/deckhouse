/*
Copyright 2023 Flant JSC

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

var _ = Describe("Istio hooks :: external service monitoring ::", func() {
	f := HookExecutionConfigInit(`
{
  "istio":{"internal":{}}
}
`, "")

	Context("ClusterIP service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterIPService))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1)) // group expire
		})
	})

	Context("ExternalName services service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(extService + extServiceWithPort))
			f.RunHook()
		})

		It("Hook must figure out irrelevant external service with ports and set metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1].Labels).To(BeEquivalentTo(map[string]string{"namespace": "default", "name": "b-ext"}))
		})
	})
})

const (
	extService = `
---
apiVersion: v1
kind: Service
metadata:
  name: a-ext
  namespace: default
spec:
  type: ExternalName
  externalName: a.echo.svc.cluster.local
`

	extServiceWithPort = `
---
apiVersion: v1
kind: Service
metadata:
  name: b-ext
  namespace: default
spec:
  ports:
  - name: tcp
    port: 80
    targetPort: 18080
  type: ExternalName
  externalName: b.echo.svc.cluster.local
`

	clusterIPService = `
---
apiVersion: v1
kind: Service
metadata:
  name: kubernetes
  namespace: default
spec:
  clusterIP: 10.222.0.1
  clusterIPs:
  - 10.222.0.1
  internalTrafficPolicy: Cluster
  ports:
  - name: https
    port: 443
    protocol: TCP
    targetPort: 6443
  type: ClusterIP
`
)
