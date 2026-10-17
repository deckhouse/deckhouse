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

package migrate

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: migrate :: delete obsolete monitoring-deckhouse ClusterObservabilityDashboard", func() {
	const obsoleteDashboard = `
---
apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
metadata:
  name: d8-monitoring-deckhouse-main-deckhouse
spec:
  definition: "{}"
`
	const currentDashboard = `
---
apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
metadata:
  name: d8-deckhouse-main-deckhouse
spec:
  definition: "{}"
`

	Context("When the observability module is enabled and the obsolete dashboard exists", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("observability.deckhouse.io", "v1alpha1", "ClusterObservabilityDashboard", false)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[deckhouse, observability]`))
			f.KubeStateSet(obsoleteDashboard + currentDashboard)
			f.RunHook()
		})

		It("Deletes the obsolete dashboard and keeps the current one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-monitoring-deckhouse-main-deckhouse").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-deckhouse-main-deckhouse").Exists()).To(BeTrue())
		})
	})

	Context("When the observability module is disabled", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("observability.deckhouse.io", "v1alpha1", "ClusterObservabilityDashboard", false)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[deckhouse]`))
			f.KubeStateSet(obsoleteDashboard)
			f.RunHook()
		})

		It("Keeps the obsolete dashboard untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-monitoring-deckhouse-main-deckhouse").Exists()).To(BeTrue())
		})
	})

	Context("When the observability module is enabled and the obsolete dashboard is absent", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("observability.deckhouse.io", "v1alpha1", "ClusterObservabilityDashboard", false)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[deckhouse, observability]`))
			f.KubeStateSet(currentDashboard)
			f.RunHook()
		})

		It("Runs successfully and keeps the current dashboard", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-deckhouse-main-deckhouse").Exists()).To(BeTrue())
		})
	})
})
