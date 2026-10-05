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

// Native ambient multicluster (multicluster.mode: Ambient): the ambient east-west gateway,
// the per-peer `istio-remote` Gateways, network labelling, the sidecar path the mode turns
// off, and the switches that keep the feature off. globalValues and istioValues come from
// module_test.go.

// Peers covering every case the per-peer `istio-remote` Gateway has to distinguish: one
// running the ambient gateway, one not, a flat peer that opted out of the gateway hop, one
// publishing several addresses, one whose own name is that peer's name plus an index - the
// pair that collides if the index is ever omitted for the first address - and one reached
// by DNS name rather than by IP, which is the half of the address type the object declares.
//
// Unusable addresses are not among them: a peer's ambient endpoints are validated by the
// multicluster discovery hook, at the point they enter the cluster, so nothing that reaches
// these values can be malformed. See TestSanitizeAmbientGateways.
const ambientMulticlusters = `
- name: neighbour-ambient
  apiHost: remote.api.example.com
  apiJWT: aAaA.bBbB.CcCc
  enableIngressGateway: true
  insecureSkipVerify: false
  ingressGateways:
  - address: 1.1.1.1
    port: 15443
  ambientGateways:
  - address: 1.1.1.2
    addressType: IPAddress
    port: 15008
  networkName: network-neigh-ambient
  clusterID: neigh-ambient
  metadataExporterCA: ""
  rootCA: ---ROOT CA---
  spiffeEndpoint: https://some-proper-host/spiffe-bundle-endpoint
- name: neighbour-sidecar-only
  apiHost: remote2.api.example.com
  apiJWT: aAaA.bBbB.CcCc
  enableIngressGateway: true
  insecureSkipVerify: false
  ingressGateways:
  - address: 2.2.2.1
    port: 15443
  networkName: network-neigh-sidecar-only
  clusterID: neigh-sidecar-only
  metadataExporterCA: ""
  rootCA: ---ROOT CA---
  spiffeEndpoint: https://some-proper-host/spiffe-bundle-endpoint
- name: neighbour-multi-address
  apiHost: remote4.api.example.com
  apiJWT: aAaA.bBbB.CcCc
  enableIngressGateway: true
  insecureSkipVerify: false
  ingressGateways:
  - address: 4.4.4.1
    port: 15443
  ambientGateways:
  - address: "2001:db8::1"
    addressType: IPAddress
    port: 15008
  - address: 4.4.4.2
    addressType: IPAddress
    port: 15008
  networkName: network-neigh-multi-address
  clusterID: neigh-multi-address
  metadataExporterCA: ""
  rootCA: ---ROOT CA---
  spiffeEndpoint: https://some-proper-host/spiffe-bundle-endpoint
- name: neighbour-multi-address-1
  apiHost: remote5.api.example.com
  apiJWT: aAaA.bBbB.CcCc
  enableIngressGateway: true
  insecureSkipVerify: false
  ingressGateways:
  - address: 5.5.5.1
    port: 15443
  ambientGateways:
  - address: 5.5.5.2
    addressType: IPAddress
    port: 15008
  networkName: network-neigh-multi-address-1
  clusterID: neigh-multi-address-1
  metadataExporterCA: ""
  rootCA: ---ROOT CA---
  spiffeEndpoint: https://some-proper-host/spiffe-bundle-endpoint
- name: neighbour-hostname
  apiHost: remote6.api.example.com
  apiJWT: aAaA.bBbB.CcCc
  enableIngressGateway: true
  insecureSkipVerify: false
  ingressGateways:
  - address: 6.6.6.1
    port: 15443
  ambientGateways:
  - address: ambient.lb.example.com
    addressType: Hostname
    port: 15008
  networkName: network-neigh-hostname
  clusterID: neigh-hostname
  metadataExporterCA: ""
  rootCA: ---ROOT CA---
  spiffeEndpoint: https://some-proper-host/spiffe-bundle-endpoint
- name: neighbour-flat
  apiHost: remote3.api.example.com
  apiJWT: aAaA.bBbB.CcCc
  enableIngressGateway: false
  insecureSkipVerify: false
  ambientGateways:
  - address: 3.3.3.1
    addressType: IPAddress
    port: 15008
  networkName: network-neigh-flat
  clusterID: neigh-flat
  metadataExporterCA: ""
  rootCA: ---ROOT CA---
  spiffeEndpoint: https://some-proper-host/spiffe-bundle-endpoint
`

const expectedNetworkName = "network-my-domain-199688871"

// setUpAmbientMulticluster applies the values every ambient multicluster context needs.
func setUpAmbientMulticluster(f *Config) {
	f.ValuesSetFromYaml("global", globalValues)
	f.ValuesSet("global.modulesImages", GetModulesImages())
	f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
	f.ValuesSet("istio.internal.globalVersion", "1.29")
	f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.29"]`)
	f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `[]`)
	f.ValuesSet("istio.ambient.enabled", true)
	f.ValuesSet("istio.multicluster.enabled", true)
	f.ValuesSet("istio.multicluster.mode", "Ambient")
	// Unused in the Ambient mode. Set to show that the sidecar gateway stays off anyway.
	f.ValuesSet("istio.internal.multiclustersNeedIngressGateway", true)
}

