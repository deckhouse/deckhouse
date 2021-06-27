/*
Copyright 2021 Flant CJSC

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

var _ = Describe("Kube DNS :: migrate_service_ports_final ::", func() {
	const (
		initValues = `
kubeDns:
  enableLogs: false
  internal:
    replicas: 2
    enablePodAntiAffinity: false
`
		initConfigValues = `{}`

		originalState = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
spec:
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 53
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 53
`
	)

	f := HookExecutionConfigInit(initValues, initConfigValues)

	Context("Fresh cluster without a Service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with a Service for migration with migration flag set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(originalState))
			f.ValuesSet("kubeDns.internal.migration", true)
			f.RunHook()
		})

		It("Hook must not fail, ports should not be patched", func() {
			Expect(f).To(ExecuteSuccessfully())
			service := f.KubernetesResource("Service", "kube-system", "d8-kube-dns")
			Expect(service.Exists()).To(BeTrue())
			Expect(service.Field("spec.ports").String()).To(MatchYAML(`
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 53
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 53
`))
		})
	})

	Context("Cluster with a Service for migration migration flag unset", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(originalState))
			f.ValuesSet("kubeDns.internal.migration", false)
			f.RunHook()
		})

		It("Hook must not fail, ports should be patched", func() {
			Expect(f).To(ExecuteSuccessfully())
			service := f.KubernetesResource("Service", "kube-system", "d8-kube-dns")
			Expect(service.Exists()).To(BeTrue())
			Expect(service.Field("spec.ports").String()).To(MatchYAML(`
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 5353
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 5353
`))
		})
	})

})
