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
			if m.Name == obsoleteKubernetesVersionFieldMetricName {
				Expect(*m.Value).To(Equal(1.0))
				return true
			}
		}
		return false
	}

	DescribeTable("D8ObsoleteKubernetesVersionFieldInClusterConfiguration metric",
		func(ccVersion, mcVersion string, expectSet bool) {
			f.ValuesDelete("controlPlaneManager.kubernetesVersion")
			if mcVersion != "" {
				f.ValuesSet("controlPlaneManager.kubernetesVersion", mcVersion)
			}
			f.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConfigurationValues(ccVersion)))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(Equal(expectSet))
		},

		Entry("CC pins a version, MC unset — fires", "1.34", "", true),
		Entry("CC field absent, MC unset — does not fire", "", "", false),
		Entry("CC field absent, MC pins a version — does not fire", "", "1.35", false),
		Entry("CC field absent, MC is Default — does not fire", "", "Default", false),

		Entry("CC and MC both pin a version — fires", "1.34", "1.35", true),
		Entry("CC pins a version, MC is Default — fires", "1.34", "Default", true),
	)

	It("ignores the kubernetesVersionIsDefault discovery flag", func() {
		f.ValuesDelete("controlPlaneManager.kubernetesVersion")
		f.ValuesSet("global.discovery.kubernetesVersionIsDefault", true)
		f.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConfigurationValues("1.34")))
		f.RunHook()

		Expect(f).To(ExecuteSuccessfully())
		Expect(metricIsSet()).To(BeTrue())
	})
})
