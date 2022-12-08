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

var _ = Describe("Modules :: prometheus :: hooks :: lock_main_queue_during_update ::", func() {
	const (
		initValuesString       = `{"global": {"clusterIsBootstrapped": true}}`
		initConfigValuesString = ``
		runningReadyPods       = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    prometheus: main
  name: prometheus-main-0
  namespace: d8-monitoring
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    prometheus: main
  name: prometheus-main-1
  namespace: d8-monitoring
status:
  conditions:
  - type: Ready
    status: 'False'
`
		runningNotReadyPods = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    prometheus: main
  name: prometheus-main-0
  namespace: d8-monitoring
status:
  conditions:
  - type: Ready
    status: 'False'
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    prometheus: main
  name: prometheus-main-1
  namespace: d8-monitoring
status:
  conditions:
  - type: Ready
    status: 'False'
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should exit without error", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster having one prometheus main Pod being Ready", func() {
		BeforeEach(func() {
			f.KubeStateSet(runningReadyPods)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.ValuesSet("global.discovery.clusterControlPlaneIsHighlyAvailable", true)
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster having not Ready Pods", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningNotReadyPods))
			f.ValuesSet("global.discovery.clusterControlPlaneIsHighlyAvailable", true)
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})

	Context("Cluster having not Ready Pods", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningNotReadyPods))
			f.ValuesSet("global.discovery.clusterControlPlaneIsHighlyAvailable", false)
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

	})

})
