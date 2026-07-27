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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: alert_migrate_kubernetes_version ::", func() {
	const secretTemplate = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  cluster-configuration.yaml: <<PLACEHOLDER_B64>>
`

	// The Secret carries the raw ClusterConfiguration, so "Automatic" survives here — unlike in
	// global.clusterConfiguration.kubernetesVersion, which the global discovery hook has already
	// resolved to a concrete version by the time this hook runs. That distinction is the whole
	// point of the Secret binding: without it every cluster looks like it pinned a version.
	buildState := func(ccRawVersion string) string {
		cc := fmt.Sprintf("apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nkubernetesVersion: %q\n", ccRawVersion)
		return strings.ReplaceAll(secretTemplate, "<<PLACEHOLDER_B64>>", base64.StdEncoding.EncodeToString([]byte(cc)))
	}

	f := HookExecutionConfigInit(`{"controlPlaneManager":{}}`, `{}`)

	metricIsSet := func() bool {
		for _, m := range f.MetricsCollector.CollectedMetrics() {
			if m.Name == obsoleteKubernetesVersionMetricName {
				Expect(*m.Value).To(Equal(1.0))
				return true
			}
		}
		return false
	}

	DescribeTable("D8ObsoleteKubernetesVersionInClusterConfiguration metric",
		func(ccRawVersion, mcVersion string, expectSet bool) {
			f.ValuesSet("controlPlaneManager.kubernetesVersion", "")
			if mcVersion != "" {
				f.ValuesSet("controlPlaneManager.kubernetesVersion", mcVersion)
			}
			f.BindingContexts.Set(f.KubeStateSet(buildState(ccRawVersion)))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(Equal(expectSet))

			// Same predicate feeds the T+2 release gate; keep alert and requirement in lockstep.
			migrated, exists := requirements.GetValue(KubernetesVersionMigratedRequirementKey)
			Expect(exists).To(BeTrue())
			Expect(migrated.(bool)).To(Equal(!expectSet))
		},
		// Nothing migrated yet: ClusterConfiguration still drives the version.
		Entry("CC pins a version, MC unset — fires", "1.34", "", true),
		Entry("CC pins a version, MC is Automatic — fires", "1.34", "Automatic", true),

		// ModuleConfig wins, so ClusterConfiguration is no longer consulted.
		Entry("CC pins a version, MC overrides it — does not fire", "1.34", "1.35", false),

		// Nothing to migrate: dropping the ClusterConfiguration field would change nothing.
		// This is the case the previous, Values-based implementation got wrong — it fired here,
		// on the majority of real clusters, and its own runbook could not clear the alert.
		Entry("CC is Automatic, MC unset — does not fire", "Automatic", "", false),
		Entry("CC is Automatic, MC is Automatic — does not fire", "Automatic", "Automatic", false),
		Entry("CC is Automatic, MC pins a version — does not fire", "Automatic", "1.35", false),
	)

	Context("ClusterConfiguration Secret is absent", func() {
		It("does not fire — there is nothing to migrate from", func() {
			f.ValuesSet("controlPlaneManager.kubernetesVersion", "")
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(metricIsSet()).To(BeFalse())

			migrated, exists := requirements.GetValue(KubernetesVersionMigratedRequirementKey)
			Expect(exists).To(BeTrue())
			Expect(migrated.(bool)).To(BeTrue())
		})
	})
})
