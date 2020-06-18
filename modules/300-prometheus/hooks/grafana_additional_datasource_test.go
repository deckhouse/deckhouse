package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: grafana additional datasource ::", func() {
	f := HookExecutionConfigInit(`{"prometheus":{"internal":{"grafana":{}}}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "GrafanaAdditionalDatasource", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		Context("After adding GrafanaAdditionalDatasource", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: GrafanaAdditionalDatasource
metadata:
  name: test
spec:
  url: /abc
  type: test
  access: proxy
`))
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
   "uuid": "test",
   "version": 1
}]`))
			})

			Context("And after deleting GrafanaAdditionalDatasource", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(``))
					f.RunHook()
				})

				It("Should delete GrafanaAdditionalDatasource from values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("prometheus.internal.grafana.additionalDatasources").String()).To(MatchJSON(`[]`))
				})
			})

			Context("And after updating GrafanaAdditionalDatasource", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: GrafanaAdditionalDatasource
metadata:
  name: test
spec:
  url: /def
  type: test
  access: direct
`))
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
   "uuid": "test",
   "version": 1
}]`))
				})
			})
		})
	})

	Context("Cluster with GrafanaAdditionalDatasource", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: GrafanaAdditionalDatasource
metadata:
  name: test
spec:
  url: /abc
  type: test
  access: proxy
---
apiVersion: deckhouse.io/v1alpha1
kind: GrafanaAdditionalDatasource
metadata:
  name: test-next
spec:
  url: /def
  type: test-next
  access: direct
`))
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
   "uuid": "test",
   "version": 1
},{
   "access": "direct",
   "editable": false,
   "isDefault": false,
   "name": "test-next",
   "orgId": 1,
   "type": "test-next",
   "url": "/def",
   "uuid": "test-next",
   "version": 1
}]`))
		})
	})
})
