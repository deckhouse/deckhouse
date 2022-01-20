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

var _ = Describe("Module hooks :: vertical-pod-autoscaler :: set_maxallowed", func() {
	const (
		DeckhousePodIsReady = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: deckhouse
  name: deckhouse-pod
  namespace: d8-system
status:
  conditions:
  - status: "True"
    type: Ready
`
		DeckhousePodIsNotReady = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: deckhouse
  name: deckhouse-pod
  namespace: d8-system
status:
  conditions:
  - status: "False"
    type: Ready
`
		TwoVpasWithoutRecommendations = `
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
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: deckhouse
  updatePolicy:
    updateMode: Initial
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
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: node-exporter
  updatePolicy:
    updateMode: Auto
`
		TwoVpas = `
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
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: deckhouse
  updatePolicy:
    updateMode: Initial
status:
  recommendation:
    containerRecommendations:
    - containerName: deckhouse
      uncappedTarget:
        cpu: 671m
        memory: 2823238195
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
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: node-exporter
  updatePolicy:
    updateMode: Auto
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
		TwoVpasInTreshold = `
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
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: deckhouse
  updatePolicy:
    updateMode: Initial
status:
  recommendation:
    containerRecommendations:
    - containerName: deckhouse
      uncappedTarget:
        cpu: 671m
        memory: 2823238195
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
        cpu: 240m
        memory: "40063150"
    - containerName: kubelet-eviction-thresholds-exporter
      maxAllowed:
        cpu: 240m
        memory: "40077200"
    - containerName: kube-rbac-proxy
      maxAllowed:
        cpu: 240m
        memory: "40077200"
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: node-exporter
  updatePolicy:
    updateMode: Auto
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
	)
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("autoscaling.k8s.io", "v1", "VerticalPodAutoscaler", true)

	Context("Deckhouse pod is not ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(DeckhousePodIsNotReady))
			f.RunHook()
		})

		It("Hook should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

	})
	Context("Cluster without global.modules.resourcesRequests.internal variables, Deckhouse pod is ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(DeckhousePodIsReady))
			f.RunHook()
		})

		It("Hook should fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})
	Context("Cluster with two VPAs without container recommendationsm Deckhouse pod is ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(DeckhousePodIsReady + TwoVpasWithoutRecommendations))
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuMaster", "1024")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryMaster", "520093696")
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuEveryNode", "1024")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryEveryNode", "520093696")
			f.RunHook()
		})

		It("Hook should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			vpaDeckhouse := f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "deckhouse")
			vpaNodeExporter := f.KubernetesResource("VerticalPodAutoscaler", "d8-monitoring", "node-exporter")
			Expect(vpaDeckhouse.Exists()).To(BeTrue())
			Expect(vpaNodeExporter.Exists()).To(BeTrue())
			Expect(vpaDeckhouse.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))
			Expect(vpaNodeExporter.Field("spec.updatePolicy.updateMode").String()).To(Equal("Auto"))
			Expect(vpaDeckhouse.Field("spec.targetRef").String()).To(MatchYAML(`
apiVersion: apps/v1
kind: Deployment
name: deckhouse
`))
			Expect(vpaNodeExporter.Field("spec.targetRef").String()).To(MatchYAML(`
apiVersion: apps/v1
kind: DaemonSet
name: node-exporter
`))
		})
	})
	Context("Cluster with two VPAs and set of global.modules.resourcesRequests.internal variables, Deckhouse pod is ready", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuMaster", "1850")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryMaster", "3864053781")
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuEveryNode", "300")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryEveryNode", "536870912")
			f.BindingContexts.Set(f.KubeStateSet(DeckhousePodIsReady + TwoVpas))
			f.RunHook()
		})

		It("Hook should run and calculate new limits for vpa resources, vpa resources is exists", func() {
			Expect(f).To(ExecuteSuccessfully())
			vpaDeckhouse := f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "deckhouse")
			vpaNodeExporter := f.KubernetesResource("VerticalPodAutoscaler", "d8-monitoring", "node-exporter")
			Expect(vpaDeckhouse.Exists()).To(BeTrue())
			Expect(vpaNodeExporter.Exists()).To(BeTrue())
			Expect(vpaDeckhouse.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))
			Expect(vpaNodeExporter.Field("spec.updatePolicy.updateMode").String()).To(Equal("Auto"))
			Expect(vpaDeckhouse.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: deckhouse
  maxAllowed:
    cpu: 1850m
    memory: "3864053781"
`))
			Expect(vpaNodeExporter.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: kubelet-eviction-thresholds-exporter
  maxAllowed:
    cpu: 100m
    memory: "160309154"
- containerName: kube-rbac-proxy
  maxAllowed:
    cpu: 100m
    memory: "160309154"
- containerName: node-exporter
  maxAllowed:
    cpu: 100m
    memory: "216252602"
`))
			Expect(vpaDeckhouse.Field("spec.targetRef").String()).To(MatchYAML(`
apiVersion: apps/v1
kind: Deployment
name: deckhouse
`))
			Expect(vpaNodeExporter.Field("spec.targetRef").String()).To(MatchYAML(`
apiVersion: apps/v1
kind: DaemonSet
name: node-exporter
`))
		})
	})
	Context("Cluster with two VPAs, and another set of global.modules.resourcesRequests.internal variables, Deckhouse pod is ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(DeckhousePodIsReady + TwoVpas))
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuMaster", "4096")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryMaster", "8589934592")
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuEveryNode", "750")
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
			Expect(vpaDeckhouse.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))
			Expect(vpaNodeExporter.Field("spec.updatePolicy.updateMode").String()).To(Equal("Auto"))
			Expect(vpaDeckhouse.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: deckhouse
  maxAllowed:
    cpu: 4096m
    memory: "8589934592"
`))
			Expect(vpaNodeExporter.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: kubelet-eviction-thresholds-exporter
  maxAllowed:
    cpu: 250m
    memory: "40077288"
- containerName: kube-rbac-proxy
  maxAllowed:
    cpu: 250m
    memory: "40077288"
- containerName: node-exporter
  maxAllowed:
    cpu: 250m
    memory: "54063150"
`))
			Expect(vpaDeckhouse.Field("spec.targetRef").String()).To(MatchYAML(`
apiVersion: apps/v1
kind: Deployment
name: deckhouse
`))
			Expect(vpaNodeExporter.Field("spec.targetRef").String()).To(MatchYAML(`
apiVersion: apps/v1
kind: DaemonSet
name: node-exporter
`))
		})
	})
	Context("Cluster with two VPAs, maxAllowed values near newly calculated values, Deckhouse pod is ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(DeckhousePodIsReady + TwoVpasInTreshold))
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuMaster", "4096")
			f.ValuesSet("global.modules.resourcesRequests.internal.memoryMaster", "8589934592")
			f.ValuesSet("global.modules.resourcesRequests.internal.milliCpuEveryNode", "750")
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
			Expect(vpaDeckhouse.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))
			Expect(vpaNodeExporter.Field("spec.updatePolicy.updateMode").String()).To(Equal("Auto"))
			Expect(vpaDeckhouse.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: deckhouse
  maxAllowed:
    cpu: 4096m
    memory: "8589934592"
`))
			Expect(vpaNodeExporter.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: kubelet-eviction-thresholds-exporter
  maxAllowed:
    cpu: 240m
    memory: "40077200"
- containerName: kube-rbac-proxy
  maxAllowed:
    cpu: 240m
    memory: "40077200"
- containerName: node-exporter
  maxAllowed:
    cpu: 240m
    memory: "54063150"
`))
			Expect(vpaDeckhouse.Field("spec.targetRef").String()).To(MatchYAML(`
apiVersion: apps/v1
kind: Deployment
name: deckhouse
`))
			Expect(vpaNodeExporter.Field("spec.targetRef").String()).To(MatchYAML(`
apiVersion: apps/v1
kind: DaemonSet
name: node-exporter
`))
		})
	})
})
