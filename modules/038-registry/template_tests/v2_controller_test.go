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
	"encoding/base64"
	"strings"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// The discriminator is what keeps the two implementations from ever owning the
// image pull path at the same time, and it does so by deciding what is rendered
// at all. So both sides of it are asserted here: with it off, not a single object
// of the new implementation may appear.

const v2Disabled = `
internal:
  orchestrator:
    hash: "123"
    state:
      mode: "Unmanaged"
      target_mode: "Unmanaged"
      bashible:
        config:
          mode: Unmanaged
          version: "legacy1"
          imagesBase: registry.example.com/deckhouse/ee
          hosts:
            registry.example.com:
              mirrors:
                - host: registry.example.com
                  scheme: https
  v2:
    enabled: false
`

// A cluster upgraded to a release carrying the new schema can have `internal`
// populated by the old hooks with no `v2` branch at all. Templates must tolerate
// that instead of failing to render the whole module.
const v2Absent = `
internal:
  orchestrator:
    hash: "123"
    state:
      mode: "Unmanaged"
      target_mode: "Unmanaged"
`

const v2Enabled = `
internal:
  orchestrator: {}
  v2:
    enabled: true
    bashibleConfig:
      agent:
        endpoint: 127.0.0.1:5001
        layout: '{"backends":[{"name":"Upstream","host":"registry.deckhouse.io"}]}'
      mode: Managed
      version: "abc123"
      imagesBase: registry.d8-system.svc:5001/system/deckhouse
      proxyEndpoints: []
      hosts:
        registry.d8-system.svc:5001:
          mirrors:
            - host: registry.d8-system.svc:5001
              scheme: https
    registryConfig:
      mode: Managed
      primary:
        upstream:
          scheme: HTTPS
          host: registry.deckhouse.io
          path: /deckhouse/ee
      storage:
        cache: true
        size: 50Gi
        garbageCollection:
          enabled: true
          schedule: "17 3 * * *"
    storageAddresses: ["10.0.0.1", "10.0.0.2"]
    pki:
      ca:
        cert: CA-CERT
        key: CA-KEY
      distribution:
        cert: DISTRIBUTION-CERT
        key: DISTRIBUTION-KEY
      auth:
        cert: AUTH-CERT
        key: AUTH-KEY
      token:
        cert: TOKEN-CERT
        key: TOKEN-KEY
      ingressClient:
        cert: INGRESS-CLIENT-CERT
        key: INGRESS-CLIENT-KEY
      httpSecret: HTTP-SECRET
      ro:
        name: registry-ro
        password: RO-PASSWORD
        passwordHash: RO-HASH
      rw:
        name: registry-rw
        password: RW-PASSWORD
        passwordHash: RW-HASH
`

// The state the switch gate is supposed to make unreachable: the new implementation
// active while the legacy state machine still wants its own Service and publication
// endpoint. Rendered on purpose, because two definitions of one object are resolved by
// template ordering rather than by intent — and `Service/registry` is the name every image
// reference in the cluster is built from.
const v2WithLegacyStillActive = `
internal:
  orchestrator:
    hash: "123"
    state:
      mode: "Proxy"
      target_mode: "Proxy"
      conditions: []
      registry_service: "node-services"
      ingress_enabled: true
      bashible:
        config:
          mode: Proxy
          version: "legacy1"
          imagesBase: registry.d8-system.svc:5001/system/deckhouse
          hosts:
            registry.d8-system.svc:5001:
              mirrors:
                - host: 10.0.0.1:5001
                  scheme: https
  v2:
    enabled: true
    bashibleConfig:
      agent:
        endpoint: 127.0.0.1:5001
        dropInFile: /etc/containerd/registry.d/_default/hosts.toml
        layout: '{"backends":[{"name":"Upstream","host":"registry.deckhouse.io"}]}'
      mode: Managed
      version: "abc123"
      imagesBase: registry.d8-system.svc:5001/system/deckhouse
      proxyEndpoints: []
      hosts:
        registry.d8-system.svc:5001:
          mirrors:
            - host: registry.d8-system.svc:5001
              scheme: https
    registryConfig:
      mode: Managed
      storage:
        cache: true
        size: 50Gi
    storageAddresses: ["10.0.0.1", "10.0.0.2"]
    pki:
      ca:
        cert: CA-CERT
        key: CA-KEY
      distribution:
        cert: DISTRIBUTION-CERT
        key: DISTRIBUTION-KEY
      auth:
        cert: AUTH-CERT
        key: AUTH-KEY
      token:
        cert: TOKEN-CERT
        key: TOKEN-KEY
      ingressClient:
        cert: INGRESS-CLIENT-CERT
        key: INGRESS-CLIENT-KEY
      httpSecret: HTTP-SECRET
      ro:
        name: registry-ro
        password: RO-PASSWORD
        passwordHash: RO-HASH
      rw:
        name: registry-rw
        password: RW-PASSWORD
        passwordHash: RW-HASH
`