// exporterEnv returns the value of an env var on the metadata-exporter container, or ""
// if unset.
func exporterEnv(f *Config, name string) string {
	deployment := f.KubernetesResource("Deployment", "d8-istio", "metadata-exporter")
	if !deployment.Exists() {
		return ""
	}

	return containerEnv(deployment.Field("spec.template.spec.containers").Array()[0], name)
}

// istiodEnv returns the value of an env var on the istiod container, or "" if unset.
func istiodEnv(f *Config, revision, name string) string {
	deployment := f.KubernetesResource("Deployment", "d8-istio", "istiod-"+revision)
	if !deployment.Exists() {
		return ""
	}
	return containerEnv(deployment.Field("spec.template.spec.containers").Array()[0], name)
}

func containerEnv(container gjson.Result, name string) string {
	for _, env := range container.Get("env").Array() {
		if env.Get("name").String() == name {
			return env.Get("value").String()
		}
	}
	return ""
}

var _ = Describe("Module :: istio :: helm template :: ambient multicluster", func() {
	f := SetupHelmConfig(``)

	Context("There are some multiclusters and ambient multicluster is enabled", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSetFromYaml("istio.internal.multiclusters", ambientMulticlusters)
			f.HelmRender()
		})

		It("Renders the ambient east-west gateway workload, which Helm owns end to end", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// No Gateway of our own: peers can only stop discovering an observed address
			// from us if istiod publishes no istio-east-west Gateway at all.
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ambientgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-east-west-gateway-defaults").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-istio", "ambientgateway").Exists()).To(BeFalse())

			ds := f.KubernetesResource("DaemonSet", "d8-istio", "ambientgateway")
			Expect(ds.Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-ambientgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-ambientgateway").Exists()).To(BeTrue())

			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ambientgateway")
			Expect(vpa.Exists()).To(BeTrue())
			Expect(vpa.Field("spec.targetRef.kind").String()).To(Equal("DaemonSet"))

			pod := ds.Field("spec.template")

			// `isEastWestGateway` is this label AND the waypoint node type.
			Expect(pod.Get(`metadata.labels.gateway\.istio\.io/managed`).String()).
				To(Equal("istio.io-eastwest-controller"))
			Expect(pod.Get(`metadata.labels.gateway\.networking\.k8s\.io/gateway-class-name`).String()).
				To(Equal("istio-east-west"))
			Expect(pod.Get(`metadata.labels.gateway\.networking\.k8s\.io/gateway-name`).String()).
				To(Equal("ambientgateway"))
			Expect(pod.Get(`metadata.labels.topology\.istio\.io/network`).String()).To(Equal(expectedNetworkName))
			Expect(pod.Get(`metadata.labels.istio\.io/dataplane-mode`).String()).To(Equal("none"))
			Expect(pod.Get(`metadata.labels.sidecar\.istio\.io/inject`).String()).To(Equal("false"))

			// Without istio.io/rev the tagWatcher hands this proxy to no revision.
			Expect(pod.Get(`metadata.annotations.istio\.io/rev`).String()).To(Equal("v1x29"))

			container := ds.Field("spec.template.spec.containers").Array()[0]
			Expect(container.Get("name").String()).To(Equal("istio-proxy"))

			args := []string{}
			for _, a := range container.Get("args").Array() {
				args = append(args, a.String())
			}
			// model.Waypoint is half of isEastWestGateway.
			Expect(args[0]).To(Equal("proxy"))
			Expect(args[1]).To(Equal("waypoint"))

			// The gateway's scope comes from these and the pod labels, nothing else.
			Expect(containerEnv(container, "ISTIO_META_NETWORK")).To(Equal(expectedNetworkName))
			Expect(containerEnv(container, "ISTIO_META_REQUESTED_NETWORK_VIEW")).To(Equal(expectedNetworkName))
			Expect(containerEnv(container, "ISTIO_META_CLUSTER_ID")).To(Equal("my-domain-199688871"))
			Expect(containerEnv(container, "CA_ADDR")).To(Equal("istiod-v1x29.d8-istio.svc:15012"))
			Expect(containerEnv(container, "TRUST_DOMAIN")).To(Equal("my.domain"))

			// These would size the Go runtime to node allocatable: no limits are set.
			for _, name := range []string{"ISTIO_CPU_LIMIT", "GOMAXPROCS", "GOMEMLIMIT"} {
				Expect(containerEnv(container, name)).To(BeEmpty(), "%s must not be set without resource limits", name)
			}

			ports := map[string]int64{}
			for _, port := range container.Get("ports").Array() {
				ports[port.Get("name").String()] = port.Get("containerPort").Int()
			}
			Expect(ports).To(HaveKeyWithValue("hbone", int64(15008)))

			// The mesh ConfigMap stands in for upstream's PROXY_CONFIG.
			mounts := map[string]string{}
			for _, mount := range container.Get("volumeMounts").Array() {
				mounts[mount.Get("mountPath").String()] = mount.Get("name").String()
			}
			Expect(mounts).To(HaveKeyWithValue("/etc/istio/config", "config-volume"))

			volumes := map[string]string{}
			for _, volume := range ds.Field("spec.template.spec.volumes").Array() {
				volumes[volume.Get("name").String()] = volume.Get("configMap.name").String()
			}
			Expect(volumes).To(HaveKeyWithValue("config-volume", "istio-v1x29"))
			Expect(volumes).To(HaveKeyWithValue("istiod-ca-cert", "istio-ca-root-cert"))
			Expect(volumes).To(HaveKey("istio-token"))
		})

		It("Publishes the ambient gateway through an inlet Service that is not a network gateway", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			svc := f.KubernetesResource("Service", "d8-istio", "ambientgateway")
			Expect(svc.Exists()).To(BeTrue())
			Expect(svc.Field("spec.type").String()).To(Equal("LoadBalancer"))
			Expect(svc.Field("spec.selector.app").String()).To(Equal("ambientgateway"))

			// The port name is the contract with the metadata exporter.
			ports := svc.Field("spec.ports").Array()
			Expect(ports).To(HaveLen(1))
			Expect(ports[0].Get("name").String()).To(Equal("hbone"))
			Expect(ports[0].Get("port").Int()).To(Equal(int64(15008)))

			// See ambientgateway/service.yaml: a labelled Service would fill the
			// bucket selectNetworkGateways prefers and strand the ambient path.
			Expect(svc.Field(`metadata.labels.topology\.istio\.io/network`).Exists()).To(BeFalse())
		})

		It("Describes each ambient peer with an istio-remote Gateway", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// Named after the IstioMulticluster CR, not the peer-published clusterID: the
			// CR name is a DNS-1123 subdomain the API server already validated, whereas
			// clusterID is whatever the peer chose to publish.
			gw := f.KubernetesResource("Gateway", "d8-istio", "ambientgateway-remote-neighbour-ambient-0")
			Expect(gw.Exists()).To(BeTrue())
			Expect(gw.Field("apiVersion").String()).To(Equal("gateway.networking.k8s.io/v1"))

			// Not a class istiod deploys anything for.
			Expect(gw.Field("spec.gatewayClassName").String()).To(Equal("istio-remote"))

			// The network the gateway *serves*, which is the peer's, not ours.
			Expect(gw.Field(`metadata.labels.topology\.istio\.io/network`).String()).To(Equal("network-neigh-ambient"))

			// Without it, status.addresses is never written and the ztunnel path sees
			// nothing while sidecars keep working.
			Expect(gw.Field(`metadata.labels.istio\.io/rev`).String()).To(Equal("v1x29"))

			// Otherwise kube.GatewaySA derives the identity from this object's own name.
			Expect(gw.Field(`metadata.annotations.gateway\.istio\.io/service-account`).String()).
				To(Equal("alliance-ambientgateway"))

			addresses := gw.Field("spec.addresses").Array()
			Expect(addresses).To(HaveLen(1))
			Expect(addresses[0].Get("type").String()).To(Equal("IPAddress"))
			Expect(addresses[0].Get("value").String()).To(Equal("1.1.1.2"))

			// HBONE only: an mTLS listener would describe the peer's sidecar gateway too.
			listeners := gw.Field("spec.listeners").Array()
			Expect(listeners).To(HaveLen(1))
			Expect(listeners[0].Get("protocol").String()).To(Equal("HBONE"))
			Expect(listeners[0].Get("port").Int()).To(Equal(int64(15008)))

			// A peer publishing no ambient gateway gets no object.
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ambientgateway-remote-neighbour-sidecar-only-0").Exists()).
				To(BeFalse())

			// enableIngressGateway is ignored: ambient multicluster does not support flat networks.
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ambientgateway-remote-neighbour-flat-0").Exists()).
				To(BeTrue())
		})

		// The address type is carried in the values, decided once by the merge hook: istiod
		// copies spec.addresses to status.addresses verbatim for this class, and the
		// network-gateway readers key off the type. A DNS name published as an IPAddress
		// would reach ztunnel as an address that resolves to nothing, and nothing would say
		// so. See TestAmbientGatewayEndpoints for where the type is derived.
		It("Declares a peer reached by name as a Hostname address", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			gw := f.KubernetesResource("Gateway", "d8-istio", "ambientgateway-remote-neighbour-hostname-0")
			Expect(gw.Exists()).To(BeTrue())

			addresses := gw.Field("spec.addresses").Array()
			Expect(addresses).To(HaveLen(1))
			Expect(addresses[0].Get("type").String()).To(Equal("Hostname"))
			Expect(addresses[0].Get("value").String()).To(Equal("ambient.lb.example.com"))

			// Everything else about the object is the same as an IP-addressed peer's -
			// ztunnel reaches both through the Workload istiod synthesizes from it, and
			// that Workload takes its identity from this annotation either way.
			Expect(gw.Field("spec.gatewayClassName").String()).To(Equal("istio-remote"))
			Expect(gw.Field(`metadata.annotations.gateway\.istio\.io/service-account`).String()).
				To(Equal("alliance-ambientgateway"))
			Expect(gw.Field(`metadata.labels.topology\.istio\.io/network`).String()).
				To(Equal("network-neigh-hostname"))
			Expect(gw.Field("spec.listeners.0.protocol").String()).To(Equal("HBONE"))
			Expect(gw.Field("spec.listeners.0.port").Int()).To(Equal(int64(15008)))
		})

		// One Gateway per address, because a Gateway carries a single address and istiod
		// needs every one of them to load-balance the cross-network hop.
		It("Gives a peer publishing several addresses one Gateway each", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			first := f.KubernetesResource("Gateway", "d8-istio", "ambientgateway-remote-neighbour-multi-address-0")
			Expect(first.Exists()).To(BeTrue())
			// An IPv6 literal reaches the Gateway verbatim, and so does its type: both the
			// address and the kind of address it is were settled on the way in, so the
			// template neither parses the value nor judges whether it is well formed.
			Expect(first.Field("spec.addresses.0.value").String()).To(Equal("2001:db8::1"))
			Expect(first.Field("spec.addresses.0.type").String()).To(Equal("IPAddress"))

			second := f.KubernetesResource("Gateway", "d8-istio", "ambientgateway-remote-neighbour-multi-address-1")
			Expect(second.Exists()).To(BeTrue())
			Expect(second.Field("spec.addresses.0.value").String()).To(Equal("4.4.4.2"))
		})

		// The index is appended to every name, including the first. Omitting it for index 0
		// makes `<peer>` collide with `<other peer>-<index>` whenever one peer's name is
		// another's plus a number - and the render reports nothing, so one peer is simply
		// left pointed at the other's address.
		It("Keeps a peer named after another peer's indexed name apart from it", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// Would have been `...-neighbour-multi-address-1` too, under the peer whose
			// name it extends.
			own := f.KubernetesResource("Gateway", "d8-istio", "ambientgateway-remote-neighbour-multi-address-1-0")
			Expect(own.Exists()).To(BeTrue())
			Expect(own.Field("spec.addresses.0.value").String()).To(Equal("5.5.5.2"))

			// ... and that name still belongs to the second address of the shorter-named
			// peer, which is the half that would have been silently overwritten.
			taken := f.KubernetesResource("Gateway", "d8-istio", "ambientgateway-remote-neighbour-multi-address-1")
			Expect(taken.Field("spec.addresses.0.value").String()).To(Equal("4.4.4.2"))
			Expect(taken.Field(`metadata.labels.topology\.istio\.io/network`).String()).
				To(Equal("network-neigh-multi-address"))
		})

		It("Enables multi-network in istiod, labels the namespace and tells ztunnel its network", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(istiodEnv(f, "v1x29", "AMBIENT_ENABLE_MULTI_NETWORK")).To(Equal("true"))

			// Outranks meshNetworks, and peers adopt it as soon as they see it, so it must
			// match what the metadata exporter publishes.
			ns := f.KubernetesGlobalResource("Namespace", "d8-istio")
			Expect(ns.Field(`metadata.labels.topology\.istio\.io/network`).String()).To(Equal(expectedNetworkName))

			ztunnel := f.KubernetesResource("DaemonSet", "d8-istio", "ztunnel")
			Expect(ztunnel.Exists()).To(BeTrue())
			Expect(containerEnv(ztunnel.Field("spec.template.spec.containers").Array()[0], "NETWORK")).
				To(Equal(expectedNetworkName))
		})

		It("Publishes the waypoint template through the injection ConfigMap", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			injector := f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x29")
			Expect(injector.Exists()).To(BeTrue())

			config := injector.Field("data.config").String()
			Expect(config).To(ContainSubstring("waypoint: |"))
			// The pod label that selects istiod's east-west code paths, and the
			// ControllerLabel comparison that gates ISTIO_META_REQUESTED_NETWORK_VIEW.
			// A hand-edit dropping either fails silently at runtime.
			Expect(config).To(ContainSubstring("gateway.istio.io/managed"))
			Expect(config).To(ContainSubstring(".ControllerLabel"))
			Expect(config).To(ContainSubstring("ISTIO_META_REQUESTED_NETWORK_VIEW"))

			values := injector.Field("data.values").String()
			for _, key := range []string{"waypoint", "affinity", "topologySpreadConstraints", "nodeSelector", "tolerations", "resources"} {
				Expect(values).To(ContainSubstring(key), "global.waypoint.%s missing from injection values", key)
			}

			Expect(values).To(ContainSubstring(`"seccompProfile": {"type": "RuntimeDefault"}`))
		})

		// Even though every peer has enableIngressGateway set.
		It("Runs no sidecar east-west path", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-ingressgateway").Exists()).To(BeFalse())
			// The Istio Gateway for the sidecar gateway's AUTO_PASSTHROUGH listener.
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ingressgateway").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-v1x29").
				Field("data.meshNetworks").String()).To(MatchYAML(`networks: {}`))

			// So the exporter publishes an empty ingressGateways list.
			Expect(exporterEnv(f, "INGRESS_GATEWAY_ENABLED")).To(BeEmpty())
		})

		// Everything that is not a sidecar data path is shared by both modes.
		It("Keeps the multicluster plumbing the ambient path relies on", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Secret", "d8-istio", "istio-remote-secret-neighbour-ambient").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "d8-istio", "api-proxy").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(exporterEnv(f, "MULTICLUSTER_ENABLED")).To(Equal("true"))
			Expect(exporterEnv(f, "AMBIENT_GATEWAY_ENABLED")).To(Equal("true"))

			meshConfig := f.KubernetesResource("ConfigMap", "d8-istio", "istio-v1x29").Field("data.mesh").String()
			Expect(meshConfig).To(ContainSubstring("caCertificates:"))
			Expect(meshConfig).To(ContainSubstring("---ROOT CA---"))
		})

		It("Scopes cross-cluster service visibility to istio.io/global", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			meshConfig := f.KubernetesResource("ConfigMap", "d8-istio", "istio-v1x29").Field("data.mesh").String()
			Expect(meshConfig).To(ContainSubstring("serviceScopeConfigs:"))
			Expect(meshConfig).To(ContainSubstring("istio.io/global"))
			Expect(meshConfig).To(ContainSubstring("scope: GLOBAL"))
		})
	})

	// `istioNetworkName` is `len(clusterDomain) + 19` at worst and nothing bounds
	// clusterDomain: ClusterConfiguration gives it no maxLength and the discovery values
	// schema only constrains its charset. Ambient multicluster is the first thing to put
	// the derived value in a label, on the d8-istio namespace, so it is the first thing
	// that breaks - and it breaks by making the namespace unapplicable, which takes the
	// whole module with it.
	Context("Ambient multicluster is enabled on a cluster with a long cluster domain", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("global.discovery.clusterDomain", "aaaaaaaaaa.bbbbbbbbbb.cccccccccc.dddddddddd.eee")
			f.HelmRender()
		})

		It("Refuses to render, naming the cluster domain rather than leaving it to the API server", func() {
			Expect(f.RenderError).To(HaveOccurred())
			Expect(f.RenderError.Error()).To(ContainSubstring("over the 63-character limit"))
			// The message has to name the cause, which is two derivations away from the
			// label that would be rejected.
			Expect(f.RenderError.Error()).To(ContainSubstring("global.discovery.clusterDomain"))
			Expect(f.RenderError.Error()).To(ContainSubstring("aaaaaaaaaa.bbbbbbbbbb.cccccccccc.dddddddddd.eee"))
		})
	})

	// The same cluster domain is fine with the feature off: there the network name is only
	// ever an env var or a JSON field, so failing the render would break an install that
	// works today.
	Context("A long cluster domain without ambient multicluster", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("istio.multicluster.mode", "Sidecar")
			f.ValuesSet("global.discovery.clusterDomain", "aaaaaaaaaa.bbbbbbbbbb.cccccccccc.dddddddddd.eee")
			f.HelmRender()
		})

		It("Renders", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("Namespace", "d8-istio").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("Namespace", "d8-istio").
				Field(`metadata.labels.topology\.istio\.io/network`).Exists()).To(BeFalse())
		})
	})

	Context("Ambient multicluster is enabled with the NodePort inlet", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("istio.alliance.ingressGateway.inlet", "NodePort")
			f.ValuesSet("istio.alliance.ingressGateway.nodePort.port", 30001)
			f.ValuesSet("istio.alliance.ambientGateway.inlet", "NodePort")
			f.ValuesSet("istio.alliance.ambientGateway.nodePort.port", 30002)
			f.HelmRender()
		})

		It("Gives the ambient east-west gateway its own node port", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// Only the ambient gateway is deployed in this mode.
			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())

			ambient := f.KubernetesResource("Service", "d8-istio", "ambientgateway")
			Expect(ambient.Field("spec.type").String()).To(Equal("NodePort"))
			ports := ambient.Field("spec.ports").Array()
			Expect(ports).To(HaveLen(1))
			Expect(ports[0].Get("name").String()).To(Equal("hbone"))
			Expect(ports[0].Get("port").Int()).To(Equal(int64(15008)))
			Expect(ports[0].Get("nodePort").Int()).To(Equal(int64(30002)))

			// A DaemonSet, so every node the pods land on can serve the node port.
			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ambientgateway").Exists()).To(BeTrue())

			// The exporter needs the ambient inlet separately: it reads the address and
			// port peers dial off a different Service, which may be exposed differently.
			Expect(exporterEnv(f, "INLET")).To(Equal("NodePort"))
			Expect(exporterEnv(f, "AMBIENT_INLET")).To(Equal("NodePort"))
		})
	})

	Context("Ambient multicluster is enabled with a different inlet set for each gateway", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("istio.alliance.ingressGateway.inlet", "NodePort")
			f.ValuesSet("istio.alliance.ingressGateway.nodePort.port", 30001)
			f.ValuesSet("istio.alliance.ambientGateway.inlet", "LoadBalancer")
			f.HelmRender()
		})

		// The ambient gateway follows its own inlet, never the sidecar gateway's.
		It("Exposes the ambient gateway on a load balancer", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "ambientgateway").
				Field("spec.type").String()).To(Equal("LoadBalancer"))

			Expect(exporterEnv(f, "INLET")).To(Equal("NodePort"))
			Expect(exporterEnv(f, "AMBIENT_INLET")).To(Equal("LoadBalancer"))
		})
	})

	Context("Ambient multicluster is enabled with the same node port set for both east-west gateways", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("istio.alliance.ingressGateway.inlet", "NodePort")
			f.ValuesSet("istio.alliance.ingressGateway.nodePort.port", 30001)
			f.ValuesSet("istio.alliance.ambientGateway.inlet", "NodePort")
			f.ValuesSet("istio.alliance.ambientGateway.nodePort.port", 30001)
			f.HelmRender()
		})

		// The node port conflict check only applies when both gateways are deployed, which no
		// mode does yet.
		It("Renders, because the sidecar gateway that would claim the port is not deployed", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "ambientgateway").
				Field("spec.ports").Array()[0].Get("nodePort").Int()).To(Equal(int64(30001)))
		})
	})

	Context("Ambient multicluster is enabled with one gateway on a node port and the other on an LB", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("istio.alliance.ingressGateway.inlet", "LoadBalancer")
			f.ValuesSet("istio.alliance.ingressGateway.nodePort.port", 30001)
			f.ValuesSet("istio.alliance.ambientGateway.inlet", "NodePort")
			f.ValuesSet("istio.alliance.ambientGateway.nodePort.port", 30001)
			f.HelmRender()
		})

		// The conflict check must look at the inlet, not just the number: a nodePort.port
		// left over from an earlier NodePort inlet claims nothing.
		It("Renders, because only one of them actually claims the node port", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Service", "d8-istio", "ambientgateway").
				Field("spec.ports").Array()[0].Get("nodePort").Int()).To(Equal(int64(30001)))
		})
	})

	Context("Ambient multicluster is enabled with per-gateway pod annotations and placement", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSetFromYaml("istio.alliance.ingressGateway.gatewayPodAnnotations", `
checksum/config: sidecar-123
`)
			f.ValuesSetFromYaml("istio.alliance.ingressGateway.nodeSelector", `
node-role.deckhouse.io/istio-gateway: ""
`)
			f.ValuesSetFromYaml("istio.alliance.ambientGateway.gatewayPodAnnotations", `
checksum/config: ambient-123
istio.io/rev: someone-elses-revision
cluster-autoscaler.kubernetes.io/enable-ds-eviction: "true"
`)
			f.ValuesSetFromYaml("istio.alliance.ambientGateway.nodeSelector", `
node-role.deckhouse.io/istio-ambient-gateway: ""
`)
			f.ValuesSetFromYaml("istio.alliance.ambientGateway.tolerations", `
- operator: Exists
`)
			f.HelmRender()
		})

		// The ambient gateway reads its own block and nothing of the sidecar gateway's.
		It("Keeps the ambient gateway's annotations and placement to itself", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeFalse())

			ambient := f.KubernetesResource("DaemonSet", "d8-istio", "ambientgateway").Field("spec.template")
			Expect(ambient.Get(`metadata.annotations.checksum/config`).String()).To(Equal("ambient-123"))
			Expect(ambient.Get(`spec.nodeSelector.node-role\.deckhouse\.io/istio-ambient-gateway`).Exists()).To(BeTrue())
			Expect(ambient.Get(`spec.nodeSelector.node-role\.deckhouse\.io/istio-gateway`).Exists()).To(BeFalse())
			Expect(ambient.Get("spec.tolerations").Array()).To(HaveLen(1))

			// The module's own keys win over operator-supplied ones of the same name,
			// and the map is merged rather than appended so a repeated key renders once
			// instead of twice.
			//
			// enable-ds-eviction is the one that matters: it is what stops the cluster
			// autoscaler evicting this pod during a scale-down, and under the NodePort
			// inlet a single node can be carrying every cross-cluster ambient
			// connection. istio.io/rev is record-keeping - nothing reads the pod
			// annotation back - but it must not render twice either.
			Expect(ambient.Get(`metadata.annotations.cluster-autoscaler\.kubernetes\.io/enable-ds-eviction`).
				String()).To(Equal("false"))
			Expect(ambient.Get(`metadata.annotations.istio\.io/rev`).String()).To(Equal("v1x29"))

			// Nothing the operator did not ask for was dropped on the way through.
			Expect(ambient.Get(`metadata.annotations.checksum/proxy-bootstrap-config`).String()).ToNot(BeEmpty())
			Expect(ambient.Get("metadata.annotations").Map()).To(HaveLen(4))
		})
	})

	// pilot-agent reads /etc/istio/config/mesh once at startup, so a DaemonSet that
	// nothing else rolls has to roll itself when that half of the ConfigMap changes -
	// and only when it does. The hash is deliberately narrower than the ConfigMap.
	Context("Ambient multicluster is enabled and the mesh ConfigMap changes", func() {
		bootstrapChecksum := func(mutate func()) string {
			setUpAmbientMulticluster(f)
			f.ValuesSetFromYaml("istio.internal.multiclusters", ambientMulticlusters)
			mutate()
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			return f.KubernetesResource("DaemonSet", "d8-istio", "ambientgateway").
				Field(`spec.template.metadata.annotations.checksum/proxy-bootstrap-config`).String()
		}

		It("Rolls the gateway when a value it can only read at startup changes", func() {
			base := bootstrapChecksum(func() {})
			Expect(base).ToNot(BeEmpty())

			// holdApplicationUntilProxyStarts and idleTimeout both land in defaultConfig.
			changed := bootstrapChecksum(func() {
				f.ValuesSet("istio.dataPlane.proxyConfig.idleTimeout", "42m")
			})
			Expect(changed).ToNot(Equal(base))
		})

		It("Leaves them alone when only what reaches the proxy dynamically changes", func() {
			meshData := func() string {
				return f.KubernetesResource("ConfigMap", "d8-istio", "istio-v1x29").Field("data.mesh").String()
			}

			base := bootstrapChecksum(func() {})
			Expect(base).ToNot(BeEmpty())
			baseMesh := meshData()

			// caCertificates is rewritten on every peer addition and CA rotation, and
			// reaches the agent over xDS through PROXY_CONFIG_XDS_AGENT. Hashing the
			// whole ConfigMap would roll every gateway pod on peer churn for nothing.
			fewerPeers := bootstrapChecksum(func() {
				f.ValuesSetFromYaml("istio.internal.multiclusters", `[]`)
			})
			Expect(fewerPeers).ToNot(BeEmpty())
			// Without this the assertion below would hold for a ConfigMap that never
			// changed, and the test would pass while proving nothing.
			Expect(meshData()).ToNot(Equal(baseMesh), "dropping the peers must change the ConfigMap")

			Expect(fewerPeers).To(Equal(base))
		})
	})

	Context("Ambient multicluster is enabled with per-gateway Service settings", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSetFromYaml("istio.alliance.ingressGateway.serviceAnnotations", `
yandex.cpi.flant.com/listener-subnet-id: sidecar-123
`)
			f.ValuesSet("istio.alliance.ingressGateway.loadBalancerClass", "sidecar-class")
			f.ValuesSetFromYaml("istio.alliance.ambientGateway.serviceAnnotations", `
yandex.cpi.flant.com/listener-subnet-id: ambient-123
`)
			f.ValuesSet("istio.alliance.ambientGateway.loadBalancerClass", "ambient-class")
			f.HelmRender()
		})

		// An annotation or class that pins one external resource - a static IP, an
		// address pool, an existing load balancer's name - cannot be honoured by two
		// Services at once, so the sidecar gateway's are never copied across.
		It("Gives the ambient Service its own annotations and load balancer class", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())

			ambient := f.KubernetesResource("Service", "d8-istio", "ambientgateway")
			Expect(ambient.Field(`metadata.annotations.yandex\.cpi\.flant\.com/listener-subnet-id`).
				String()).To(Equal("ambient-123"))
			Expect(ambient.Field("spec.loadBalancerClass").String()).To(Equal("ambient-class"))
		})
	})

	Context("Ambient multicluster is enabled with an advertise list per gateway", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSetFromYaml("istio.alliance.ingressGateway.advertise", `
- address: istio-ew.example.com
  port: 15443
`)
			f.ValuesSetFromYaml("istio.alliance.ambientGateway.advertise", `
- address: 172.16.0.6
  port: 15008
`)
			f.HelmRender()
		})

		// A ConfigMap per gateway, so the exporter can tell the lists apart. The sidecar list
		// is not published in this mode.
		It("Publishes the ambient list to the exporter, and only it", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "metadata-exporter-ingressgateway-advertise").
				Exists()).To(BeFalse())

			ambient := f.KubernetesResource("ConfigMap", "d8-istio", "metadata-exporter-ambientgateway-advertise")
			Expect(ambient.Exists()).To(BeTrue())
			Expect(ambient.Field(`data.gateways\.json`).String()).
				To(MatchJSON(`[{"address": "172.16.0.6", "port": 15008}]`))
		})

		// An exporter from the previous release reads the ConfigMap this release rewrote,
		// for as long as its own rollout takes: its pod has no checksum annotation on these
		// ConfigMaps, and Helm applies them before the Deployment. Finding no key it knows,
		// it would publish an empty gateway list to every peer.
		It("Keeps writing the key the previous release's exporter reads", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-istio", "metadata-exporter-ambientgateway-advertise")
			Expect(cm.Field(`data.ingressgateways-array\.json`).String()).
				To(MatchJSON(`[{"address": "172.16.0.6", "port": 15008}]`))
		})
	})

	Context("Ambient multicluster is enabled alongside an additional pre-1.29 revision", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.29", "1.27"]`)
			f.HelmRender()
		})

		It("Enables multi-network only for the revision that ships a waypoint template", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// supportsAmbient is true from 1.25, but no revision below 1.29 ships a
			// waypoint template, so v1x27 must not get the flag.
			Expect(istiodEnv(f, "v1x29", "AMBIENT_ENABLE_MULTI_NETWORK")).To(Equal("true"))
			Expect(istiodEnv(f, "v1x27", "AMBIENT_ENABLE_MULTI_NETWORK")).To(BeEmpty())

			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-v1x29").
				Field("data.mesh").String()).To(ContainSubstring("serviceScopeConfigs:"))
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-v1x27").
				Field("data.mesh").String()).ToNot(ContainSubstring("serviceScopeConfigs"))

			// The key follows the shipped template file, not the feature switch.
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x29").
				Field("data.config").String()).To(ContainSubstring("waypoint: |"))
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x27").
				Field("data.config").String()).ToNot(ContainSubstring("waypoint: |"))

			// The gateway pod is claimed by the global revision alone.
			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ambientgateway").
				Field(`spec.template.metadata.annotations.istio\.io/rev`).String()).To(Equal("v1x29"))
		})
	})

	Context("Ambient multicluster is enabled and no peer asks for a sidecar gateway", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("istio.internal.multiclustersNeedIngressGateway", false)
			f.HelmRender()
		})

		It("Still renders the ambient east-west gateway", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// NOT derived from multiclustersNeedIngressGateway, which is false before the
			// first IstioMulticluster exists: that would leave AMBIENT_ENABLE_MULTI_NETWORK
			// on with no gateway for this network.
			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ambientgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Service", "d8-istio", "ambientgateway").Exists()).To(BeTrue())
			Expect(istiodEnv(f, "v1x29", "AMBIENT_ENABLE_MULTI_NETWORK")).To(Equal("true"))

			// The sidecar east-west gateway stays derived, and is not needed here.
			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
		})
	})

	// The global revision does support ambient multicluster here, so the only thing
	// keeping the feature off is multicluster.mode.
	Context("Ambient and multicluster are enabled, with multicluster in the Sidecar mode", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("istio.multicluster.mode", "Sidecar")
			f.HelmRender()
		})

		It("Renders no ambient east-west gateway", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ambientgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "ambientgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-ambientgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ambientgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-ambientgateway").Exists()).To(BeFalse())

			// The sidecar east-west gateway is the Sidecar mode's own.
			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeTrue())
			Expect(exporterEnv(f, "INGRESS_GATEWAY_ENABLED")).To(Equal("true"))

			// Deployment is signalled separately, so the inlet is always set.
			Expect(exporterEnv(f, "AMBIENT_GATEWAY_ENABLED")).To(BeEmpty())
			Expect(exporterEnv(f, "AMBIENT_INLET")).To(Equal("LoadBalancer"))

			// It must not be labelled as a network gateway: that would break the ambient path
			// once both paths run side by side. Same as for the ambient gateway's Service.
			svc := f.KubernetesResource("Service", "d8-istio", "ingressgateway")
			Expect(svc.Exists()).To(BeTrue())
			Expect(svc.Field(`metadata.labels.topology\.istio\.io/network`).Exists()).To(BeFalse())
			Expect(svc.Field(`metadata.labels.networking\.istio\.io/gatewayPort`).Exists()).To(BeFalse())

			// AMBIENT_ENABLE_MULTI_NETWORK in particular would change sidecar behaviour: it
			// removes istiod's "no gateway means directly reachable" fallback.
			Expect(istiodEnv(f, "v1x29", "AMBIENT_ENABLE_MULTI_NETWORK")).To(BeEmpty())
			Expect(f.KubernetesGlobalResource("Namespace", "d8-istio").
				Field(`metadata.labels.topology\.istio\.io/network`).Exists()).To(BeFalse())

			ztunnel := f.KubernetesResource("DaemonSet", "d8-istio", "ztunnel")
			Expect(ztunnel.Exists()).To(BeTrue())
			Expect(containerEnv(ztunnel.Field("spec.template.spec.containers").Array()[0], "NETWORK")).To(BeEmpty())

			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-v1x29").
				Field("data.mesh").String()).ToNot(ContainSubstring("serviceScopeConfigs"))
		})
	})

	// A flat network: the sidecar east-west gateway is not deployed, so the exporter must not
	// look for its Service.
	Context("Multicluster in the Sidecar mode with no peer needing the east-west gateway", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("istio.multicluster.mode", "Sidecar")
			f.ValuesSet("istio.internal.multiclustersNeedIngressGateway", false)
			f.HelmRender()
		})

		It("Tells the exporter the sidecar east-west gateway is not deployed", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(exporterEnv(f, "INGRESS_GATEWAY_ENABLED")).To(BeEmpty())
		})
	})

	// The `waypoint` injection key is deliberately not gated on the feature switch, or on
	// ambient at all: with it present a user can create an `istio-waypoint` Gateway and
	// have istiod deploy it, which is upstream's own waypoint mechanism offered alongside
	Context("Neither ambient nor multicluster is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.29")
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.29"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `[]`)
			f.HelmRender()
		})

		It("Still publishes the waypoint template, and nothing else from the feature", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x29").
				Field("data.config").String()).To(ContainSubstring("waypoint: |"))

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ambientgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "ambientgateway").Exists()).To(BeFalse())
			Expect(istiodEnv(f, "v1x29", "AMBIENT_ENABLE_MULTI_NETWORK")).To(BeEmpty())
		})
	})

	// The Ambient mode turns the sidecar path off, so a silent fallback on an unsupported
	// version would leave the multicluster with no data path.
	Context("Ambient multicluster is enabled on a global version that does not support it", func() {
		BeforeEach(func() {
			setUpAmbientMulticluster(f)
			f.ValuesSet("istio.internal.globalVersion", "1.27")
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.27"]`)
			f.HelmRender()
		})

		It("Refuses to render, naming the version and the way out", func() {
			Expect(f.RenderError).To(HaveOccurred())
			Expect(f.RenderError.Error()).To(ContainSubstring("does not support ambient multicluster"))
			Expect(f.RenderError.Error()).To(ContainSubstring("1.27"))
			Expect(f.RenderError.Error()).To(ContainSubstring("multicluster.mode to Sidecar"))
		})
	})
})
