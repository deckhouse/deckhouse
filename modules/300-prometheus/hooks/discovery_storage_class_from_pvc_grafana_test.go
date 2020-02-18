package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: discovery_storage_class_from_pvc_grafana ::", func() {
	const (
		initValuesString       = `{"prometheus":{"internal": {"grafana": {}}}}`
		initConfigValuesString = `{"prometheus":{}}`
	)

	const (
		pvc = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: prometheus-main-db-prometheus-main-0
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: main
spec:
  storageClassName: gp2
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: prometheus-longterm-db-prometheus-longterm-0
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: longterm
spec:
  storageClassName: gp2
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: grafana-storage-grafana-0
  namespace: d8-monitoring
  labels:
    app: grafana
spec:
  storageClassName: gp2
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("cluster with storage class and pvc", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvc))
			f.RunHook()
		})

		It("currentStorageClass must be gp2", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.currentStorageClass").String()).To(Equal("gp2"))
		})
	})

})
