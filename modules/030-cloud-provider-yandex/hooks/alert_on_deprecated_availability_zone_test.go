/*
Copyright 2023 Flant JSC

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
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: alert_on_deprecated_availability_zone ::", func() {
	const initValuesString = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
`
	f := HookExecutionConfigInit(initValuesString, `{}`)

	initialKubeState := `
apiVersion: v1
kind: Node
metadata:
  name: yandex-central1-a
  labels:
    node.deckhouse.io/group: master
    topology.kubernetes.io/region: ru-central1
    topology.kubernetes.io/zone: ru-central1-a
spec:
  providerID: yandex://9cgf2kcqvi50mn5hqmhf`

	Context("With no Nodes on deprecated zones", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(initialKubeState))
			f.RunHook()
		})

		It("Should succeed with no collected metrics", func() {
			Expect(f).To(ExecuteSuccessfully())
			hasDeprecatedZone, exists := requirements.GetValue(yandexDeprecatedZoneInNodesKey)
			Expect(exists).To(BeTrue())
			Expect(hasDeprecatedZone).To(BeFalse())
			Expect(f.MetricsCollector.CollectedMetrics()).To(BeEmpty())
		})
	})

	nodeOnDeprecatedAvailabilityZone := `
---
apiVersion: v1
kind: Node
metadata:
  name: yandex-central1-c
  labels:
    node.deckhouse.io/group: system-c
    topology.kubernetes.io/region: ru-central1
    topology.kubernetes.io/zone: ru-central1-c
spec:
  providerID: yandex://fhmqh5nm05ivqck2fgc9`

	expectedMetric := operation.MetricOperation{
		Name:   "d8_node_group_node_with_deprecated_availability_zone",
		Value:  pointer.Float64(1.0),
		Labels: map[string]string{"node_group": "system-c"},
		Action: "set",
	}

	Context("With Nodes on deprecated ru-central1-c zone", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(initialKubeState + nodeOnDeprecatedAvailabilityZone))
			f.RunHook()
		})

		It("Should collect metrics with node groups", func() {
			collectedMetrics := f.MetricsCollector.CollectedMetrics()
			Expect(f).To(ExecuteSuccessfully())
			Expect(collectedMetrics).To(ContainElement(expectedMetric))
			hasDeprecatedZone, exists := requirements.GetValue(yandexDeprecatedZoneInNodesKey)
			Expect(exists).To(BeTrue())
			Expect(hasDeprecatedZone).To(BeTrue())
			Expect(len(collectedMetrics)).Should(Equal(1))
		})
	})
})
