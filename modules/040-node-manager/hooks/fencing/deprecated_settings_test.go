/*
Copyright 2026 Flant JSC

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

package fencing

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: fencing :: deprecated_settings ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	registerNodeGroupCRD(f)

	expire := operation.MetricOperation{Group: deprecatedSettingsMetric, Action: operation.ActionExpireMetrics}
	series := func(name, setting string) operation.MetricOperation {
		return operation.MetricOperation{
			Name:   deprecatedSettingsMetric,
			Group:  deprecatedSettingsMetric,
			Action: operation.ActionGaugeSet,
			Value:  ptr.To(1.0),
			Labels: map[string]string{"name": name, "setting": setting},
		}
	}

	Context("NodeGroup with mode Watchdog and watchdog.timeout", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupsYAML))
			f.RunGoHook()
		})

		It("Must publish a series for each deprecated setting and nothing for other groups", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{
				expire,
				series("legacy", "mode"),
				series("legacy", "watchdog.timeout"),
			}))
		})
	})

	Context("The user replaced Watchdog with Slow and removed watchdog.timeout", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: legacy
spec:
  nodeType: Static
  fencing:
    mode: Slow
`))
			f.RunGoHook()
		})

		It("Must only expire the group so the series disappears", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire}))
		})
	})
})