const v2AirGap = `
whitelistSourceRanges: ["10.0.0.0/10", "192.168.0.0/16"]
internal:
  orchestrator: {}
  v2:
    enabled: true
    registryConfig:
      mode: Managed
      storage:
        cache: true
        size: 50Gi
    storageAddresses: ["10.0.0.1", "10.0.0.2"]
    pki:
      ca:
        cert: CA-CERT
        key: CA-KEY
      distribution:
        cert: DISTRIBUTION-CERT
        key: DISTRIBUTION-KEY
      auth:
        cert: AUTH-CERT
        key: AUTH-KEY
      token:
        cert: TOKEN-CERT
        key: TOKEN-KEY
      ingressClient:
        cert: INGRESS-CLIENT-CERT
        key: INGRESS-CLIENT-KEY
      httpSecret: HTTP-SECRET
      ro:
        name: registry-ro
        password: RO-PASSWORD
        passwordHash: RO-HASH
      rw:
        name: registry-rw
        password: RW-PASSWORD
        passwordHash: RW-HASH
`

// The module at its default. `enabled` is the switch discriminator — this implementation
// owns the cluster rather than the previous one — and it says nothing about the module
// having been asked to manage anything. The hook still generates the PKI and publishes a
// resolved config, so these values are what a brand-new cluster actually has.
const v2EnabledUnmanaged = `
internal:
  orchestrator: {}
  v2:
    enabled: true
    registryConfig:
      mode: Unmanaged
    pki:
      ca:
        cert: CA-CERT
        key: CA-KEY
      distribution:
        cert: DISTRIBUTION-CERT
        key: DISTRIBUTION-KEY
      auth:
        cert: AUTH-CERT
        key: AUTH-KEY
      token:
        cert: TOKEN-CERT
        key: TOKEN-KEY
      ingressClient:
        cert: INGRESS-CLIENT-CERT
        key: INGRESS-CLIENT-KEY
      httpSecret: HTTP-SECRET
      ro:
        name: registry-ro
        password: RO-PASSWORD
        passwordHash: RO-HASH
      rw:
        name: registry-rw
        password: RW-PASSWORD
        passwordHash: RW-HASH
`

// Managed with the cache turned off: the node agent owns the pull path, and nothing is
// cached. The storage must not appear anywhere.
const v2ManagedNoCache = `
internal:
  orchestrator: {}
  v2:
    enabled: true
    registryConfig:
      mode: Managed
      primary:
        upstream:
          scheme: HTTPS
          host: registry.deckhouse.io
          path: /deckhouse/ee
      storage:
        cache: false
    pki:
      ca:
        cert: CA-CERT
        key: CA-KEY
      distribution:
        cert: DISTRIBUTION-CERT
        key: DISTRIBUTION-KEY
      auth:
        cert: AUTH-CERT
        key: AUTH-KEY
      token:
        cert: TOKEN-CERT
        key: TOKEN-KEY
      ingressClient:
        cert: INGRESS-CLIENT-CERT
        key: INGRESS-CLIENT-KEY
      httpSecret: HTTP-SECRET
      ro:
        name: registry-ro
        password: RO-PASSWORD
        passwordHash: RO-HASH
      rw:
        name: registry-rw
        password: RW-PASSWORD
        passwordHash: RW-HASH
`

