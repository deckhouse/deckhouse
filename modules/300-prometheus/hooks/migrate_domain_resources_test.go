package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: migrate_domain_resources ::", func() {
	const (
		properResources = `
---
apiVersion: monitoring.coreos.com/v1
kind: Alertmanager
metadata:
  name: main
  namespace: default
spec:
  nodeSelector:
    node-role.deckhouse.io/system: ""
  tolerations:
  - key: dedicated.deckhouse.io
    operator: Equal
    value: prometheus
  - key: dedicated.deckhouse.io
    operator: Equal
    value: monitoring
  - key: dedicated.deckhouse.io
    operator: Equal
    value: system
---
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: main-0
  namespace: default
spec:
  nodeSelector:
    node-role.deckhouse.io/system: ""
  tolerations:
  - key: dedicated.deckhouse.io
    operator: Equal
    value: prometheus
  - key: dedicated.deckhouse.io
    operator: Equal
    value: monitoring
  - key: dedicated.deckhouse.io
    operator: Equal
    value: system
---
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: main-1
  namespace: default
  labels:
    heritage: deckhouse
spec:
  nodeSelector:
    node-role.flant.com/system: ""
  tolerations:
  - key: dedicated.flant.com
    operator: Equal
    value: prometheus
  - key: dedicated.flant.com
    operator: Equal
    value: monitoring
  - key: dedicated.flant.com
    operator: Equal
    value: system
`
		resourcesWithOldNodeSelector = `
---
apiVersion: monitoring.coreos.com/v1
kind: Alertmanager
metadata:
  name: main
  namespace: default
spec:
  nodeSelector:
    node-role.flant.com/system: ""
---
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: main
  namespace: default
spec:
  nodeSelector:
    node-role.flant.com/system: ""
`
		resourcesWithOldTolerations = `
---
apiVersion: monitoring.coreos.com/v1
kind: Alertmanager
metadata:
  name: main
  namespace: default
spec:
  tolerations:
  - key: dedicated.flant.com
    operator: Equal
    value: system
---
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: main
  namespace: default
spec:
  tolerations:
  - key: dedicated.flant.com
    operator: Equal
    value: system
`
	)
	f := HookExecutionConfigInit(
		`{"prometheus":{"internal":{"grafana":{}}}}`,
		`{}`,
	)
	f.RegisterCRD("monitoring.coreos.com", "v1", "Alertmanager", true)
	f.RegisterCRD("monitoring.coreos.com", "v1", "Prometheus", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster containing proper resources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(properResources))
			f.RunHook()
		})

		It("Hook must not fail, no metrics should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.alertmanagers.0.filterResult.labels").Exists()).To(BeFalse())
			Expect(f.BindingContexts.Get("0.snapshots.prometheuses.0.filterResult.labels").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with resources having old NodeSelector", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithOldNodeSelector))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.alertmanagers.0.filterResult.labels").String()).To(MatchJSON(`{"name":"main","kind":"Alertmanager","namespace":"default"}`))
			Expect(f.BindingContexts.Get("0.snapshots.prometheuses.0.filterResult.labels").String()).To(MatchJSON(`{"name":"main","kind":"Prometheus","namespace":"default"}`))
		})
	})

	Context("Cluster with resources having old tolerations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithOldTolerations))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.alertmanagers.0.filterResult.labels").String()).To(MatchJSON(`{"name":"main","kind":"Alertmanager","namespace":"default"}`))
			Expect(f.BindingContexts.Get("0.snapshots.prometheuses.0.filterResult.labels").String()).To(MatchJSON(`{"name":"main","kind":"Prometheus","namespace":"default"}`))
		})
	})

})
