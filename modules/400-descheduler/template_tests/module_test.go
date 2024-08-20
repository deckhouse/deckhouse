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
  enabledModules: ["vertical-pod-autoscaler-crd"]
  modules:
    placement: {}
  discovery:
    kubernetesVersion: 1.29.1
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
        thresholds:
          cpu: 14
          memory: 23
          pods: 3
  - defaultEvictor:
      labelSelector:
        matchExpressions:
        - key: dbType
          operator: In
          values:
          - test1
          - test2
        matchLabels:
          app: test1
      nodeSelector: node.deckhouse.io/group=test1,node.deckhouse.io/type in (test1,test2)
      priorityThreshold:
        value: 1000
    name: test5
    strategies:
      highNodeUtilization:
        thresholds:
          cpu: 14
          memory: 23
          pods: 3
`
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("descheduler", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			cm := f.KubernetesResource("ConfigMap", "d8-descheduler", "descheduler-policy-a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")
			Expect(cm.Field(`data.policy\.yaml`)).To(MatchYAML(`---
apiVersion: descheduler/v1alpha1
evictFailedBarePods: true
kind: DeschedulerPolicy
strategies:
    HighNodeUtilization:
        enabled: true
        params:
            nodeResourceUtilizationThresholds:
                thresholds:
                    cpu: 50
                    memory: 50
    LowNodeUtilization:
        enabled: true
        params:
            nodeResourceUtilizationThresholds:
                targetThresholds:
                    cpu: 80
                    memory: 90
                    pods: 80
                thresholds:
                    cpu: 40
                    memory: 50
                    pods: 40
    PodLifeTime:
        enabled: true
        params:
            podLifeTime:
                maxPodLifeTimeSeconds: 86400
                podStatusPhases:
                    - Pending
    RemoveDuplicates:
        enabled: true
        params: null
    RemovePodsHavingTooManyRestarts:
        enabled: true
        params:
            podsHavingTooManyRestarts:
                includingInitContainers: true
                podRestartThreshold: 100
    RemovePodsViolatingInterPodAntiAffinity:
        enabled: true
        params: null
    RemovePodsViolatingNodeAffinity:
        enabled: false
    RemovePodsViolatingNodeTaints:
        enabled: true
        params: null
    RemovePodsViolatingTopologySpreadConstraint:
        enabled: true
        params: null
`))
			Expect(f.KubernetesResource("Deployment", "d8-descheduler", "descheduler-a94a8fe5ccb19ba61c4c0873d391e987982fbbd3").Exists()).To(BeTrue())
		})
	})
})
