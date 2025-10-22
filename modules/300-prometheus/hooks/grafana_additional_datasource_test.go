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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: grafana additional datasource ::", func() {
	f := HookExecutionConfigInit(`{"prometheus":{"internal":{"grafana":{}}}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1", "GrafanaAdditionalDatasource", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		Context("After adding GrafanaAdditionalDatasource", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: test
spec:
  url: /abc
  type: test
  access: Proxy
`, 1))
				f.RunHook()
			})

			It("Should store GrafanaAdditionalDatasource in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("prometheus.internal.grafana.additionalDatasources").String()).To(MatchJSON(`
[{
   "access": "proxy",
   "editable": false,
   "isDefault": false,
   "name": "test",
   "orgId": 1,
   "type": "test",
   "url": "/abc",
   "uid": "test",
   "version": 1
}]`))
			})

			Context("And after deleting GrafanaAdditionalDatasource", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
					f.RunHook()
				})

				It("Should delete GrafanaAdditionalDatasource from values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("prometheus.internal.grafana.additionalDatasources").String()).To(MatchJSON(`[]`))
				})
			})

			Context("And after updating GrafanaAdditionalDatasource", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: test
spec:
  url: /def
  type: test
  access: Direct
`, 1))
					f.RunHook()
				})

				It("Should update GrafanaAdditionalDatasource in values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("prometheus.internal.grafana.additionalDatasources").String()).To(MatchJSON(`
[{
   "access": "direct",
   "editable": false,
   "isDefault": false,
   "name": "test",
   "orgId": 1,
   "type": "test",
   "url": "/def",
   "uid": "test",
   "version": 1
}]`))
				})
			})
		})
	})

	Context("Cluster with GrafanaAdditionalDatasource", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: test
spec:
  url: /abc
  type: test
  access: Proxy
---
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: test-next
spec:
  url: /def
  type: test-next
  access: Direct
`, 2))
			f.RunHook()
		})

		It("Should synchronize the GrafanaAdditionalDatasource to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.additionalDatasources").String()).To(MatchJSON(`
[{
   "access": "proxy",
   "editable": false,
   "isDefault": false,
   "name": "test",
   "orgId": 1,
   "type": "test",
   "url": "/abc",
   "uid": "test",
   "version": 1
},{
   "access": "direct",
   "editable": false,
   "isDefault": false,
   "name": "test-next",
   "orgId": 1,
   "type": "test-next",
   "url": "/def",
   "uid": "test-next",
   "version": 1
}]`))
		})
	})
})
