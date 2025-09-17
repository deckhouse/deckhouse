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
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const testRoot = "testdata/required_constraint_templates"

var _ = Describe("Modules :: admission-policy-engine :: hooks :: bootstrap_handler", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"internal": {"bootstrapped": false} } }`,
		`{"admissionPolicyEngine":{}}`,
	)
	f.RegisterCRD("templates.gatekeeper.sh", "v1", "ConstraintTemplate", true)
	Context("fresh cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/empty/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as false and have d8_admission_policy_engine_not_bootstrapped metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeFalse())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_not_bootstrapped",
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: nil,
			}))
		})
	})

	Context("Some constraint templates are missing", func() {
		BeforeEach(func() {
			f.KubeStateSet(constraintTemplate1)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as false and have d8_admission_policy_engine_not_bootstrapped metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeFalse())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_not_bootstrapped",
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: nil,
			}))
		})
	})

	Context("Required constraint templates are in place, but CRDs aren't created", func() {
		BeforeEach(func() {
			f.KubeStateSet(constraintTemplate1 + constraintTemplate2)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as false and have d8_admission_policy_engine_not_bootstrapped metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeFalse())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_not_bootstrapped",
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: nil,
			}))
		})
	})

	Context("Required constraint templates are in place, but some CRD's failed to be created", func() {
		BeforeEach(func() {
			f.KubeStateSet(constraintTemplate1 + statusNotCreated + constraintTemplate2 + statusCreated)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as false and have d8_admission_policy_engine_not_bootstrapped metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeFalse())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_not_bootstrapped",
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: nil,
			}))
		})
	})

	Context("Required constraint templates are in place, all CRD's are created", func() {
		BeforeEach(func() {
			f.KubeStateSet(constraintTemplate1 + statusCreated + constraintTemplate2 + statusCreated)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as true and have no d8_admission_policy_engine_not_bootstrapped metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeTrue())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_not_bootstrapped",
				Action: operation.ActionExpireMetrics,
			}))
		})
	})
})

func setTestChartPath(path string) error {
	return os.Setenv("D8_TEST_CHART_PATH", path)
}

var constraintTemplate1 = `
---
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8priorityclass
  labels:
    heritage: deckhouse
    module: admission-policy-engine
    security.deckhouse.io: operation-policy
  annotations:
    metadata.gatekeeper.sh/title: "Required Priority Class"
    metadata.gatekeeper.sh/version: 1.0.0
    description: "Required Priority Class"
`

var constraintTemplate2 = `
---
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8readonlyrootfilesystem
  labels:
    heritage: deckhouse
    module: admission-policy-engine
    security.deckhouse.io: security-policy
  annotations:
    metadata.gatekeeper.sh/title: "Read Only Root Filesystem"
    metadata.gatekeeper.sh/version: 1.0.0
`

var statusCreated = `
status:
  created: true
`

var statusNotCreated = `
status:
  created: false
`
