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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	deschedulerCR1 = `---
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: test
spec:
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
	deschedulerCR2 = `
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: test2
spec:
  strategies:
    lowNodeUtilization:
      thresholds:
        cpu: 10
        memory: 20
        pods: 30
    highNodeUtilization:
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
`
	deschedulerCR3 = `
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: test3
spec:
  nodeLabelSelector:
    matchExpressions:
    - key: node.deckhouse.io/group
      operator: In
      values: ["test1", "test2"]
  strategies:
    highNodeUtilization:
      enabled: true
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
`

	deschedulerCR4 = `
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: test4
spec:
  nodeSelector: node.deckhouse.io/group=test3
  strategies:
    highNodeUtilization:
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
`

	deschedulerCR5 = `
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: test5
spec:
  nodeLabelSelector:
    matchExpressions:
    - key: node.deckhouse.io/group
      operator: In
      values: ["test1", "test2"]
  podLabelSelector:
    matchLabels:
      app: test1
    matchExpressions:
    - key: dbType
      operator: In
      values: ["test1", "test2"]
  namespaceLabelSelector:
    matchLabels:
      kubernetes.io/metadata.name: test
  priorityClassThreshold:
    value: 1000
  strategies:
    highNodeUtilization:
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
`
)

var _ = Describe("Modules :: descheduler :: hooks :: get_crds ::", func() {
	f := HookExecutionConfigInit(`{"descheduler":{"internal":{}}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha2", "Descheduler", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run without errors", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.deschedulers").String()).To(MatchYAML(`[]`))
		})
	})

	Context("Cluster with one Descheduler CR", func() {
		BeforeEach(func() {
			f.KubeStateSet(deschedulerCR1)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run without errors", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.deschedulers").String()).To(MatchYAML(`
- name: test
  evictLocalStoragePods: false
  strategies:
    lowNodeUtilization:
      enabled: true
      targetThresholds:
        cpu: 40
        gpu: gpuNode
        memory: 50
        pods: 50
      thresholds:
        cpu: 10
        memory: 20
        pods: 30
`))
		})
	})

	Context("Cluster with two Deschedulers CR", func() {
		BeforeEach(func() {
			f.KubeStateSet(deschedulerCR1 + deschedulerCR2)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run without errors", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.deschedulers").String()).To(MatchYAML(`
- name: test
  evictLocalStoragePods: false
  strategies:
    lowNodeUtilization:
      enabled: true
      targetThresholds:
        cpu: 40
        gpu: gpuNode
        memory: 50
        pods: 50
      thresholds:
        cpu: 10
        memory: 20
        pods: 30
- name: test2
  evictLocalStoragePods: false
  strategies:
    highNodeUtilization:
      enabled: false
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
    lowNodeUtilization:
      enabled: false
      targetThresholds:
        cpu: 70
        memory: 70
        pods: 70
      thresholds:
        cpu: 10
        memory: 20
        pods: 30
`))
		})
	})

	Context("Cluster with Deschedulers CR with DefaultEvictor setup (nodeSelector uses MatchExpressions)", func() {
		BeforeEach(func() {
			f.KubeStateSet(deschedulerCR3)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run without errors", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.deschedulers").String()).To(MatchYAML(`
- nodeLabelSelector: node.deckhouse.io/group in (test1,test2)
  name: test3
  evictLocalStoragePods: false
  strategies:
    highNodeUtilization:
      enabled: true
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
`))
		})
	})

	Context("Cluster with Deschedulers CR with DefaultEvictor setup (nodeSelector uses MatchLabels)", func() {
		BeforeEach(func() {
			f.KubeStateSet(deschedulerCR4)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run without errors", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.deschedulers").String()).To(MatchYAML(`
- nodeLabelSelector: node.deckhouse.io/group=test3
  name: test4
  evictLocalStoragePods: false
  strategies:
    highNodeUtilization:
      enabled: false
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
`))
		})
	})

	Context("Cluster with Deschedulers CR with DefaultEvictor setup (nodeSelector uses MatchLabels, having LabelSelector and PriorityThreshold)", func() {
		BeforeEach(func() {
			f.KubeStateSet(deschedulerCR5)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run without errors", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.deschedulers").String()).To(MatchYAML(`
- nodeLabelSelector: node.deckhouse.io/group in (test1,test2)
  name: test5
  evictLocalStoragePods: false
  podLabelSelector:
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
  namespaceLabelSelector:
    matchLabels:
      kubernetes.io/metadata.name: test
  strategies:
    highNodeUtilization:
      enabled: false
      thresholds:
        cpu: 14
        memory: 23
        pods: 3
`))
		})
	})
})
