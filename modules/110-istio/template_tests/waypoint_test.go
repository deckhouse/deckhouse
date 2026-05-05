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

var _ = Describe("Module :: istio :: helm template :: waypoint", func() {
	f := SetupHelmConfig(``)

	Context("Ambient mode enabled, waypoint enabled, global version 1.25.2 (supports ambient)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("istio.dataPlane.waypoint.enabled", true)
			f.HelmRender()
		})

		It("waypoint resources should be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			waypointDeploy := f.KubernetesResource("Deployment", "d8-istio", "waypoint")
			Expect(waypointDeploy.Exists()).To(BeTrue())
			Expect(waypointDeploy.Field("spec.template.spec.serviceAccountName").String()).To(Equal("waypoint"))

			waypointSa := f.KubernetesResource("ServiceAccount", "d8-istio", "waypoint")
			Expect(waypointSa.Exists()).To(BeTrue())

			waypointSvc := f.KubernetesResource("Service", "d8-istio", "waypoint")
			Expect(waypointSvc.Exists()).To(BeTrue())
			Expect(waypointSvc.Field("spec.type").String()).To(Equal("ClusterIP"))

			waypointGateway := f.KubernetesResource("Gateway", "d8-istio", "waypoint")
			Expect(waypointGateway.Exists()).To(BeTrue())
			Expect(waypointGateway.Field("spec.gatewayClassName").String()).To(Equal("istio-waypoint"))

			waypointVpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint")
			Expect(waypointVpa.Exists()).To(BeTrue())
			Expect(waypointVpa.Field("spec.targetRef.name").String()).To(Equal("waypoint"))
			Expect(waypointVpa.Field("spec.targetRef.kind").String()).To(Equal("Deployment"))

			waypointPodMonitor := f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-waypoint")
			Expect(waypointPodMonitor.Exists()).To(BeTrue())

			waypointHpa := f.KubernetesResource("HorizontalPodAutoscaler", "d8-istio", "waypoint")
			Expect(waypointHpa.Exists()).To(BeFalse())
		})
	})

	Context("Ambient mode enabled, waypoint enabled, global version 1.21.6 (does not support ambient)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.21.6")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("istio.dataPlane.waypoint.enabled", true)
			f.HelmRender()
		})

		It("waypoint resources should NOT be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Deployment", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Gateway", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("HorizontalPodAutoscaler", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-waypoint").Exists()).To(BeFalse())
		})
	})

	Context("Ambient mode disabled, waypoint enabled, global version 1.25.2", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", false)
			f.ValuesSet("istio.dataPlane.waypoint.enabled", true)
			f.HelmRender()
		})

		It("waypoint resources should NOT be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Deployment", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Gateway", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("HorizontalPodAutoscaler", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-waypoint").Exists()).To(BeFalse())
		})
	})

	Context("Ambient mode enabled, waypoint disabled, global version 1.25.2", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("istio.dataPlane.waypoint.enabled", false)
			f.HelmRender()
		})

		It("waypoint resources should NOT be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Deployment", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Gateway", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("HorizontalPodAutoscaler", "d8-istio", "waypoint").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-waypoint").Exists()).To(BeFalse())
		})
	})

	Context("Waypoint with HPA replicasManagement mode", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSetFromYaml("istio.dataPlane.waypoint", `
enabled: true
replicasManagement:
  mode: HPA
  hpa:
    minReplicas: 2
    maxReplicas: 5
    metrics:
    - type: CPU
      targetAverageUtilization: 80
resourcesManagement:
  mode: Static
  static:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 200m
      memory: 256Mi
`)
			f.HelmRender()
		})

		It("HPA should be created and deployment should not have replicas set", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			waypointHpa := f.KubernetesResource("HorizontalPodAutoscaler", "d8-istio", "waypoint")
			Expect(waypointHpa.Exists()).To(BeTrue())
			Expect(waypointHpa.Field("spec.minReplicas").Int()).To(Equal(int64(2)))
			Expect(waypointHpa.Field("spec.maxReplicas").Int()).To(Equal(int64(5)))
			Expect(waypointHpa.Field("spec.scaleTargetRef.name").String()).To(Equal("waypoint"))
			Expect(waypointHpa.Field("spec.scaleTargetRef.kind").String()).To(Equal("Deployment"))

			waypointDeploy := f.KubernetesResource("Deployment", "d8-istio", "waypoint")
			Expect(waypointDeploy.Exists()).To(BeTrue())
			Expect(waypointDeploy.Field("spec.replicas").Exists()).To(BeFalse())

			waypointVpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint")
			Expect(waypointVpa.Exists()).To(BeFalse())
		})
	})
})