// A cluster on its first converge: this module has weight 38 and cert-manager 101, so
// cert-manager is not enabled yet and the https mode is the default CertManager. The
// ingress helpers refuse that combination, and they refuse it by failing the render.
const globalValuesBeforeCertManager = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler", "registry"]
modules:
  https:
    mode: CertManager
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`

var _ = Describe("Module :: registry :: helm template :: v2 controller", func() {
	f := SetupHelmConfig(``)

	renderWith := func(registryValues string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryValues)
		f.HelmRender()
	}

	Context("discriminator off", func() {
		BeforeEach(func() { renderWith(v2Disabled) })

		It("renders nothing of the new implementation", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Deployment", "d8-system", "registry-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-system", "registry-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:registry:controller").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:registry:agent").Exists()).To(BeFalse())
		})
	})

	Context("v2 branch absent from internal values", func() {
		BeforeEach(func() { renderWith(v2Absent) })

		It("renders without error and without the new implementation", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Deployment", "d8-system", "registry-controller").Exists()).To(BeFalse())
		})
	})

	Context("discriminator on", func() {
		BeforeEach(func() { renderWith(v2Enabled) })

		It("renders the controller Deployment on the master nodes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-system", "registry-controller")
			Expect(deployment.Exists()).To(BeTrue())
			Expect(deployment.Field("spec.template.spec.serviceAccountName").String()).To(Equal("registry-controller"))
			// The storage lives on the master nodes, and so does the controller that
			// drives it.
			Expect(deployment.Field("spec.template.spec.nodeSelector").String()).
				To(ContainSubstring("node-role.deckhouse.io/control-plane"))

			container := deployment.Field("spec.template.spec.containers.0")
			Expect(container.Get("name").String()).To(Equal("registry-controller"))
			Expect(container.Get("securityContext.readOnlyRootFilesystem").Bool()).To(BeTrue())
			// POD_NAMESPACE scopes the leader election lease; without it the manager
			// would try to elect in the wrong namespace.
			Expect(container.Get("env.0.name").String()).To(Equal("POD_NAMESPACE"))
		})

		It("grants the controller its own resources and the nodes theirs", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ServiceAccount", "d8-system", "registry-controller").Exists()).To(BeTrue())

			controllerRole := f.KubernetesGlobalResource("ClusterRole", "d8:registry:controller")
			Expect(controllerRole.Exists()).To(BeTrue())
			// The controller may only write the status of the objects other writers
			// own, never their spec.
			Expect(controllerRole.Field("rules").String()).To(ContainSubstring("registryconfigs/status"))
			Expect(controllerRole.Field("rules").String()).To(ContainSubstring("registryupstreams/status"))

			// The agent authenticates with the kubelet's kubeconfig, so the binding
			// is to the system:nodes group rather than to a ServiceAccount.
			agentBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:registry:agent")
			Expect(agentBinding.Exists()).To(BeTrue())
			Expect(agentBinding.Field("subjects").String()).To(ContainSubstring("system:nodes"))

			// Leader election is namespaced, so it must be a Role and not part of the
			// ClusterRole.
			Expect(f.KubernetesResource("Role", "d8-system", "registry:controller").
				Field("rules").String()).To(ContainSubstring("leases"))
			Expect(controllerRole.Field("rules").String()).ShouldNot(ContainSubstring("leases"))
		})
	})
})

var _ = Describe("Module :: registry :: helm template :: v2 storage", func() {
	f := SetupHelmConfig(``)

	renderWith := func(registryValues string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryValues)
		f.HelmRender()
	}

	Context("discriminator off", func() {
		BeforeEach(func() { renderWith(v2Disabled) })

		It("renders no storage at all", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("StatefulSet", "d8-system", "registry-storage").Exists()).To(BeFalse())
			// The endpoint every image reference in the cluster is built from must not
			// appear while the legacy implementation still owns it.
			Expect(f.KubernetesResource("Service", "d8-system", "registry").Exists()).To(BeFalse())
		})
	})

	Context("discriminator on", func() {
		BeforeEach(func() { renderWith(v2Enabled) })

		It("renders the storage on the master nodes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			set := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
			Expect(set.Exists()).To(BeTrue())
			Expect(set.Field("spec.serviceName").String()).To(Equal("registry"))
			Expect(set.Field("spec.template.spec.nodeSelector").String()).
				To(ContainSubstring("node-role.deckhouse.io/control-plane"))

			// The syncer signals the registry process to make it re-read its
			// configuration, which needs a shared process namespace. Without this the
			// whole "reconfigure while the operator is down" path does not work.
			Expect(set.Field("spec.template.spec.shareProcessNamespace").Bool()).To(BeTrue())

			containers := set.Field("spec.template.spec.containers").Array()
			names := []string{}
			for _, container := range containers {
				names = append(names, container.Get("name").String())
			}
			Expect(names).To(ConsistOf("distribution", "auth", "kube-rbac-proxy", "syncer"))

			// One replica per node, since each owns the store on its own host.
			Expect(set.Field("spec.template.spec.affinity.podAntiAffinity").String()).
				To(ContainSubstring("kubernetes.io/hostname"))

			// The registry binds the node address while the node agent binds the
			// loopback one — the same port on different interfaces. Without the host
			// network namespace the node address does not exist for the container, and a
			// host port would take 5001 away from the agent.
			Expect(set.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(set.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(containers[0].Get("ports.0.hostPort").Exists()).To(BeFalse())
		})

		It("keeps the blobs on the host at the fixed path", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			set := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
			// Not a persistent volume claim: the storage has to come up before any
			// storage provisioner, whose own images come from here.
			Expect(set.Field("spec.template.spec.volumes").String()).
				To(ContainSubstring("/opt/deckhouse/registry/local_data"))
		})

		It("publishes the endpoint the whole cluster pulls from", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			service := f.KubernetesResource("Service", "d8-system", "registry")
			Expect(service.Exists()).To(BeTrue())
			Expect(service.Field("spec.ports.0.port").String()).To(Equal("5001"))
		})

		It("lets a syncer write only its own report and take the lease", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ServiceAccount", "d8-system", "registry-storage").Exists()).To(BeTrue())

			role := f.KubernetesGlobalResource("ClusterRole", "d8:registry:storage")
			Expect(role.Exists()).To(BeTrue())
			// What the storage should do is the controller's decision; a syncer only
			// reports what it holds.
			Expect(role.Field("rules").String()).To(ContainSubstring("registrystorages/status"))

			Expect(f.KubernetesResource("Role", "d8-system", "registry:storage").
				Field("rules").String()).To(ContainSubstring("leases"))
		})
	})
})

var _ = Describe("Module :: registry :: helm template :: v2 storage PKI", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
	})

	It("keeps the signing key away from the storage", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		secret := f.KubernetesResource("Secret", "d8-system", "registry-storage-pki")
		Expect(secret.Exists()).To(BeTrue())

		data := secret.Field("data").Map()
		Expect(data).To(HaveKey("token.crt"))
		// The storage only verifies tokens. Handing it the signing key would let a
		// compromised storage mint its own.
		Expect(data).ShouldNot(HaveKey("token.key"))
	})

	It("keeps the write credentials out of the read secret", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		access := f.KubernetesResource("Secret", "d8-system", "registry-storage-access")
		Expect(access.Exists()).To(BeTrue())
		Expect(access.Field("data.username").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("registry-ro"))))
		// No private key at all: the controller reads this to build the node layouts,
		// and it must not become a way to obtain the storage's identity.
		Expect(access.Field("data").Map()).ShouldNot(HaveKey("distribution.key"))

		push := f.KubernetesResource("Secret", "d8-system", "registry-storage-push")
		Expect(push.Exists()).To(BeTrue())
		Expect(push.Field("data.password").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("RW-PASSWORD"))))
		// Granting the ability to read the pull credentials must not grant the ability
		// to replace images.
		Expect(access.Field("data.password").String()).ShouldNot(Equal(push.Field("data.password").String()))
	})

	It("persists the whole state so a restart does not reissue the authority", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		secret := f.KubernetesResource("Secret", "d8-system", "registry-storage-pki")
		state, err := base64.StdEncoding.DecodeString(secret.Field(`data.state\.yaml`).String())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(state)).To(ContainSubstring("CA-KEY"))
	})
})

// The publication endpoint is the only way into the storage from outside the
// cluster, and the only path that can replace an image. So both sides are asserted:
// it must appear where pushing is the only way images get in, and must not appear
// where it buys nothing.
var _ = Describe("Module :: registry :: helm template :: v2 publication endpoint", func() {
	f := SetupHelmConfig(``)

	renderWith := func(registryValues string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryValues)
		f.HelmRender()
	}

	Context("a pass-through cache", func() {
		BeforeEach(func() { renderWith(v2Enabled) })

		It("is not published", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// There is an upstream to pull from, so an internet-facing endpoint that can
			// replace an image would be surface for nothing.
			Expect(f.KubernetesResource("Ingress", "d8-system", "registry-push").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-system", "registry-push").Exists()).To(BeFalse())
		})
	})

	Context("an air-gapped cache", func() {
		BeforeEach(func() { renderWith(v2AirGap) })

		It("is published, since pushing is the only way images get in", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ingress := f.KubernetesResource("Ingress", "d8-system", "registry-push")
			Expect(ingress.Exists()).To(BeTrue())

			annotations := ingress.Field("metadata.annotations").Map()
			// A client certificate is required on this path, so a leaked password alone
			// cannot be used to replace an image.
			Expect(annotations).To(HaveKey("nginx.ingress.kubernetes.io/proxy-ssl-secret"))
			Expect(annotations["nginx.ingress.kubernetes.io/proxy-ssl-secret"].String()).
				To(Equal("d8-system/registry-storage-ingress-client"))
			// An image layer has no meaningful size limit.
			Expect(annotations["nginx.ingress.kubernetes.io/proxy-body-size"].String()).To(Equal("0"))
			// A push is a sequence of requests that has to reach the same replica.
			Expect(annotations).To(HaveKey("nginx.ingress.kubernetes.io/upstream-hash-by"))

			// A routable service, unlike the headless one the cluster pulls through.
			service := f.KubernetesResource("Service", "d8-system", "registry-push")
			Expect(service.Exists()).To(BeTrue())
			Expect(service.Field("spec.clusterIP").Exists()).To(BeFalse())
		})

		It("honours the publication settings that predate the new model", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ingress := f.KubernetesResource("Ingress", "d8-system", "registry-push")
			Expect(ingress.Field(`metadata.annotations.nginx\.ingress\.kubernetes\.io/whitelist-source-range`).String()).
				To(Equal("10.0.0.0/10,192.168.0.0/16"))
		})
	})
})

// The node-facing secret is where the two implementations would collide most directly:
// its content decides which writer owns /etc/containerd/registry.d on every node. Only
// one of them may ever author it.
var _ = Describe("Module :: registry :: helm template :: v2 node configuration", func() {
	f := SetupHelmConfig(``)

	renderWith := func(registryValues string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryValues)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	Context("discriminator off", func() {
		BeforeEach(func() { renderWith(v2Disabled) })

		It("tells nodes nothing about an agent, so the old writer keeps the directory", func() {
			secret := f.KubernetesResource("Secret", "d8-system", "registry-bashible-config")
			Expect(secret.Exists()).To(BeTrue())

			config := decodeBashibleConfig(secret)
			Expect(config).ToNot(ContainSubstring("agent"))
			Expect(config).To(ContainSubstring("registry.example.com"))
		})

		It("renders no marker of a switch that has not happened", func() {
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-v2-switch").Exists()).To(BeFalse())
		})

		It("renders no RegistryConfig", func() {
			Expect(f.KubernetesResource("RegistryConfig", "", "registry").Exists()).To(BeFalse())
		})
	})

	Context("discriminator on", func() {
		BeforeEach(func() { renderWith(v2Enabled) })

		It("hands the container runtime configuration to the agent", func() {
			secret := f.KubernetesResource("Secret", "d8-system", "registry-bashible-config")
			Expect(secret.Exists()).To(BeTrue())

			config := decodeBashibleConfig(secret)
			Expect(config).To(ContainSubstring("agent"))
			Expect(config).To(ContainSubstring("127.0.0.1:5001"))
			// The layout the node routes by before it can reach the API server, without
			// which a node joining a new cluster cannot pull the control plane.
			Expect(config).To(ContainSubstring("backends"))
		})

		It("records that the switch happened, so a restart does not re-ask the gate", func() {
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-v2-switch").Exists()).To(BeTrue())
		})

		It("renders the singleton RegistryConfig from the resolved configuration", func() {
			config := f.KubernetesGlobalResource("RegistryConfig", "registry")
			Expect(config.Exists()).To(BeTrue())
			Expect(config.Field("spec.mode").String()).To(Equal("Managed"))
			Expect(config.Field("spec.primary.upstream.host").String()).To(Equal("registry.deckhouse.io"))
			Expect(config.Field("spec.primary.upstream.path").String()).To(Equal("/deckhouse/ee"))
			Expect(config.Field("spec.storage.cache").Bool()).To(BeTrue())
			Expect(config.Field("spec.storage.size").String()).To(Equal("50Gi"))
		})
	})

	Context("an air-gapped cluster", func() {
		BeforeEach(func() { renderWith(v2AirGap) })

		It("renders a RegistryConfig with no upstream at all", func() {
			config := f.KubernetesGlobalResource("RegistryConfig", "registry")
			Expect(config.Exists()).To(BeTrue())
			Expect(config.Field("spec.primary").Exists()).To(BeFalse())
			Expect(config.Field("spec.storage.cache").Bool()).To(BeTrue())
		})
	})
})

func decodeBashibleConfig(secret object_store.KubeObject) string {
	decoded, err := base64.StdEncoding.DecodeString(secret.Field("data.config").String())
	Expect(err).ShouldNot(HaveOccurred())
	return string(decoded)
}

// Objects both implementations name identically. The switch gate is what normally keeps
// the two apart, but a collision on these would be decided by template ordering, and one
// of them carries the name every image reference in the cluster is built from — so the
// templates refuse it themselves rather than trusting the state machine that feeds them.
var _ = Describe("Module :: registry :: helm template :: v2 object ownership", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2WithLegacyStillActive)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("gives the registry Service to the new implementation alone", func() {
		service := f.KubernetesResource("Service", "d8-system", "registry")
		Expect(service.Exists()).To(BeTrue())
		Expect(service.Field("spec.selector.app").String()).To(Equal("registry-storage"),
			"the legacy Service definition won, so pulls would resolve to the old components")
	})

	It("gives the publication endpoint to the new implementation alone", func() {
		service := f.KubernetesResource("Service", "d8-system", "registry-push")
		Expect(service.Exists()).To(BeTrue())
		Expect(service.Field("spec.selector.app").String()).To(Equal("registry-storage"))

		// The legacy publication Ingress is named `registry`, the new one `registry-push`.
		// Both existing at once would publish two write endpoints for one cluster.
		Expect(f.KubernetesResource("Ingress", "d8-system", "registry").Exists()).To(BeFalse())
	})
})

// What a client needs to reach the storage. The addresses are the part that is easy to
// get wrong: the Service name is what image references are built from, but everything
// that dials the storage does so from a node, in the host network, where a cluster DNS
// name does not exist.
var _ = Describe("Module :: registry :: helm template :: v2 storage access", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("publishes the replica addresses, not the service name", func() {
		secret := f.KubernetesResource("Secret", "d8-system", "registry-storage-access")
		Expect(secret.Exists()).To(BeTrue())

		addresses, err := base64.StdEncoding.DecodeString(secret.Field("data.addresses").String())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(addresses)).To(Equal("10.0.0.1,10.0.0.2"))
	})

	It("keeps the private key out of what clients read", func() {
		secret := f.KubernetesResource("Secret", "d8-system", "registry-storage-access")
		Expect(secret.Field("data").Map()).ToNot(HaveKey("ca.key"))
		Expect(secret.Field("data").Map()).ToNot(HaveKey("tls.key"))
	})
})

// The token service. The registry advertises a token endpoint to every client and
// exchanges credentials there; without one running, every authenticated pull from the
// cache fails at the exchange with an error that says nothing about a missing container.
var _ = Describe("Module :: registry :: helm template :: v2 token service", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("runs beside the registry it issues tokens for", func() {
		set := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
		containers := set.Field("spec.template.spec.containers").Array()

		var auth gjson.Result
		for _, container := range containers {
			if container.Get("name").String() == "auth" {
				auth = container
			}
		}
		Expect(auth.Exists()).To(BeTrue())
		Expect(auth.Get("args").Array()).To(HaveLen(2))
		Expect(auth.Get("args.1").String()).To(Equal("/auth/auth_config.yaml"))

		// Probed on the loopback address explicitly: the pod shares the host network, so
		// the default probe host is the node address, where this does not listen.
		Expect(auth.Get("livenessProbe.httpGet.host").String()).To(Equal("127.0.0.1"))
		Expect(auth.Get("readinessProbe.httpGet.host").String()).To(Equal("127.0.0.1"))
	})

	It("grants the read account pulls and the write account everything", func() {
		secret := f.KubernetesResource("Secret", "d8-system", "registry-storage-auth-config")
		Expect(secret.Exists()).To(BeTrue())

		config := secret.Field("stringData.auth_config\\.yaml").String()
		// Loopback only. The registry fetches tokens on a client's behalf, so nothing
		// outside the pod talks to this — which is what keeps it off every node address
		// and out of every certificate.
		Expect(config).To(ContainSubstring(`addr: "127.0.0.1:5051"`))

		// The hashes, never the plaintext: this is the service that verifies a password,
		// not one that presents it.
		Expect(config).To(ContainSubstring("RO-HASH"))
		Expect(config).To(ContainSubstring("RW-HASH"))
		Expect(config).ToNot(ContainSubstring("RO-PASSWORD"))
		Expect(config).ToNot(ContainSubstring("RW-PASSWORD"))

		Expect(config).To(ContainSubstring(`actions: ["pull"]`))
		Expect(config).To(ContainSubstring(`actions: ["*"]`))
	})
})

// The garbage collection instruction, which reaches the replicas through the resource rather
// than through each of them reading the ModuleConfig: three replicas taking turns behind one
// lease have to agree on when a turn comes round.
var _ = Describe("Module :: registry :: helm template :: v2 garbage collection", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("carries the schedule into the RegistryConfig", func() {
		config := f.KubernetesGlobalResource("RegistryConfig", "registry")
		Expect(config.Field("spec.storage.garbageCollection.enabled").Bool()).To(BeTrue())
		Expect(config.Field("spec.storage.garbageCollection.schedule").String()).To(Equal("17 3 * * *"))
	})

	It("gives the syncer the store to reclaim", func() {
		set := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
		containers := set.Field("spec.template.spec.containers").Array()

		var syncer gjson.Result
		for _, container := range containers {
			if container.Get("name").String() == "syncer" {
				syncer = container
			}
		}
		Expect(syncer.Exists()).To(BeTrue())

		// The collector walks the filesystem directly, which is why reclaiming cannot be
		// done through the registry API alone.
		mounts := []string{}
		for _, mount := range syncer.Get("volumeMounts").Array() {
			mounts = append(mounts, mount.Get("mountPath").String())
		}
		Expect(mounts).To(ContainElement("/data"))
	})

	It("lets the syncer take a lease and read what the cluster is running", func() {
		role := f.KubernetesResource("Role", "d8-system", "registry:storage")
		Expect(role.Exists()).To(BeTrue())
		Expect(role.Field("rules").String()).To(ContainSubstring("coordination.k8s.io"))

		cluster := f.KubernetesGlobalResource("ClusterRole", "d8:registry:storage")
		Expect(cluster.Field("rules").String()).To(ContainSubstring("deckhousereleases"))
		// Read-only: this is the one input that justifies a deletion.
		Expect(cluster.Field("rules").String()).ToNot(ContainSubstring("deckhousereleases/status"))
	})
})

// What the module deploys is decided by three separate questions, and conflating them is
// what once made a default installation fail: everything rendered at `Unmanaged`, and the
// publication endpoint — whose own condition is "no upstream", which `Unmanaged` satisfies
// vacuously — could not render for want of cert-manager and took the module down with it.
var _ = Describe("Module :: registry :: helm template :: v2 what gets deployed", func() {
	f := SetupHelmConfig(``)

	renderWith := func(global, registryValues string) {
		f.ValuesSetFromYaml("global", global)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryValues)
		f.HelmRender()
	}

	Context("the module manages nothing", func() {
		BeforeEach(func() { renderWith(globalValues, v2EnabledUnmanaged) })

		It("renders without error", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("deploys no components at all", func() {
			Expect(f.KubernetesResource("Deployment", "d8-system", "registry-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("StatefulSet", "d8-system", "registry-storage").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-system", "registry").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-system", "registry-push").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Ingress", "d8-system", "registry-push").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-storage-pki").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("RegistryConfig", "registry").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "registry-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "registry-storage").Exists()).To(BeFalse())
		})

		It("still records that the switch happened", func() {
			// Not conditional on the mode: the record answers whether the previous
			// implementation has handed over, and going Unmanaged does not hand it back.
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-v2-switch").Exists()).To(BeTrue())
		})
	})

	Context("managed with the cache turned off", func() {
		BeforeEach(func() { renderWith(globalValues, v2ManagedNoCache) })

		It("deploys the controller but no storage", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Deployment", "d8-system", "registry-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("RegistryConfig", "registry").Exists()).To(BeTrue())

			Expect(f.KubernetesResource("StatefulSet", "d8-system", "registry-storage").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-system", "registry").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-storage-pki").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "registry-storage").Exists()).To(BeFalse())
		})
	})

	Context("air-gapped, on the converge before cert-manager exists", func() {
		BeforeEach(func() { renderWith(globalValuesBeforeCertManager, v2AirGap) })

		It("renders the cache without the publication endpoint, rather than failing", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("StatefulSet", "d8-system", "registry-storage").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Ingress", "d8-system", "registry-push").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-system", "registry-push").Exists()).To(BeFalse())
		})
	})
})

// Two definitions of one object are not merged. Helm renders both, the object store here
// keys by kind/namespace/name and so silently keeps the last, and nelm refuses the release
// on install — which means a duplicate is invisible to every other spec in this file and
// fatal in a cluster. That is how a leftover `Role/registry:storage` reached a test cluster
// and left the module unable to start.
var _ = Describe("Module :: registry :: helm template :: no duplicated objects", func() {
	f := SetupHelmConfig(``)

	// Every configuration that renders anything, since a duplicate can hide in a branch.
	for name, fixture := range map[string]string{
		"unmanaged":             v2EnabledUnmanaged,
		"managed without cache": v2ManagedNoCache,
		"a pass-through cache":  v2Enabled,
		"an air-gapped cache":   v2AirGap,
	} {
		name, fixture := name, fixture

		It("renders each object exactly once with "+name, func() {
			rendered := map[string]string{}
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", fixture)
			f.HelmRender(WithRenderOutput(rendered))
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			type identity struct{ kind, namespace, name string }
			seen := map[identity][]string{}

			for path, manifests := range rendered {
				for _, doc := range strings.Split(manifests, "\n---") {
					if strings.TrimSpace(doc) == "" {
						continue
					}
					var obj struct {
						Kind     string `yaml:"kind"`
						Metadata struct {
							Name      string `yaml:"name"`
							Namespace string `yaml:"namespace"`
						} `yaml:"metadata"`
					}
					if err := yaml.Unmarshal([]byte(doc), &obj); err != nil || obj.Kind == "" {
						continue
					}
					id := identity{obj.Kind, obj.Metadata.Namespace, obj.Metadata.Name}
					seen[id] = append(seen[id], path)
				}
			}

			for id, paths := range seen {
				Expect(paths).To(HaveLen(1),
					"%s %s/%s is defined %d times (%v); one definition would silently win",
					id.kind, id.namespace, id.name, len(paths), paths)
			}
		})
	}
})

// The name of the auth Secret appears in a Helm template and in a Go constant, and the two
// have to agree: the components resolve credentials against the constant, while RBAC grants
// access to the name in the template. A drift between them is silent — the agent gets a
// forbidden error on a Secret nobody granted, and the pull fails with a 401 that mentions
// neither.
var _ = Describe("Module :: registry :: helm template :: the auth Secret is granted by name", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("grants the node agent exactly that Secret and nothing else", func() {
		role := f.KubernetesResource("Role", "d8-system", "registry:agent")
		Expect(role.Exists()).To(BeTrue())

		rule := role.Field("rules.0")
		Expect(rule.Get("resources").Array()).To(HaveLen(1))
		Expect(rule.Get("resources.0").String()).To(Equal("secrets"))
		Expect(rule.Get("resourceNames").Array()).To(HaveLen(1),
			"without resourceNames this grants every Secret in the namespace to system:nodes")
		Expect(rule.Get("resourceNames.0").String()).To(Equal(constant.AuthSecretName))
		Expect(rule.Get("verbs").Array()).NotTo(ContainElement(gjsonString("create")))
		// The agent reads this with an uncached client, which is what lets the grant stay
		// name-scoped: a cached read would need a list nobody can authorize by name.
		Expect(rule.Get("verbs").Array()).NotTo(ContainElement(gjsonString("list")))
	})

	It("grants the storage the same one", func() {
		role := f.KubernetesResource("Role", "d8-system", "registry:storage")
		Expect(role.Exists()).To(BeTrue())

		var granted []string
		for _, rule := range role.Field("rules").Array() {
			for _, name := range rule.Get("resourceNames").Array() {
				granted = append(granted, name.String())
			}
		}
		Expect(granted).To(ContainElement(constant.AuthSecretName))
	})
})

func gjsonString(s string) gjson.Result {
	return gjson.Result{Type: gjson.String, Str: s}
}

// The controller's own access to the two Secrets it cannot work without.
//
// Missing, this fails in the least legible way there is: a cached client backs every read
// with an informer, the informer's first list is refused, the cache never starts, and every
// controller in the manager stalls. Nothing in the module's own status says so — the symptom
// is an empty cluster (no RegistryStorage, no RegistryNode) and nodes that cannot pull. It
// took a three-master install to notice, because on a single master the control plane comes
// up on images the installer pulled and never needs the module at all.
var _ = Describe("Module :: registry :: helm template :: the controller can reach its Secrets", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("may read the storage access secret and write the resolved auth secret", func() {
		role := f.KubernetesResource("Role", "d8-system", "registry:controller")
		Expect(role.Exists()).To(BeTrue())

		// The rule this encodes, which cost two cluster installs to get right: resourceNames
		// authorizes only the verbs that address an object already there — get, update,
		// patch, delete. A create names itself in the body, and a list names nothing at all,
		// so a rule carrying resourceNames never matches either and the request is refused.
		//
		// So the shape has to be: everything that reads or modifies these two Secrets is
		// named, `create` stands alone unnamed because it cannot be named, and list and watch
		// appear nowhere — the components read uncached for exactly that reason.
		named := map[string][]string{}
		var unnamed []string
		for _, rule := range role.Field("rules").Array() {
			isSecrets := false
			for _, resource := range rule.Get("resources").Array() {
				if resource.String() == "secrets" {
					isSecrets = true
				}
			}
			if !isSecrets {
				continue
			}

			names := rule.Get("resourceNames").Array()
			for _, verb := range rule.Get("verbs").Array() {
				if len(names) == 0 {
					unnamed = append(unnamed, verb.String())
					continue
				}
				for _, name := range names {
					named[name.String()] = append(named[name.String()], verb.String())
				}
			}
		}

		Expect(named).To(HaveKey("registry-storage-access"))
		Expect(named["registry-storage-access"]).To(ContainElement("get"))

		Expect(named).To(HaveKey(constant.AuthSecretName))
		Expect(named[constant.AuthSecretName]).To(ContainElements("get", "update", "patch"),
			"the controller is the only writer of the credentials its layouts reference")

		Expect(unnamed).To(ConsistOf("create"),
			"create is the one verb that cannot be restricted by name; nothing else belongs "+
				"in an unnamed rule, least of all a read")

		for name, granted := range named {
			Expect(granted).NotTo(ContainElement("list"),
				"%s: list cannot be authorized by name, so this grant is a trap", name)
			Expect(granted).NotTo(ContainElement("watch"),
				"%s: watch cannot be authorized by name, so this grant is a trap", name)
			Expect(granted).NotTo(ContainElement("create"),
				"%s: create cannot be authorized by name, so this grant does nothing", name)
		}
	})
})

// The storage runs as root, and says so.
//
// Not a preference — a requirement of what it touches. The registry reads its certificate
// material from a Secret mount, and a Secret volume's files are owned by root with mode 0400;
// it writes its store to a hostPath owned by root on the node. The base image is distroless and
// runs as 64535, so an unset security context is not "root by default", it is "whatever the
// image says, decided in a file this one does not mention" — and the result is a pod that
// crash-loops on `permission denied` opening a token bundle that is plainly there.
var _ = Describe("Module :: registry :: helm template :: the storage runs as root explicitly", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("sets the user in the pod spec rather than inheriting it from the image", func() {
		sts := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
		Expect(sts.Exists()).To(BeTrue())

		security := sts.Field("spec.template.spec.securityContext")
		Expect(security.Exists()).To(BeTrue(),
			"an unset security context takes the user from the distroless base image, which is not root")
		Expect(security.Get("runAsUser").Int()).To(BeEquivalentTo(0),
			"the registry reads root-owned 0400 files from a Secret mount and writes a root-owned hostPath")
		Expect(security.Get("runAsNonRoot").Bool()).To(BeFalse())
		Expect(security.Get("seccompProfile.type").String()).To(Equal("RuntimeDefault"))
	})
})
