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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/module-sdk/pkg/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: alert_helm_project_templates ::", func() {
	// helmTemplate is a template that renders from a Helm text, with the given extra metadata lines.
	helmTemplate := func(name, metadata string) string {
		return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: %s
%s
spec:
  parametersSchema:
    openAPIV3Schema:
      type: object
  resourcesTemplate: |
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: settings
`, name, metadata)
	}
	// structuredTemplate is a template that renders from its structured fields, with the given
	// resourcesTemplate line, if any.
	structuredTemplate := func(name, resourcesTemplate string) string {
		return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: %s
spec:
  podSecurityStandard: Baseline
  parametersSchema:
    openAPIV3Schema:
      type: object
%s
`, name, resourcesTemplate)
	}

	expire := operation.MetricOperation{
		Group:  helmProjectTemplatesMetricGroup,
		Action: operation.ActionExpireMetrics,
	}
	series := func(templateName string) operation.MetricOperation {
		return operation.MetricOperation{
			Action: operation.ActionGaugeSet,
			// Spelled out, as the rule in monitoring/prometheus-rules/project-templates.yaml spells it.
			Name:   "d8_multitenancy_manager_helm_project_template",
			Value:  ptr.To(1.0),
			Group:  helmProjectTemplatesMetricGroup,
			Labels: map[string]string{"template": templateName},
		}
	}

	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha2", "ProjectTemplate", false)

	// expectExported checks that the run expires the group first and then sets a series for each of
	// the given templates and nothing else. addon-operator applies the operations of a group in order,
	// so an expire after the sets would drop every series, and a set-equal check would not see it.
	expectExported := func(templateNames ...string) {
		operations := f.MetricsCollector.CollectedMetrics()
		ExpectWithOffset(1, operations).NotTo(BeEmpty())
		ExpectWithOffset(1, operations[0]).To(Equal(expire))
		sets := make([]operation.MetricOperation, 0, len(templateNames))
		for _, name := range templateNames {
			sets = append(sets, series(name))
		}
		ExpectWithOffset(1, operations[1:]).To(ConsistOf(sets))
	}

	Context("there are no project templates", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("only expires the metric group", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectExported()
		})
	})

	Context("user templates render from Helm texts", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				helmTemplate("team-template", "") +
					helmTemplate("commander-v3", "  labels:\n    app: commander") +
					helmTemplate("chart-template", "  labels:\n    heritage: Helm") +
					structuredTemplate("structured", ""),
			))
			f.RunHook()
		})

		It("exports one series per template, whatever heritage label other than deckhouse it has", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectExported("team-template", "commander-v3", "chart-template")
		})
	})

	Context("the built-in templates are still the Helm texts of the previous release", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				helmTemplate("default", "  labels:\n    heritage: deckhouse") +
					helmTemplate("secure", "  labels:\n    heritage: deckhouse") +
					helmTemplate("secure-with-dedicated-nodes", "  labels:\n    heritage: deckhouse"),
			))
			f.RunHook()
		})

		It("exports nothing for them", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectExported()
		})
	})

	// The name alone keeps a built-in out, so that each name is pinned apart from the heritage label
	// that the built-ins of the previous release carry as well.
	for _, name := range []string{"default", "secure", "secure-with-dedicated-nodes"} {
		Context(fmt.Sprintf("a Helm text without the heritage label is named like the built-in template %s", name), func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(helmTemplate(name, "")))
				f.RunHook()
			})

			It("exports nothing for it, since the controller overwrites it with the built-in", func() {
				Expect(f).To(ExecuteSuccessfully())
				expectExported()
			})
		})
	}

	Context("a user template of the previous release is named simple", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(helmTemplate("simple", "")))
			f.RunHook()
		})

		It("exports nothing for it, since the controller overwrites it with the built-in simple", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectExported()
		})
	})

	Context("a template of Deckhouse has a name that is not built in", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(helmTemplate("retired", "  labels:\n    heritage: deckhouse")))
			f.RunHook()
		})

		It("exports nothing for it", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectExported()
		})
	})

	Context("a template is named like the template of the virtual projects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(helmTemplate("virtual", "")))
			f.RunHook()
		})

		It("exports nothing for it, since no project renders from it", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectExported()
		})
	})

	Context("templates have an empty or a blank resourcesTemplate", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				structuredTemplate("without-text", "") +
					structuredTemplate("empty-text", `  resourcesTemplate: ""`) +
					structuredTemplate("blank-text", `  resourcesTemplate: " \n\t"`),
			))
			f.RunHook()
		})

		It("exports nothing for them", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectExported()
		})
	})

	// A run starts from an expired group and keeps nothing of the run before, so a template drops out
	// of the alert in the first run that no longer finds its Helm text.
	Context("a template moves from its Helm text to structured fields", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(helmTemplate("team-template", "") + helmTemplate("other", "")))
			f.RunHook()
			f.BindingContexts.Set(f.KubeStateSet(structuredTemplate("team-template", "") + helmTemplate("other", "")))
			f.RunHook()
		})

		It("expires the group and exports only the other template", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectExported("other")
		})
	})

	Context("a template with a Helm text is deleted", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(helmTemplate("team-template", "")))
			f.RunHook()
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("only expires the group", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectExported()
		})
	})

	Context("the controller ships its built-in templates", func() {
		It("names every one of them in builtinTemplateNames", func() {
			files, err := filepath.Glob("../images/multitenancy-manager/src/templates/*.yaml")
			Expect(err).NotTo(HaveOccurred())
			Expect(files).NotTo(BeEmpty())

			names := make([]string, 0, len(files))
			for _, file := range files {
				raw, err := os.ReadFile(file)
				Expect(err).NotTo(HaveOccurred())
				var template struct {
					Metadata struct {
						Name string `json:"name"`
					} `json:"metadata"`
				}
				Expect(yaml.Unmarshal(raw, &template)).To(Succeed(), file)
				names = append(names, template.Metadata.Name)
			}
			Expect(names).To(ConsistOf(builtinTemplateNames))
		})
	})
})
