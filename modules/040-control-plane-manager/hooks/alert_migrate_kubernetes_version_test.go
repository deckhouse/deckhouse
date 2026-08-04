/*
Copyright 2024 Flant JSC

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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: alert_migrate_kubernetes_version ::", func() {
	f := HookExecutionConfigInit(`{"controlPlaneManager":{}}`, `{}`)

	metricIsSet := func() bool {
		for _, m := range f.MetricsCollector.CollectedMetrics() {
			if m.Name == unsetKubernetesVersionMetricName {
				Expect(*m.Value).To(Equal(1.0))
				return true
			}
		}
		return false
	}

	DescribeTable("D8UnsetKubernetesVersionInModuleConfig metric",
		func(mcVersion string, expectSet bool) {
			f.ValuesSet("controlPlaneManager.kubernetesVersion", "")
			if mcVersion != "" {
				f.ValuesSet("controlPlaneManager.kubernetesVersion", mcVersion)
			}
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(Equal(expectSet))

			// Same predicate feeds the T+1 release gate; keep alert and requirement in lockstep.
			migrated, exists := requirements.GetValue(KubernetesVersionMigratedRequirementKey)
			Expect(exists).To(BeTrue())
			Expect(migrated.(bool)).To(Equal(!expectSet))
		},
		// Conscious fleet spam: unset ModuleConfig kubernetesVersion always fires.
		Entry("MC unset — fires", "", true),

		// Any explicit setting clears the alert, including Default / Automatic (R6).
		Entry("MC pins a version — does not fire", "1.35", false),
		Entry("MC is Automatic — does not fire", "Automatic", false),
		Entry("MC is Default — does not fire", "Default", false),
	)
})
