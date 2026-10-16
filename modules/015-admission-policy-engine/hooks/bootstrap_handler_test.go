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
	"strings"

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

	Context("Some constraint templates are missing, but the module was bootstrapped before", func() {
		BeforeEach(func() {
			f.KubeStateSet(createdTemplate("d8priorityclass") + webhookConfiguration("d8-admission-policy-engine-config") +
				constraintCRDManifest("D8PriorityClass", true) +
				// The template of this kind is not in the cluster: its CRD is a leftover.
				constraintCRDManifest("D8RequiredLabels", true) +
				// Gatekeeper has not established this CRD yet.
				constraintCRDManifest("D8AllowedRepos", false) + `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: unrelated.example.com
spec:
  group: example.com
  names:
    kind: Unrelated
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("keeps the webhooks, lists only the ready kinds and still reports the missing templates", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeTrue())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.readyConstraintKinds").String()).To(MatchJSON(`["D8PriorityClass"]`))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1].Name).To(Equal("d8_admission_policy_engine_not_bootstrapped"))
		})
	})

	Context("Some constraint templates are missing and only an unrelated webhook configuration exists", func() {
		BeforeEach(func() {
			f.KubeStateSet(createdTemplate("d8priorityclass") + webhookConfiguration("another-webhook") + constraintCRDManifest("D8PriorityClass", true))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("does not treat the module as bootstrapped, but still lists the ready kinds", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeFalse())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.readyConstraintKinds").String()).To(MatchJSON(`["D8PriorityClass"]`))
		})
	})

	Context("Every Gatekeeper pod is gone in a bootstrapped cluster", func() {
		BeforeEach(func() {
			// Gatekeeper deletes the status objects of its pods together with them, so the templates report
			// created: false, while their CRDs stay established.
			f.KubeStateSet(webhookConfiguration("d8-admission-policy-engine-config") +
				notCreatedTemplate("d8priorityclass") + constraintCRDManifest("D8PriorityClass", true))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("keeps the webhooks and the constraints of the established kinds", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeTrue())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.readyConstraintKinds").String()).To(MatchJSON(`["D8PriorityClass"]`))
		})
	})

	Context("The webhook configuration is deleted in a bootstrapped cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("admissionPolicyEngine.internal.bootstrapped", true)
			f.KubeStateSet(createdTemplate("d8priorityclass") + constraintCRDManifest("D8PriorityClass", true))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("keeps the flag set from its previous value and still reports the missing templates", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeTrue())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1].Name).To(Equal("d8_admission_policy_engine_not_bootstrapped"))
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

func createdTemplate(name string) string {
	return templateWithStatus(name, true)
}

func notCreatedTemplate(name string) string {
	return templateWithStatus(name, false)
}

func templateWithStatus(name string, created bool) string {
	return fmt.Sprintf(`
---
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: %s
  labels:
    heritage: deckhouse
    module: admission-policy-engine
status:
  created: %t
`, name, created)
}

func webhookConfiguration(name string) string {
	return `
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: ` + name + `
webhooks: []
`
}

func constraintCRDManifest(kind string, established bool) string {
	status := "False"
	if established {
		status = "True"
	}
	return `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ` + strings.ToLower(kind) + `.constraints.gatekeeper.sh
  labels:
    gatekeeper.sh/constraint: "yes"
spec:
  group: constraints.gatekeeper.sh
  names:
    kind: ` + kind + `
status:
  conditions:
  - type: Established
    status: "` + status + `"
`
}
