package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Modules :: prometheus :: hooks :: prometheus_disk ::", func() {
	const (
		pvcMain = `
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
      storage: 15Gi
  storageClassName: ceph-ssd
status:
  capacity:
    storage: 15Gi
  conditions:
  - status: "True"
    type: FileSystemResizePending
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
      storage: 45Gi
  storageClassName: ceph-ssd
status:
  capacity:
    storage: 45Gi
  conditions:
  - status: "True"
    type: FileSystemResizePending
`
		pvcLt = `
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
      storage: 10Gi
  storageClassName: ceph-ssd
status:
  capacity:
    storage: 10Gi
  conditions:
  - status: "True"
    type: FileSystemResizePending
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: prometheus
    prometheus: longterm
  name: prometheus-longterm-db-prometheus-longterm-1
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 40Gi
  storageClassName: ceph-ssd
status:
  capacity:
    storage: 40Gi
  conditions:
  - status: "True"
    type: FileSystemResizePending
`
		storageClassExpensionTrue = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
allowVolumeExpansion: true
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: rbd
allowVolumeExpansion: false
`
		storageClassExpensionFalse = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
allowVolumeExpansion: false
`
		pods = `
---
apiVersion: v1
kind: Pod
metadata:
  name: prometheus-main-0
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: main
status:
  conditions:
  - status: "True"
    type: PodScheduled
---
apiVersion: v1
kind: Pod
metadata:
  name: prometheus-main-1
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: main
status:
  conditions:
  - status: "True"
    type: PodScheduled
---
apiVersion: v1
kind: Pod
metadata:
  name: prometheus-longterm-0
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: longterm
status:
  conditions:
  - status: "True"
    type: PodScheduled
---
apiVersion: v1
kind: Pod
metadata:
  name: prometheus-longterm-1
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: longterm
status:
  conditions:
  - status: "True"
    type: PodScheduled
`
	)

	f := HookExecutionConfigInit(`{"prometheus": {"internal":{"prometheusMain":{}, "prometheusLongterm":{} }}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Empty cluster and Schedule", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.RunSchedule("*/15 * * * *"))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with storageClassExpensionFalse", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(storageClassExpensionFalse))
			f.BindingContexts.Set(f.RunSchedule("*/15 * * * *"))
			f.ValuesSet("prometheus.internal.prometheusMain.effectiveStorageClass", "ceph-ssd")
			f.ValuesSet("prometheus.internal.prometheusLongterm.effectiveStorageClass", "ceph-ssd")
			f.RunHook()
		})

		It("must be executed successfully; main and longterm disk size must be 30, retention must be 25", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.diskSizeGigabytes").String()).To(Equal("30"))
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.retentionGigabytes").String()).To(Equal("25"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.diskSizeGigabytes").String()).To(Equal("30"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.retentionGigabytes").String()).To(Equal("25"))
		})
	})

	Context("Cluster with storageClassExpensionTrue", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(storageClassExpensionTrue))
			f.BindingContexts.Set(f.RunSchedule("*/15 * * * *"))
			f.ValuesSet("prometheus.internal.prometheusMain.effectiveStorageClass", "ceph-ssd")
			f.ValuesSet("prometheus.internal.prometheusLongterm.effectiveStorageClass", "ceph-ssd")
			f.RunHook()
		})

		It("must be executed successfully; main and longterm disk size must be 15, retention must be 10", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.diskSizeGigabytes").String()).To(Equal("15"))
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.retentionGigabytes").String()).To(Equal("10"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.diskSizeGigabytes").String()).To(Equal("15"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.retentionGigabytes").String()).To(Equal("10"))
		})
	})

	Context("Cluster with storageClassExpensionTrue and pvc", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcMain + pvcLt + storageClassExpensionTrue))
			f.BindingContexts.Set(f.RunSchedule("*/15 * * * *"))
			f.ValuesSet("prometheus.internal.prometheusMain.effectiveStorageClass", "ceph-ssd")
			f.ValuesSet("prometheus.internal.prometheusLongterm.effectiveStorageClass", "ceph-ssd")
			f.RunHook()
		})

		It("must be executed successfully; main disk size must be 45, retention must be 36; longterm disk size must be 40, retention must be 32", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.diskSizeGigabytes").String()).To(Equal("45"))
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.retentionGigabytes").String()).To(Equal("36"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.diskSizeGigabytes").String()).To(Equal("40"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.retentionGigabytes").String()).To(Equal("32"))
		})
	})

	Context("Cluster with storageClassExpensionTrue and pvc", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(storageClassExpensionTrue + pvcMain + pvcLt))
			f.BindingContexts.Set(f.RunSchedule("*/15 * * * *"))
			f.ValuesSet("prometheus.internal.prometheusMain.effectiveStorageClass", "ceph-ssd")
			f.ValuesSet("prometheus.internal.prometheusLongterm.effectiveStorageClass", "ceph-ssd")
			f.ValuesSet("prometheus.internal.prometheusMain.diskUsage", "91")
			f.ValuesSet("prometheus.internal.prometheusLongterm.diskUsage", "91")
			f.RunHook()
		})

		It("must be executed successfully; prometheus-main-0,1 pvc must be 50Gi; prometheus-longterm-0,1 pvc must be 45Gi", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-main-db-prometheus-main-0").Field("spec.resources.requests.storage").String()).To(Equal("50Gi"))
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-main-db-prometheus-main-1").Field("spec.resources.requests.storage").String()).To(Equal("50Gi"))
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-longterm-db-prometheus-longterm-0").Field("spec.resources.requests.storage").String()).To(Equal("45Gi"))
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-longterm-db-prometheus-longterm-1").Field("spec.resources.requests.storage").String()).To(Equal("45Gi"))
		})
	})

	Context("Cluster with pvc's in state FileSystemResizePending", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pods + pvcMain + pvcLt))
			f.RunHook()
		})

		It("must be executed successfully; pods must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-monitoring", "prometheus-main-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Pod", "d8-monitoring", "prometheus-main-1").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Pod", "d8-monitoring", "prometheus-longterm-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Pod", "d8-monitoring", "prometheus-longterm-1").Exists()).To(BeFalse())
		})
	})

})
