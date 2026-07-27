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

var _ = Describe("Module :: istio :: helm template :: waypoint-controller", func() {
	f := SetupHelmConfig(``)

	Context("Ambient mode enabled with global version 1.25.2 (supports ambient)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.HelmRender()
		})

		It("waypoint-controller resources should be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// Deployment
			deployment := f.KubernetesResource("Deployment", "d8-istio", "waypoint-controller")
			Expect(deployment.Exists()).To(BeTrue())
			Expect(deployment.Field("spec.template.spec.serviceAccountName").String()).To(Equal("waypoint-controller"))
			Expect(deployment.Field("spec.template.spec.containers.0.name").String()).To(Equal("waypoint-controller"))

			// Verify the image reference uses proxyv2 with the correct suffix
			envVars := deployment.Field("spec.template.spec.containers.0.env")
			foundWaypointProxyImage := false
			for _, env := range envVars.Array() {
				if env.Get("name").String() == "WAYPOINT_PROXY_IMAGE" {
					Expect(env.Get("value").String()).To(ContainSubstring("proxyv2V1x25x2"))
					foundWaypointProxyImage = true
					break
				}
			}
			Expect(foundWaypointProxyImage).To(BeTrue())

			// Verify ISTIO_REVISION env
			for _, env := range envVars.Array() {
				if env.Get("name").String() == "ISTIO_REVISION" {
					Expect(env.Get("value").String()).To(Equal("v1x25x2"))
				}
			}

			// Verify leader election enabled for HA
			args := deployment.Field("spec.template.spec.containers.0.args")
			Expect(args.Exists()).To(BeTrue())
			Expect(args.String()).To(ContainSubstring("--leader-elect=true"))

			// Verify health probe port
			Expect(deployment.Field("spec.template.spec.containers.0.ports.0.name").String()).To(Equal("healthz"))
			Expect(deployment.Field("spec.template.spec.containers.0.ports.0.containerPort").String()).To(Equal("9239"))

			// ServiceAccount
			sa := f.KubernetesResource("ServiceAccount", "d8-istio", "waypoint-controller")
			Expect(sa.Exists()).To(BeTrue())

			// ClusterRole
			cr := f.KubernetesGlobalResource("ClusterRole", "d8:istio:waypoint-controller")
			Expect(cr.Exists()).To(BeTrue())

			// ClusterRoleBinding
			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:waypoint-controller")
			Expect(crb.Exists()).To(BeTrue())

			// PDB
			pdb := f.KubernetesResource("PodDisruptionBudget", "d8-istio", "waypoint-controller")
			Expect(pdb.Exists()).To(BeTrue())
			Expect(pdb.Field("spec.minAvailable").String()).To(Equal("1"))

			// VPA
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint-controller")
			Expect(vpa.Exists()).To(BeTrue())
			Expect(vpa.Field("spec.targetRef.name").String()).To(Equal("waypoint-controller"))
			Expect(vpa.Field("spec.targetRef.kind").String()).To(Equal("Deployment"))
			Expect(vpa.Field("spec.targetRef.apiVersion").String()).To(Equal("apps/v1"))
			Expect(vpa.Field("spec.updatePolicy.updateMode").String()).To(Equal("InPlaceOrRecreate"))
		})
	})

	Context("Ambient mode enabled with global version 1.21.6 (does not support ambient)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.21.6")
			f.ValuesSet("istio.ambient.enabled", true)
			f.HelmRender()
		})

		It("waypoint-controller resources should NOT be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Deployment", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodDisruptionBudget", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())
		})
	})

	Context("Ambient mode disabled with global version 1.25.2", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", false)
			f.HelmRender()
		})

		It("waypoint-controller resources should NOT be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Deployment", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodDisruptionBudget", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())
		})
	})

	Context("Ambient enabled, VPA module disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			// Remove vertical-pod-autoscaler from enabledModules
			f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus","cert-manager","cni-cilium"]`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.HelmRender()
		})

		It("waypoint-controller resources should be created except VPA", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// Core resources still created
			Expect(f.KubernetesResource("Deployment", "d8-istio", "waypoint-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "waypoint-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:waypoint-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:waypoint-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("PodDisruptionBudget", "d8-istio", "waypoint-controller").Exists()).To(BeTrue())

			// VPA should NOT be created when vertical-pod-autoscaler module is disabled
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())
		})
	})

	Context("Ambient enabled, Static resources management for waypoint-controller", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSetFromYaml("istio.ambient.waypointController.resourcesManagement", `
mode: Static
static:
  requests:
    cpu: 50m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
`)
			f.HelmRender()
		})

		It("VPA should NOT be created, resources should be set on Deployment", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// VPA should NOT be created when mode is Static
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint-controller").Exists()).To(BeFalse())

			// Deployment should have the static resources
			deployment := f.KubernetesResource("Deployment", "d8-istio", "waypoint-controller")
			Expect(deployment.Exists()).To(BeTrue())

			resources := deployment.Field("spec.template.spec.containers.0.resources")
			Expect(resources.Exists()).To(BeTrue())
			Expect(resources.String()).To(MatchYAML(`
requests:
  cpu: 50m
  memory: 128Mi
  ephemeral-storage: 50Mi
limits:
  cpu: 500m
  memory: 512Mi
`))
		})
	})

	Context("Ambient enabled, VPA with custom resources for waypoint-controller", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSetFromYaml("istio.ambient.waypointController.resourcesManagement", `
mode: VPA
vpa:
  mode: Initial
  cpu:
    min: 50m
    max: "2"
    limitRatio: 2.5
  memory:
    min: 128Mi
    max: 1Gi
    limitRatio: 2.0
`)
			f.HelmRender()
		})

		It("VPA should be created with custom policies", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "waypoint-controller")
			Expect(vpa.Exists()).To(BeTrue())
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: waypoint-controller
updatePolicy:
  updateMode: Initial
