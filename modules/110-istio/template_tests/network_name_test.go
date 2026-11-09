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
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// The network name the sidecars and gateways already use is also the network of the
// d8-istio namespace and of ztunnel, whatever the mode. If the two disagree, ztunnel looks
// services up in the wrong network and bypasses their waypoints.
var _ = Describe("Module :: istio :: helm template :: network name", func() {
	f := SetupHelmConfig(``)

	setUp := func(version string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
		f.ValuesSet("istio.internal.globalVersion", version)
		f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["`+version+`"]`)
		f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `[]`)
	}

	storedNetwork := func() string {
		return f.KubernetesGlobalResource("Namespace", "d8-istio").Field(`metadata.annotations.istio\.deckhouse\.io/network-name`).String()
	}

	namespaceNetwork := func() gjson.Result {
		return f.KubernetesGlobalResource("Namespace", "d8-istio").Field(`metadata.labels.topology\.istio\.io/network`).Result
	}

	sidecarNetwork := func() string {
		values := f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x29").Field("data.values").String()
		return gjson.Get(values, "global.network").String()
	}

	ztunnelEnv := func(name string) string {
		ztunnel := f.KubernetesResource("DaemonSet", "d8-istio", "ztunnel")
		Expect(ztunnel.Exists()).To(BeTrue())
		return containerEnv(ztunnel.Field("spec.template.spec.containers").Array()[0], name)
	}

	waypointControllerEnv := func(name string) gjson.Result {
		deployment := f.KubernetesResource("Deployment", "d8-istio", "waypoint-controller")
		Expect(deployment.Exists()).To(BeTrue())
		return deployment.Field("spec.template.spec.containers.0.env.#(name==" + name + ").value").Result
	}

	Context("Neither ambient nor multicluster is enabled", func() {
		BeforeEach(func() {
			setUp("1.29")
			f.HelmRender()
		})

		It("Labels the namespace with the network the sidecars are injected with", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(storedNetwork()).To(Equal(expectedNetworkName))
			Expect(namespaceNetwork().String()).To(Equal(expectedNetworkName))
			Expect(sidecarNetwork()).To(Equal(expectedNetworkName))
		})
	})

	// The cluster domain cannot give this name, so it must come from values.
	Context("Ambient is enabled with a network name of a new installation", func() {
		const newNetworkName = "network-09710b4801ba99ae"

		BeforeEach(func() {
			setUp("1.29")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("istio.internal.networkName", newNetworkName)
			f.HelmRender()
		})

		It("Uses the name everywhere and stores it in the namespace", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(storedNetwork()).To(Equal(newNetworkName))
			Expect(namespaceNetwork().String()).To(Equal(newNetworkName))
			Expect(sidecarNetwork()).To(Equal(newNetworkName))
			Expect(ztunnelEnv("NETWORK")).To(Equal(newNetworkName))
			Expect(waypointControllerEnv("ISTIO_NETWORK_NAME").String()).To(Equal(newNetworkName))
		})
	})

	for _, version := range []string{"1.25", "1.29"} {
		Context("Ambient is enabled without multicluster on "+version, func() {
			BeforeEach(func() {
				setUp(version)
				f.ValuesSet("istio.ambient.enabled", true)
				f.HelmRender()
			})

			It("Puts the namespace, ztunnel and waypoints in the same network", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				Expect(namespaceNetwork().String()).To(Equal(expectedNetworkName))
				Expect(ztunnelEnv("NETWORK")).To(Equal(expectedNetworkName))
				Expect(waypointControllerEnv("ISTIO_NETWORK_NAME").String()).To(Equal(expectedNetworkName))
			})
		})
	}

	// An invalid label would make the namespace unapplicable and block waypoints, so the
	// name is left out of all labels.
	Context("Ambient is enabled on a cluster with a long network name", func() {
		BeforeEach(func() {
			setUp("1.29")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("istio.internal.networkName", longNetworkName)
			f.HelmRender()
		})

		It("Renders without a network for the namespace, ztunnel and waypoints", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("Namespace", "d8-istio").Exists()).To(BeTrue())
			Expect(namespaceNetwork().Exists()).To(BeFalse())
			Expect(ztunnelEnv("NETWORK")).To(BeEmpty())

			// Present but empty: the controller leaves the label out of its objects.
			Expect(waypointControllerEnv("ISTIO_NETWORK_NAME").Exists()).To(BeTrue())
			Expect(waypointControllerEnv("ISTIO_NETWORK_NAME").String()).To(BeEmpty())
		})

		// Clearing it would rename the network for the gateways and multicluster peers
		// that already use the long name.
		It("Still stores the long name and injects sidecars with it", func() {
			Expect(storedNetwork()).To(Equal(longNetworkName))
			Expect(sidecarNetwork()).To(Equal(longNetworkName))
		})
	})

	Context("The network name is not discovered", func() {
		BeforeEach(func() {
			setUp("1.29")
			f.ValuesDelete("istio.internal.networkName")
			f.HelmRender()
		})

		It("Refuses to render", func() {
			Expect(f.RenderError).To(HaveOccurred())
			Expect(f.RenderError.Error()).To(ContainSubstring("istio.internal.networkName is not set"))
		})
	})

	Context("Ambient is enabled with a network name of exactly 63 characters", func() {
		const maxNetworkName = "network-aaaaaaaaaa-bbbbbbbbbb-cccccccccc-dddddddddd-e-199688871"

		BeforeEach(func() {
			setUp("1.29")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("istio.internal.networkName", maxNetworkName)
			f.HelmRender()
		})

		It("Still labels the namespace", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(namespaceNetwork().String()).To(Equal(maxNetworkName))
		})
	})
})
