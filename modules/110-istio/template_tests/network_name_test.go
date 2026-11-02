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

	namespaceNetwork := func() gjson.Result {
		return f.KubernetesGlobalResource("Namespace", "d8-istio").Field(`metadata.labels.topology\.istio\.io/network`).Result
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
			Expect(namespaceNetwork().String()).To(Equal(expectedNetworkName))
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x29").
				Field("data.values").String()).To(ContainSubstring(`"network": "` + expectedNetworkName + `"`))
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

	// The network name is `len(clusterDomain) + 19` characters long, so a cluster domain
	// over 44 characters makes it an invalid label value. The namespace label would make
	// the whole module unapplicable, and the waypoint labels would keep waypoints from
	// being created, so the name is left out of every place a label value goes.
	Context("Ambient is enabled on a cluster with a long cluster domain", func() {
		const longClusterDomain = "aaaaaaaaaa.bbbbbbbbbb.cccccccccc.dddddddddd.eee"

		BeforeEach(func() {
			setUp("1.29")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("global.discovery.clusterDomain", longClusterDomain)
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
		// that already use the long name, so it waits until the name is configurable.
		It("Still injects sidecars with the long name", func() {
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x29").
				Field("data.values").String()).To(ContainSubstring(`"network": "network-aaaaaaaaaa-bbbbbbbbbb-`))
		})
	})
})