resourcePolicy:
  containerPolicies:
  - containerName: waypoint-controller
    maxAllowed:
      cpu: "2"
      memory: 1Gi
    minAllowed:
      cpu: 50m
      memory: 128Mi
    controlledValues: RequestsAndLimits
`))
		})
	})

	Context("Ambient enabled, non-HA mode", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("global.highAvailability", false)
			f.HelmRender()
		})

		It("leader election should be disabled and PDB minAvailable should be 0", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-istio", "waypoint-controller")
			Expect(deployment.Exists()).To(BeTrue())

			// No leader election args in non-HA mode
			args := deployment.Field("spec.template.spec.containers.0.args")
			Expect(args.Exists()).To(BeTrue())
			Expect(args.String()).NotTo(ContainSubstring("--leader-elect=true"))

			// PDB minAvailable should be 0 in non-HA mode
			pdb := f.KubernetesResource("PodDisruptionBudget", "d8-istio", "waypoint-controller")
			Expect(pdb.Exists()).To(BeTrue())
			Expect(pdb.Field("spec.minAvailable").String()).To(Equal("0"))
		})
	})

	Context("Ambient enabled, waypoint-controller ClusterRole RBAC permissions", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.HelmRender()
		})

		It("ClusterRole should have all required rules", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cr := f.KubernetesGlobalResource("ClusterRole", "d8:istio:waypoint-controller")
			Expect(cr.Exists()).To(BeTrue())

			// Verify waypointinstances access
			rules := cr.Field("rules")
			Expect(rules.Exists()).To(BeTrue())

			// Helper: find rule by apiGroup and resource
			findRule := func(apiGroup, resource string) bool {
				for _, rule := range rules.Array() {
					apiGroups := rule.Get("apiGroups")
					resources := rule.Get("resources")
					if !apiGroups.Exists() || !resources.Exists() {
						continue
					}
					for _, ag := range apiGroups.Array() {
						if ag.String() == apiGroup || (apiGroup == "" && ag.String() == "") {
							for _, r := range resources.Array() {
								if r.String() == resource {
									return true
								}
							}
						}
					}
				}
				return false
			}

			Expect(findRule("network.deckhouse.io", "waypointinstances")).To(BeTrue())
			Expect(findRule("network.deckhouse.io", "waypointinstances/status")).To(BeTrue())
			Expect(findRule("apps", "deployments")).To(BeTrue())
			Expect(findRule("", "services")).To(BeTrue())
			Expect(findRule("", "serviceaccounts")).To(BeTrue())
			Expect(findRule("gateway.networking.k8s.io", "gateways")).To(BeTrue())
			Expect(findRule("policy", "poddisruptionbudgets")).To(BeTrue())
			Expect(findRule("autoscaling", "horizontalpodautoscalers")).To(BeTrue())
			Expect(findRule("autoscaling.k8s.io", "verticalpodautoscalers")).To(BeTrue())
			Expect(findRule("coordination.k8s.io", "leases")).To(BeTrue())
			Expect(findRule("", "events")).To(BeTrue())
		})
	})

	Context("Ambient enabled, ClusterRoleBinding subject", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.HelmRender()
		})

		It("ClusterRoleBinding should reference correct ServiceAccount", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:waypoint-controller")
			Expect(crb.Exists()).To(BeTrue())

			Expect(crb.Field("subjects.0.kind").String()).To(Equal("ServiceAccount"))
			Expect(crb.Field("subjects.0.name").String()).To(Equal("waypoint-controller"))
			Expect(crb.Field("subjects.0.namespace").String()).To(Equal("d8-istio"))

			Expect(crb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
			Expect(crb.Field("roleRef.name").String()).To(Equal("d8:istio:waypoint-controller"))
		})
	})

	Context("Ambient enabled, VPA_ENABLED env var", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.HelmRender()
		})

		It("VPA_ENABLED env var should be true when vertical-pod-autoscaler module is enabled", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-istio", "waypoint-controller")
			Expect(deployment.Exists()).To(BeTrue())

			envVars := deployment.Field("spec.template.spec.containers.0.env")
			found := false
			for _, env := range envVars.Array() {
				if env.Get("name").String() == "VPA_ENABLED" {
					Expect(env.Get("value").String()).To(Equal("true"))
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})
	})
})
