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
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: delete obsolete monitoring-kubernetes-control-plane ClusterObservabilityDashboards", func() {
	const obsoleteDashboards = `
---
apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
metadata:
  name: d8-monitoring-kubernetes-control-plane-kubernetes-cluster-control-plane-status
spec:
  definition: "{}"
---
apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
metadata:
  name: d8-monitoring-kubernetes-control-plane-kubernetes-cluster-kube-etcd3
spec:
  definition: "{}"
---
apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
metadata:
  name: d8-monitoring-kubernetes-control-plane-kubernetes-cluster-deprecated-resources
spec:
  definition: "{}"
`
	const currentDashboards = `
---
apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
metadata:
  name: d8-control-plane-manager-kubernetes-cluster-control-plane-status
spec:
  definition: "{}"
`

	Context("When the observability module is enabled and the obsolete dashboards exist", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("observability.deckhouse.io", "v1alpha1", "ClusterObservabilityDashboard", false)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[control-plane-manager, observability]`))
			f.KubeStateSet(obsoleteDashboards + currentDashboards)
			f.RunHook()
		})

		It("Deletes all obsolete dashboards and keeps the current one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-monitoring-kubernetes-control-plane-kubernetes-cluster-control-plane-status").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-monitoring-kubernetes-control-plane-kubernetes-cluster-kube-etcd3").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-monitoring-kubernetes-control-plane-kubernetes-cluster-deprecated-resources").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-control-plane-manager-kubernetes-cluster-control-plane-status").Exists()).To(BeTrue())
		})
	})

	Context("When the observability module is disabled", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("observability.deckhouse.io", "v1alpha1", "ClusterObservabilityDashboard", false)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[control-plane-manager]`))
			f.KubeStateSet(obsoleteDashboards)
			f.RunHook()
		})

		It("Keeps the obsolete dashboards untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-monitoring-kubernetes-control-plane-kubernetes-cluster-control-plane-status").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-monitoring-kubernetes-control-plane-kubernetes-cluster-kube-etcd3").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-monitoring-kubernetes-control-plane-kubernetes-cluster-deprecated-resources").Exists()).To(BeTrue())
		})
	})

	Context("When the observability module is enabled and the obsolete dashboards are absent", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("observability.deckhouse.io", "v1alpha1", "ClusterObservabilityDashboard", false)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[control-plane-manager, observability]`))
			f.KubeStateSet(currentDashboards)
			f.RunHook()
		})

		It("Runs successfully and keeps the current dashboard", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ClusterObservabilityDashboard", "d8-control-plane-manager-kubernetes-cluster-control-plane-status").Exists()).To(BeTrue())
		})
	})
})
