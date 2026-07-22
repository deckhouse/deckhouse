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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: alert_migrate_kubernetes_version ::", func() {
	f := HookExecutionConfigInit(`{"controlPlaneManager":{}}`, `{}`)

	DescribeTable("D8ObsoleteKubernetesVersionInClusterConfiguration metric",
		func(ccVersion, mcVersion string, expectSet bool) {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion", ccVersion)
			if mcVersion == "" {
				f.ValuesDelete("controlPlaneManager.kubernetesVersion")
			} else {
				f.ValuesSet("controlPlaneManager.kubernetesVersion", mcVersion)
			}
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			var found bool
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == "d8_obsolete_kubernetes_version_in_cluster_configuration" {
					found = true
					Expect(*m.Value).To(Equal(1.0))
				}
			}
			Expect(found).To(Equal(expectSet))
		},
		Entry("CC has explicit version, MC unset — fires", "1.34", "", true),
		Entry("CC has explicit version, MC is Automatic — fires", "1.34", "Automatic", true),
		Entry("CC has explicit version, MC overrides it — does not fire", "1.34", "1.35", false),
		Entry("CC is Automatic, MC unset — does not fire", "Automatic", "", false),
		Entry("CC unset, MC unset — does not fire", "", "", false),
	)
})
