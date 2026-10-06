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

package hooks

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/module-sdk/pkg/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: alert_on_project_templates ::", func() {
	// template renders a ProjectTemplate manifest with a fixed minimal spec. The hook reads only the name.
	template := func(name string) string {
		return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ProjectTemplate
metadata:
  name: %s
spec:
  parametersSchema:
    openAPIV3Schema:
      type: object
  resourcesTemplate: ""
`, name)
	}

	expire := operation.MetricOperation{
		Group:  projectTemplatesMetricGroup,
		Action: operation.ActionExpireMetrics,
	}
	reserved := func(templateName string) operation.MetricOperation {
		return operation.MetricOperation{
			Action: operation.ActionGaugeSet,
			Name:   reservedTemplateNameMetric,
			Value:  ptr.To(1.0),
			Group:  projectTemplatesMetricGroup,
			Labels: map[string]string{"template": templateName},
		}
	}

	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ProjectTemplate", false)

	Context("there are no project templates", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("only expires the metric group", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expire))
		})
	})

	Context("no template has a reserved name", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				template("default") +
					template("secure") +
					template("team-template") +
					template("simple-team"),
			))
			f.RunHook()
		})

		It("only expires the metric group", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expire))
		})
	})

	Context("a user template is named simple", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				template("simple") +
					template("default") +
					template("team-template"),
			))
			f.RunHook()
		})

		It("exports the reserved name for it alone", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expire, reserved("simple")))
		})
	})

	Context("the user template named simple is deleted", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(template("simple")))
			f.RunHook()
			f.BindingContexts.Set(f.KubeStateSet(template("my-simple")))
			f.RunHook()
		})

		It("exports nothing for it any more", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expire))
		})
	})
})
