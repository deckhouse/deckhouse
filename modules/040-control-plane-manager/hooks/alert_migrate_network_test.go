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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// The whole document, not lone keys: global.clusterConfiguration is validated against the
// ClusterConfiguration schema after every hook run, and a fragment fails that. Empty values omit
// the fields, which the schema now allows.
func clusterConfigurationNetworkValues(podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix string) string {
	doc := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
clusterDomain: cluster.local
defaultCRI: Containerd
kubernetesVersion: "1.34"
`
	if podSubnetCIDR != "" {
		doc += fmt.Sprintf("podSubnetCIDR: %s\n", podSubnetCIDR)
	}
	if serviceSubnetCIDR != "" {
		doc += fmt.Sprintf("serviceSubnetCIDR: %s\n", serviceSubnetCIDR)
	}
	if podSubnetNodeCIDRPrefix != "" {
		doc += fmt.Sprintf("podSubnetNodeCIDRPrefix: %q\n", podSubnetNodeCIDRPrefix)
	}
	return doc
}

var _ = Describe("Modules :: control-plane-manager :: hooks :: alert_migrate_network ::", func() {
	f := HookExecutionConfigInit(`{"controlPlaneManager":{}}`, `{}`)

	metricIsSet := func() bool {
		for _, m := range f.MetricsCollector.CollectedMetrics() {
			if m.Name == obsoleteNetworkFieldsMetricName {
				Expect(*m.Value).To(Equal(1.0))
				return true
			}
		}
		return false
	}

	DescribeTable("D8ObsoleteNetworkFieldsInClusterConfiguration metric",
		func(podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix string, expectSet bool) {
			f.ValuesSetFromYaml("global.clusterConfiguration", []byte(
				clusterConfigurationNetworkValues(podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix)))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(Equal(expectSet))
		},

		Entry("none of the three present — does not fire", "", "", "", false),
		Entry("only podSubnetCIDR present — fires", "10.111.0.0/16", "", "", true),
		Entry("only serviceSubnetCIDR present — fires", "", "10.222.0.0/16", "", true),
		Entry("only podSubnetNodeCIDRPrefix present — fires", "", "", "24", true),
		Entry("all three present — fires", "10.111.0.0/16", "10.222.0.0/16", "24", true),
	)

	// Presence in ClusterConfiguration is what fires the alert, not whether ModuleConfig already
	// resolves the value - the whole point is to prompt removing the deprecated field once it does.
	It("fires even when ModuleConfig already overrides the value", func() {
		f.ValuesSet("controlPlaneManager.network.podSubnetCIDR", "10.111.0.0/16")
		f.ValuesSetFromYaml("global.clusterConfiguration", []byte(
			clusterConfigurationNetworkValues("10.111.0.0/16", "", "")))
		f.RunHook()

		Expect(f).To(ExecuteSuccessfully())
		Expect(metricIsSet()).To(BeTrue())
	})
})
