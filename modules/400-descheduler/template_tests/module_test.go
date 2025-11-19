/*
Copyright 2022 Flant JSC

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

package template_tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
  enabledModules: ["vertical-pod-autoscaler"]
  modules:
    placement: {}
  discovery:
    kubernetesVersion: 1.30.1
    d8SpecificNodeCountByRole:
      master: 3
`

var _ = Describe("Module :: monitoring-kubernetes-control-plane :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("", func() {
		BeforeEach(func() {
			moduleValues := `
internal:
  deschedulers:
  - name: test1
    strategies:
      lowNodeUtilization:
        enabled: true
        thresholds:
          cpu: 10
          memory: 20
          pods: 30
        targetThresholds:
          cpu: 40
          memory: 50
          pods: 50
          gpu: "gpuNode"
  - name: test2
    strategies:
      lowNodeUtilization:
        enabled: true
        thresholds:
          cpu: 10
          memory: 20
          pods: 30
        targetThresholds:
          cpu: 40
          memory: 50
          pods: 50
          gpu: "gpuNode"
      highNodeUtilization:
        enabled: true
        thresholds:
          cpu: 14
          memory: 23
          pods: 3
  - name: test3
    nodeLabelSelector: node.deckhouse.io/group=test1,node.deckhouse.io/type in (test1,test2)
    podLabelSelector:
      matchExpressions:
      - key: dbType
        operator: In
        values:
        - test1
        - test2
      matchLabels:
        app: test1
    namespaceLabelSelector:
      matchExpressions:
      - key: dbType
        operator: In
        values:
        - test1
        - test2
      matchLabels:
        app: test1
    priorityClassThreshold:
      value: 1000
    strategies:
      highNodeUtilization:
        enabled: true
        thresholds:
          cpu: 14
          memory: 23
          pods: 3
  - name: test4
    evictLocalStoragePods: true
    strategies:
      lowNodeUtilization:
        enabled: true
        thresholds:
          cpu: 10
          memory: 20
          pods: 30
        targetThresholds:
          cpu: 40
          memory: 50
          pods: 50
          gpu: "gpuNode"
`
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("descheduler", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			cm := f.KubernetesResource("ConfigMap", "d8-descheduler", "descheduler-policy")
			Expect(cm.Field(`data.policy\.yaml`)).To(MatchYAML(`---
apiVersion: descheduler/v1alpha2
kind: DeschedulerPolicy
profiles:
- name: test1
  pluginConfig:
  - args:
      evictFailedBarePods: true
      evictLocalStoragePods: false
      evictSystemCriticalPods: false
      ignorePvcPods: false
      nodeFit: true
    name: DefaultEvictor
  - args:
      targetThresholds:
        cpu: 40
        gpu: gpuNode
        memory: 50
        pods: 50
      thresholds:
        cpu: 10
        memory: 20
        pods: 30
    name: LowNodeUtilization
  plugins:
    balance:
      enabled:
      - LowNodeUtilization
    filter:
      enabled:
      - DefaultEvictor
    preEvictionFilter:
      enabled:
      - DefaultEvictor
- name: test2
  pluginConfig:
  - args:
      evictFailedBarePods: true
      evictLocalStoragePods: false
      evictSystemCriticalPods: false
      ignorePvcPods: false
      nodeFit: true
    name: DefaultEvictor
  - args:
      targetThresholds:
        cpu: 40
        gpu: gpuNode
        memory: 50
        pods: 50
      thresholds:
        cpu: 10
        memory: 20
        pods: 30
    name: LowNodeUtilization
  - args:
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
    name: HighNodeUtilization
  plugins:
    balance:
      enabled:
      - HighNodeUtilization
      - LowNodeUtilization
    filter:
      enabled:
      - DefaultEvictor
    preEvictionFilter:
      enabled:
      - DefaultEvictor
- name: test3
  pluginConfig:
  - args:
      evictFailedBarePods: true
      evictLocalStoragePods: false
      evictSystemCriticalPods: false
      ignorePvcPods: false
      labelSelector:
        matchExpressions:
        - key: dbType
          operator: In
          values:
          - test1
          - test2
        matchLabels:
          app: test1
      namespaceLabelSelector:
        matchExpressions:
        - key: dbType
          operator: In
          values:
          - test1
          - test2
        matchLabels:
          app: test1
      nodeFit: true
      nodeSelector: node.deckhouse.io/group=test1,node.deckhouse.io/type in (test1,test2)
      priorityThreshold:
        value: 1000
    name: DefaultEvictor
  - args:
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
    name: HighNodeUtilization
  plugins:
    balance:
      enabled:
      - HighNodeUtilization
    filter:
      enabled:
      - DefaultEvictor
    preEvictionFilter:
      enabled:
      - DefaultEvictor
- name: test4
  pluginConfig:
  - args:
      evictFailedBarePods: true
      evictLocalStoragePods: true
      evictSystemCriticalPods: false
      ignorePvcPods: false
      nodeFit: true
    name: DefaultEvictor
  - args:
      targetThresholds:
        cpu: 40
        gpu: gpuNode
        memory: 50
        pods: 50
      thresholds:
        cpu: 10
        memory: 20
        pods: 30
    name: LowNodeUtilization
  plugins:
    balance:
      enabled:
      - LowNodeUtilization
    filter:
      enabled:
      - DefaultEvictor
    preEvictionFilter:
      enabled:
      - DefaultEvictor
`))
			Expect(f.KubernetesResource("Deployment", "d8-descheduler", "descheduler").Exists()).To(BeTrue())
		})
	})
})
