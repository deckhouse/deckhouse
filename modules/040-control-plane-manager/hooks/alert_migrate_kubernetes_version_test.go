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

	// trackingDefault is global.discovery.kubernetesVersionIsDefault, published by the global discovery
	// hook: true when ModuleConfig says Default, or when nothing pins the version anywhere. The
	// combinations below are the ones that hook can actually produce — pairing a ModuleConfig pin with
	// trackingDefault=true would encode a state no cluster reaches.
	DescribeTable("D8UnsetKubernetesVersionInModuleConfig metric",
		func(mcVersion string, trackingDefault, expectSet bool) {
			f.ValuesDelete("controlPlaneManager.kubernetesVersion")
			if mcVersion != "" {
				f.ValuesSet("controlPlaneManager.kubernetesVersion", mcVersion)
			}
			f.ValuesSet("global.discovery.kubernetesVersionIsDefault", trackingDefault)
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(Equal(expectSet))
		},
		// The one case worth migrating: ModuleConfig is silent and the version is pinned anyway, which
		// leaves ClusterConfiguration as the only document that can be pinning it.
		Entry("MC unset, version pinned outside ModuleConfig — fires", "", false, true),

		// Nothing to migrate: the resolved version is the release default either way, exactly as
		// it would be with ModuleConfig Default.
		Entry("MC unset, cluster tracks the release default — does not fire", "", true, false),

		// Any explicit setting takes ownership away from ClusterConfiguration, Default included.
		Entry("MC pins a version — does not fire", "1.35", false, false),
		Entry("MC is Default — does not fire", "Default", true, false),
	)

	// Regression. The hook used to read global.clusterConfiguration.kubernetesVersion, where the
	// discovery hook has already replaced Automatic with the concrete release default. A cluster with
	// ClusterConfiguration.kubernetesVersion: Automatic therefore looked pinned and kept the alert lit
	// — the exact case the alert must stay silent on. The previous test missed it by writing a literal
	// "Automatic" into that value, which no cluster ever presents to this hook.
	It("stays silent when the ClusterConfiguration value is the substituted release default", func() {
		f.ValuesDelete("controlPlaneManager.kubernetesVersion")
		f.ValuesSet("global.discovery.kubernetesVersionIsDefault", true)
		f.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConfigurationValues("1.34")))
		f.RunHook()

		Expect(f).To(ExecuteSuccessfully())
		Expect(metricIsSet()).To(BeFalse())
	})

	DescribeTable("kubernetesVersionNeedsMigration",
		func(mcVersion string, trackingDefault, expected bool) {
			Expect(kubernetesVersionNeedsMigration(mcVersion, trackingDefault)).To(Equal(expected))
		},
		Entry("MC unset, version pinned outside ModuleConfig", "", false, true),
		Entry("MC unset, cluster tracks the release default", "", true, false),
		Entry("MC pins a version", "1.35", false, false),
		Entry("MC is Default", "Default", true, false),
	)
})
