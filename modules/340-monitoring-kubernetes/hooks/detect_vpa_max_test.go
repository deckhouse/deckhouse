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

var _ = Describe("Modules :: monitoring-kubernetes :: hooks :: detect max vpa ::", func() {
	f := HookExecutionConfigInit(
		`{"monitoringKubernetes":{"internal":{"vpa":{}}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("1 node clustaer", func() {
		BeforeEach(func() {

			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Node
metadata:
  name: test-master-0
spec:
  podCIDR: 10.111.0.0/24
status:
  capacity:
    pods: "110"
`, 1))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunHook()
		})

		It("should fill internal vpa values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringKubernetes.internal.vpa.kubeStateMetricsMaxMemory").String()).Should(BeEquivalentTo("180Mi"))
			Expect(f.ValuesGet("monitoringKubernetes.internal.vpa.kubeStateMetricsMaxCPU").String()).Should(BeEquivalentTo("115m"))
		})
	})

	Context("2 node clustaer", func() {
		BeforeEach(func() {

			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  name: test-master-0
spec:
  podCIDR: 10.111.0.0/24
status:
  capacity:
    pods: "110"
---
apiVersion: v1
kind: Node
metadata:
  name: test-master-1
spec:
  podCIDR: 10.111.1.0/24
status:
  capacity:
    pods: "110"
`, 1))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunHook()
		})

		It("should fill internal vpa values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringKubernetes.internal.vpa.kubeStateMetricsMaxMemory").String()).Should(BeEquivalentTo("210Mi"))
			Expect(f.ValuesGet("monitoringKubernetes.internal.vpa.kubeStateMetricsMaxCPU").String()).Should(BeEquivalentTo("130m"))
		})
	})
})
