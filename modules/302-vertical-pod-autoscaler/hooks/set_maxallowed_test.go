package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"global": {}}`
	initConfigValuesString = `{}`
)

var _ = Describe("Module hooks :: vertical-pod-autoscaler :: set_maxallowed", func() {
	const TwoVpasWithoutRecommendations = `
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  labels:
    heritage: deckhouse
    workload-resource-policy.deckhouse.io: master
  name: deckhouse
  namespace: d8-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: deckhouse
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  labels:
    heritage: deckhouse
    workload-resource-policy.deckhouse.io: every-node
  name: node-exporter
  namespace: d8-monitoring
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: node-exporter
    - containerName: kubelet-eviction-thresholds-exporter
    - containerName: kube-rbac-proxy
`
	const TwoVpas = `
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  labels:
    heritage: deckhouse
    workload-resource-policy.deckhouse.io: master
  name: deckhouse
  namespace: d8-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: deckhouse
      maxAllowed:
        cpu: 227m
        memory: "310373428"
status:
  recommendation:
    containerRecommendations:
    - containerName: deckhouse
      uncappedTarget:
        cpu: 109m
        memory: "297164212"
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  labels:
    heritage: deckhouse
    workload-resource-policy.deckhouse.io: every-node
  name: node-exporter
  namespace: d8-monitoring
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: node-exporter
      maxAllowed:
        cpu: 21m
        memory: "30617028"
    - containerName: kubelet-eviction-thresholds-exporter
      maxAllowed:
        cpu: 21m
        memory: "22696559"
    - containerName: kube-rbac-proxy
      maxAllowed:
        cpu: 21m
        memory: "22696559"
status:
  recommendation:
    containerRecommendations:
    - containerName: kubelet-eviction-thresholds-exporter
      uncappedTarget:
        cpu: 11m
        memory: "17476266"
    - containerName: kube-rbac-proxy
      uncappedTarget:
        cpu: 11m
        memory: "17476266"
    - containerName: node-exporter
      uncappedTarget:
        cpu: 11m
        memory: "23574998"
`
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("autoscaling.k8s.io", "v1", "VerticalPodAutoscaler", true)

	Context("Cluster without global.modules.resourcesRequests.internal variables", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook should fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})
	Context("Cluster with two VPAs without container recommendations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(TwoVpasWithoutRecommendations))
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuMaster", "1024")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryMaster", "520093696")
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuEveryNode", "1024")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryEveryNode", "520093696")
			f.RunHook()
		})

		It("Hook should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

	})

	Context("Cluster with two VPAs and set of global.modules.resourcesRequests.internal variables", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuMaster", "1024")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryMaster", "520093696")
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuEveryNode", "1024")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryEveryNode", "520093696")
			f.BindingContexts.Set(f.KubeStateSet(TwoVpas))
			f.RunHook()
		})

		It("Hook should run and calculate new limits for vpa resources, vpa resources is exists", func() {
			Expect(f).To(ExecuteSuccessfully())
			vpaDeckhouse := f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "deckhouse")
			vpaNodeExporter := f.KubernetesResource("VerticalPodAutoscaler", "d8-monitoring", "node-exporter")
			Expect(vpaDeckhouse.Exists()).To(BeTrue())
			Expect(vpaNodeExporter.Exists()).To(BeTrue())
			Expect(vpaDeckhouse.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: deckhouse
  maxAllowed:
    cpu: 1024m
    memory: "520093696"
`))
			Expect(vpaNodeExporter.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: kubelet-eviction-thresholds-exporter
  maxAllowed:
    cpu: 341m
    memory: "155299493"
- containerName: kube-rbac-proxy
  maxAllowed:
    cpu: 341m
    memory: "155299493"
- containerName: node-exporter
  maxAllowed:
    cpu: 341m
    memory: "209494708"
`))

		})

	})

	Context("Cluster with two VPAs, and another set of global.modules.resourcesRequests.internal variables", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(TwoVpas))
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuMaster", "4096")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryMaster", "8589934592")
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuEveryNode", "300")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryEveryNode", "134217728")
			f.RunHook()
		})

		It(`Hook should run and calculate new limits for vpa resources, vpa resources is exists,
and global variables controlPlaneRequestsCpu, controlPlaneRequestsMemory, everyNodesRequestsCpu, everyNodesRequestsMemory`, func() {
			Expect(f).To(ExecuteSuccessfully())
			vpaDeckhouse := f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "deckhouse")
			vpaNodeExporter := f.KubernetesResource("VerticalPodAutoscaler", "d8-monitoring", "node-exporter")
			Expect(vpaDeckhouse.Exists()).To(BeTrue())
			Expect(vpaNodeExporter.Exists()).To(BeTrue())
			Expect(vpaDeckhouse.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: deckhouse
  maxAllowed:
    cpu: 4096m
    memory: "8589934592"
`))
			Expect(vpaNodeExporter.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: kubelet-eviction-thresholds-exporter
  maxAllowed:
    cpu: 100m
    memory: "40077288"
- containerName: kube-rbac-proxy
  maxAllowed:
    cpu: 100m
    memory: "40077288"
- containerName: node-exporter
  maxAllowed:
    cpu: 100m
    memory: "54063150"
`))

		})

	})

})
