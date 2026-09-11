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
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// clusterConfigurationSecretYAML builds a d8-cluster-configuration Secret carrying only the network
// fields under test, base64-encoded the way a real Secret's data map is (the test framework does not
// auto-encode a plain "data:" block).
func clusterConfigurationSecretYAML(podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix string) string {
	doc := "apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterType: Static\nclusterDomain: cluster.local\n"
	if podSubnetCIDR != "" {
		doc += fmt.Sprintf("podSubnetCIDR: %s\n", podSubnetCIDR)
	}
	if serviceSubnetCIDR != "" {
		doc += fmt.Sprintf("serviceSubnetCIDR: %s\n", serviceSubnetCIDR)
	}
	if podSubnetNodeCIDRPrefix != "" {
		doc += fmt.Sprintf("podSubnetNodeCIDRPrefix: %q\n", podSubnetNodeCIDRPrefix)
	}

	return fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  cluster-configuration.yaml: %s
`, base64.StdEncoding.EncodeToString([]byte(doc)))
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

	Context("no d8-cluster-configuration Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("does not fire", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(BeFalse())
		})
	})

	DescribeTable("D8ObsoleteNetworkFieldsInClusterConfiguration metric",
		func(podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix string, expectSet bool) {
			f.BindingContexts.Set(f.KubeStateSet(
				clusterConfigurationSecretYAML(podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix)))
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

	Context("fields removed from the Secret after a previous run had them", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				clusterConfigurationSecretYAML("10.111.0.0/16", "10.222.0.0/16", "24")))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(BeTrue())

			f.BindingContexts.Set(f.KubeStateSet(clusterConfigurationSecretYAML("", "", "")))
			f.RunHook()
		})

		// This is the regression the Values-based version of this hook could not detect: it read
		// global.clusterConfiguration.podSubnetCIDR, which the discovery hook keeps populated with
		// the ModuleConfig-resolved value even after the deprecated field is gone from the Secret,
		// so the metric never cleared. Reading the Secret directly does not have that problem.
		It("clears the metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(BeFalse())
		})
	})
})
