/*
Copyright 2021 Flant JSC

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

/*

User-stories:
1. There are services with label `prometheus.deckhous.io/alertmanager: <prometheus_instance>. Hook must discover them and store to values `prometheus.internal.alertmanagers` in format {"<prometheus_instance>": [{<service_description>}, ...], ...}.
   There is optional annotation `prometheus.deckhouse.io/alertmanager-path-prefix` with default value "/". It must be stored in service description.

*/

package hooks

import (
	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: alertmanager discovery ::", func() {
	const (
		initValuesString       = `{"prometheus": {"internal": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateNonSpecialServices = `
---
apiVersion: v1
kind: Service
metadata:
  name: some-svc-1
  namespace: some-ns-1
---
apiVersion: v1
kind: Service
metadata:
  name: some-svc-2
  namespace: some-ns-2
`

		stateSpecialServicesAlpha = `
---
apiVersion: v1
kind: Service
metadata:
  name: mysvc1
  namespace: myns1
  labels:
    prometheus.deckhouse.io/alertmanager: alphaprom
  annotations:
    prometheus.deckhouse.io/alertmanager-path-prefix: /myprefix/
spec:
  ports:
  - name: test
    port: 81
`
		stateSpecialServicesBeta = `
---
apiVersion: v1
kind: Service
metadata:
  name: mysvc2
  namespace: myns2
  labels:
    prometheus.deckhouse.io/alertmanager: betaprom
spec:
  ports:
  - port: 82
---
apiVersion: v1
kind: Service
metadata:
  name: mysvc3
  namespace: myns3
  labels:
    prometheus.deckhouse.io/alertmanager: betaprom
spec:
  ports:
  - port: 83
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster has non-special services", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNonSpecialServices, 0))
			f.RunHook()
		})

		It("snapshots must be empty; prometheus.internal.alertmanagers must be '{}'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Get("0.snapshots.alertmanager_services").Array()).To(BeEmpty())
			Expect(f.ValuesGet("prometheus.internal.alertmanagers").Exists()).ToNot(BeTrue())
		})
	})

	Context("Cluster has special service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNonSpecialServices+stateSpecialServicesAlpha, 1))
			f.RunHook()
		})

		It(`prometheus.internal.alertmanagers must be '{"alphaprom":[{"name":"mysvc1","namespace":"myns1","pathPrefix":"/myprefix/","port":"test"}]}'`, func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.alertmanagers").String()).To(MatchJSON(`{"alphaprom":[{"name":"mysvc1","namespace":"myns1","pathPrefix":"/myprefix/","port":"test"}]}`))
		})

		Context("Two more special services added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNonSpecialServices+stateSpecialServicesAlpha+stateSpecialServicesBeta, 2))
				f.RunHook()
			})

			It(`prometheus.internal.alertmanagers must be '{"alphaprom":[{"name":"mysvc1","namespace":"myns1","pathPrefix":"/myprefix/","port":"test"}],"betaprom":[{"name":"mysvc2","namespace":"myns2","pathPrefix":"/","port":82},{"name":"mysvc3","namespace":"myns3","pathPrefix":"/","port":"test"}]}'`, func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("prometheus.internal.alertmanagers").String()).To(MatchJSON(`{"alphaprom":[{"name":"mysvc1","namespace":"myns1","pathPrefix":"/myprefix/","port":"test"}],"betaprom":[{"name":"mysvc2","namespace":"myns2","pathPrefix":"/","port":82},{"name":"mysvc3","namespace":"myns3","pathPrefix":"/","port":83}]}`))
			})
		})

	})
})
