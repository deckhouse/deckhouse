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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// The whole document, not a lone kubernetesVersion key: global.clusterConfiguration is validated
// against the ClusterConfiguration schema after every hook run, and a fragment fails that. An empty
// version omits the field, which the schema now allows.
func clusterConfigurationValues(kubernetesVersion string) string {
	doc := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
clusterDomain: cluster.local
defaultCRI: Containerd
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`
	if kubernetesVersion != "" {
		doc += fmt.Sprintf("kubernetesVersion: %q\n", kubernetesVersion)
	}

	return doc
}

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

	// Only values both schemas accept: the ClusterConfiguration enum has Automatic and no Default,
	// the ModuleConfig one has Default and no Automatic. The sentinel matrix is covered by the unit
	// table below, which is not bound by either schema.
	DescribeTable("D8UnsetKubernetesVersionInModuleConfig metric",
		func(mcVersion, ccVersion string, expectSet bool) {
			f.ValuesDelete("controlPlaneManager.kubernetesVersion")
			if mcVersion != "" {
				f.ValuesSet("controlPlaneManager.kubernetesVersion", mcVersion)
			}
			f.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConfigurationValues(ccVersion)))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(Equal(expectSet))
		},
		// The one case worth migrating: ClusterConfiguration still decides the version.
		Entry("MC unset, CC pins a version — fires", "", "1.34", true),

		// Nothing to migrate: the resolved version is the release default either way, exactly as
		// it would be with ModuleConfig Default.
		Entry("MC unset, CC unset — does not fire", "", "", false),
		Entry("MC unset, CC is Automatic — does not fire", "", "Automatic", false),

		// Any explicit setting takes ownership away from ClusterConfiguration, Default included.
		Entry("MC pins a version — does not fire", "1.35", "1.34", false),
		Entry("MC is Default — does not fire", "Default", "1.34", false),
	)

	DescribeTable("kubernetesVersionNeedsMigration",
		func(mcVersion, ccVersion string, expected bool) {
			Expect(kubernetesVersionNeedsMigration(mcVersion, ccVersion)).To(Equal(expected))
		},
		Entry("MC unset, CC pins a version", "", "1.34", true),
		Entry("MC unset, CC unset", "", "", false),
		Entry("MC unset, CC is Automatic", "", "Automatic", false),
		// Not accepted by the ClusterConfiguration schema, so it can only arrive on an object that
		// predates the closed enum. It still means "track the Deckhouse default", not a pin.
		Entry("MC unset, CC is Default", "", "Default", false),
		Entry("MC pins a version", "1.35", "1.34", false),
		Entry("MC is Default", "Default", "1.34", false),
		Entry("MC is Default, CC unset", "Default", "", false),
	)
})
