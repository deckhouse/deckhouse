package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: enlarge_prometheus_disk ::", func() {
	const (
		initValuesString       = `{"prometheus":{"internal": {}}}`
		initConfigValuesString = `{"prometheus":{}}`
	)

	const (
		pvc = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: prometheus
    prometheus: main
  name: prometheus-main-db-prometheus-main-0
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 30Gi
  storageClassName: ceph-ssd
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: prometheus
    prometheus: main
  name: prometheus-main-db-prometheus-main-1
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 30Gi
  storageClassName: ceph-ssd
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: prometheus
    prometheus: longterm
  name: prometheus-longterm-db-prometheus-longterm-0
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 30Gi
  storageClassName: ceph-ssd
`
		storageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
allowVolumeExpansion: false
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

	Context("Cluster with storage class and pvc", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvc + storageClass))
			f.RunHook()
		})

		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

})
