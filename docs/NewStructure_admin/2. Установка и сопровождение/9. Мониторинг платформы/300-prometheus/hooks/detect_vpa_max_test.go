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

var _ = Describe("Prometheus hooks :: detect max vpa ::", func() {
	f := HookExecutionConfigInit(`
prometheus:
  internal:
    vpa: {}
`, ``)

	Context("1 node cluster", func() {
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
			Expect(f.ValuesGet("prometheus.internal.vpa.maxCPU").String()).Should(BeEquivalentTo("2200m"))
			Expect(f.ValuesGet("prometheus.internal.vpa.maxMemory").String()).Should(BeEquivalentTo("1650Mi"))
			Expect(f.ValuesGet("prometheus.internal.vpa.longtermMaxCPU").String()).Should(BeEquivalentTo("733m"))
			Expect(f.ValuesGet("prometheus.internal.vpa.longtermMaxMemory").String()).Should(BeEquivalentTo("550Mi"))
		})
	})

	Context("Minimal resources for Prometheus and longterm", func() {
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
    pods: "3"
`, 1))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunHook()
		})

		It("should fill minimal internal vpa values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.vpa.maxCPU").String()).Should(BeEquivalentTo("200m"))
			Expect(f.ValuesGet("prometheus.internal.vpa.maxMemory").String()).Should(BeEquivalentTo("1000Mi"))
			Expect(f.ValuesGet("prometheus.internal.vpa.longtermMaxCPU").String()).Should(BeEquivalentTo("50m"))
			Expect(f.ValuesGet("prometheus.internal.vpa.longtermMaxMemory").String()).Should(BeEquivalentTo("500Mi"))
		})
	})
})
