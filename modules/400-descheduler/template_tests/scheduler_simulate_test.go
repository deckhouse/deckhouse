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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: descheduler :: helm template :: scheduler simulate target check", func() {
	f := SetupHelmConfig(``)

	render := func(enabledModules string) {
		f.ValuesSetFromYaml("global", `
  enabledModules: `+enabledModules+`
  modules:
    placement: {}
  discovery:
    clusterDomain: example.org
    kubernetesVersion: 1.33.1
    d8SpecificNodeCountByRole:
      master: 3
`)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("descheduler", `
internal:
  deschedulers:
  - name: test
    strategies:
      lowNodeUtilization:
        enabled: true
        thresholds:
          cpu: 20
        targetThresholds:
          cpu: 50
`)
		f.HelmRender()
	}

	env := func(name string) string {
		return f.KubernetesResource("Deployment", "d8-descheduler", "descheduler").
			Field(`spec.template.spec.containers.#(name=="descheduler").env.#(name=="` + name + `").value`).String()
	}

	Context("control-plane-manager module enabled", func() {
		BeforeEach(func() { render(`["vertical-pod-autoscaler", "control-plane-manager"]`) })

		It("Should point the descheduler to the simulate endpoint of control-plane-proxy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(env("DESCHEDULER_SCHEDULER_SIMULATE_URL")).To(Equal("https://control-plane-proxy.kube-system.svc.example.org:8080/scheduler/simulate"))
			Expect(env("DESCHEDULER_SCHEDULER_SIMULATE_INSECURE_SKIP_VERIFY")).To(Equal("true"))
			Expect(env("DESCHEDULER_PROMETHEUS_INSECURE_SKIP_VERIFY")).To(BeEmpty())
		})

		It("Should bind the descheduler ServiceAccount to the simulate ClusterRole", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:descheduler:scheduler-simulate")
			Expect(binding.Field("roleRef")).To(MatchYAML(`
apiGroup: rbac.authorization.k8s.io
kind: ClusterRole
name: d8:control-plane-manager:scheduler-simulate
`))
			Expect(binding.Field("subjects")).To(MatchYAML(`
- kind: ServiceAccount
  name: descheduler
  namespace: d8-descheduler
`))
		})
	})

	Context("control-plane-manager module enabled with a Prometheus-based Descheduler", func() {
		BeforeEach(func() {
			render(`["vertical-pod-autoscaler", "control-plane-manager", "prometheus"]`)
			f.ValuesSetFromYaml("descheduler.internal.deschedulers", `
- name: average
  strategies:
    lowNodeUtilization:
      enabled: true
      metricsSource: Prometheus
      thresholds:
        cpu: 20
      targetThresholds:
        cpu: 50
`)
			f.HelmRender()
		})

		It("Should set the simulate and the Prometheus env together", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(env("DESCHEDULER_SCHEDULER_SIMULATE_URL")).To(Equal("https://control-plane-proxy.kube-system.svc.example.org:8080/scheduler/simulate"))
			Expect(env("DESCHEDULER_PROMETHEUS_INSECURE_SKIP_VERIFY")).To(Equal("true"))
		})
	})

	Context("control-plane-manager module disabled (managed Kubernetes)", func() {
		BeforeEach(func() { render(`["vertical-pod-autoscaler"]`) })

		It("Should neither set the simulate env nor bind the ClusterRole", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Deployment", "d8-descheduler", "descheduler").
				Field(`spec.template.spec.containers.#(name=="descheduler").env`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:descheduler:scheduler-simulate").Exists()).To(BeFalse())
		})
	})
})
