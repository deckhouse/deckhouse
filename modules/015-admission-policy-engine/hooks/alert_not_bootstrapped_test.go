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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: alert_not_bootstrapped", func() {

	Context("Module is bootstrapped", func() {
		f := HookExecutionConfigInit(
			`{"admissionPolicyEngine": {"internal": {"bootstrapped": true} } }`,
			`{"admissionPolicyEngine":{}}`,
		)
		BeforeEach(func() {
			f.RunHook()
		})

		It("Shouldn't have d8_admission_policy_engine_not_bootstrapped metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: "expire",
			}))
		})
	})

	Context("Module isn't bootstrapped", func() {
		f := HookExecutionConfigInit(
			`{"admissionPolicyEngine": {"internal": {"bootstrapped": false} } }`,
			`{"admissionPolicyEngine":{}}`,
		)

		BeforeEach(func() {
			f.RunHook()
		})

		It("Should have d8_admission_policy_engine_not_bootstrapped metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_not_bootstrapped",
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: "set",
				Value:  pointer.Float64(1),
				Labels: map[string]string{},
			}))
		})
	})
})
