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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: sync_desired_kubernetes_version ::", func() {
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

	buildState := func(ccRawVersion string) string {
		cc := fmt.Sprintf("apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nkubernetesVersion: \"%s\"\n", ccRawVersion)
		return strings.ReplaceAll(secretTemplate, "<<PLACEHOLDER_B64>>", base64.StdEncoding.EncodeToString([]byte(cc)))
	}

	// resolvedCC simulates what the global discovery hook already put into
	// global.clusterConfiguration.kubernetesVersion by the time module hooks run: "Automatic" is
	// always replaced by a concrete version before it reaches Values.
	resolvedCC := func(ccRawVersion string) string {
		if ccRawVersion == "Automatic" {
			return "1.34"
		}
		return ccRawVersion
	}

	f := HookExecutionConfigInit(`{"controlPlaneManager":{}}`, `{}`)

	DescribeTable("resolves desiredVersion/updateMode and writes ConfigMap d8-cluster-kubernetes",
		func(mcVersion, ccRawVersion, wantDesired, wantMode string) {
			f.ValuesSet("controlPlaneManager.kubernetesVersion", "")
			if mcVersion != "" {
				f.ValuesSet("controlPlaneManager.kubernetesVersion", mcVersion)
			}
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion", resolvedCC(ccRawVersion))
			f.BindingContexts.Set(f.KubeStateSet(buildState(ccRawVersion)))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes")
			Expect(cm.Exists()).To(BeTrue())
			spec := cm.Field("data.spec").String()
			Expect(spec).To(ContainSubstring(fmt.Sprintf("desiredVersion: %q", wantDesired)))
			Expect(spec).To(ContainSubstring(fmt.Sprintf("updateMode: %s", wantMode)))

			// Only spec is ever touched — status/labels are update-observer's exclusively.
			Expect(cm.Field("data.status").Exists()).To(BeFalse())
		},
		Entry("explicit MC version wins over an explicit CC pin — Manual", "1.35", "1.33", "1.35", "Manual"),
		Entry("MC explicitly Automatic defers to CC, tracked as Automatic", "Automatic", "1.33", "1.33", "Automatic"),
		Entry("no MC, explicit CC pin — Manual (legacy, unmigrated)", "", "1.34", "1.34", "Manual"),
		Entry("no MC, CC Automatic — Automatic", "", "Automatic", "1.34", "Automatic"),
	)

	Context("ConfigMap already exists with unrelated status/labels", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"kubernetesVersion": "1.35"}}`, `{}`)

		const existingConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
    k8s-version: "1.34"
    max-k8s-version: "1.34"
data:
  spec: |
    desiredVersion: "1.34"
    updateMode: Manual
  status: |
    currentVersion: "1.34"
    phase: UpToDate
`

		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion", "1.34")
			f.BindingContexts.Set(f.KubeStateSet(existingConfigMap + buildState("1.34")))
			f.RunHook()
		})

		It("updates only data.spec, leaving data.status and labels untouched", func() {
			Expect(f).To(ExecuteSuccessfully())

			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes")
			Expect(cm.Field("data.spec").String()).To(ContainSubstring(`desiredVersion: "1.35"`))
			Expect(cm.Field("data.status").String()).To(ContainSubstring("currentVersion: \"1.34\""))
			Expect(cm.Field("metadata.labels.k8s-version").String()).To(Equal("1.34"))
			Expect(cm.Field("metadata.labels.max-k8s-version").String()).To(Equal("1.34"))
		})
	})
})
