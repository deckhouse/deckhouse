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

// Nothing of the previous implementation reaches a cluster the current one owns.
//
// This is not a tidiness property. The two implementations both write
// /etc/containerd/registry.d on every node and both create a Service named `registry`, so
// "the old one is also deployed" means two writers racing over how the cluster pulls
// images. It was measured on a fresh cluster with the cache configured: a
// `registry-incluster-proxy` pod sat Pending beside a store that was already serving, and
// what put it there was not a configuration anybody wrote — `d8-system/registry-config`
// exists on every cluster, module 002 renders it unconditionally, and that secret was all
// the legacy hook needed to start a state machine for a cluster it does not own.
//
// The hook that clears the legacy values is the mechanism; these conditions are the lock at
// the layer that actually produces objects, and the only layer a test can see.
//
// Every case below is asserted in both directions from one set of values. A test that only
// checked absence would pass just as well against values too thin to render anything, which
// is the failure mode a test like this is most likely to have.
var _ = Describe("Module :: registry :: helm template :: the previous implementation stands down", func() {
	// A legacy state with every part of it populated: the PKI, the accounts, the node
	// services, the in-cluster proxy, and the checker on its own values path. What a
	// cluster mid-migration has.
	const legacyPopulated = `
internal:
  checker:
    params:
      version: "checker1"
      registries:
        registry.example.com:
          address: registry.example.com
          scheme: HTTPS
    state:
      queues:
        registry.example.com:
          processed: 0
  orchestrator:
    hash: "123"
    state:
      mode: "Proxy"
      target_mode: "Proxy"
      conditions: []
      registry_service: "node-services"
      ingress_enabled: false
      pki:
        ca:
          cert: LEGACY-CA-CERT
          key: LEGACY-CA-KEY
        token:
          cert: LEGACY-TOKEN-CERT
          key: LEGACY-TOKEN-KEY
      users:
        ro:
          name: ro
          password: RO-PASSWORD
          password_hash: RO-HASH
        rw:
          name: rw
          password: RW-PASSWORD
          password_hash: RW-HASH
      node_services:
        run: true
        nodes:
          master-0:
            version: "nodeservices1"
            config:
              mode: Proxy
      in_cluster_proxy:
        config:
          version: "proxy1"
          config:
            ca: LEGACY-CA-CERT
            auth_cert: AUTH-CERT
            auth_key: AUTH-KEY
            token_cert: TOKEN-CERT
            token_key: TOKEN-KEY
            distribution_cert: DISTRIBUTION-CERT
            distribution_key: DISTRIBUTION-KEY
            http_secret: HTTP-SECRET
            upstream:
              scheme: https
              host: registry.example.com
              path: /deckhouse/ee
              user:
                name: ro
                password: RO-PASSWORD
                password_hash: RO-HASH
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
`

	// The objects that exist for the legacy implementation and for nothing else. Deliberately
	// not `Service/registry` or `registry-push`: both implementations create those, so their
	// absence is not the question — ownership of them is, and that is asserted next door in
	// v2_controller_test.go.
	type legacyObject struct {
		kind      string
		namespace string
		name      string
	}

	legacyObjects := []legacyObject{
		{"Secret", "d8-system", "registry-state"},
		{"Secret", "d8-system", "registry-pki"},
		{"Secret", "d8-system", "registry-user-ro"},
		{"Secret", "d8-system", "registry-user-rw"},
		{"Secret", "d8-system", "registry-checker-state"},
		{"ServiceAccount", "d8-system", "registry-nodeservices"},
		{"Role", "d8-system", "registry:nodeservices"},
		{"RoleBinding", "d8-system", "registry:nodeservices"},
		{"DaemonSet", "d8-system", "registry-nodeservices-manager"},
		{"Secret", "d8-system", "registry-node-config-master-0"},
		{"Deployment", "d8-system", "registry-incluster-proxy"},
		{"Secret", "d8-system", "registry-incluster-proxy-config"},
	}

	// The gate is gone, by decision: of the previous implementation only `Unmanaged` is supported, and its
	// objects may not appear on a cluster again. This context used to assert the opposite half — that with
	// the switch off, these values DO render these objects — which was what made the absence below
	// meaningful. That half cannot be asserted any more, because there is no longer a way to ask for it.
	//
	// What replaces it: the values are populated exactly as they were, the switch is set to the state that
	// used to hand the cluster to the previous implementation, and nothing of it renders. So the absence
	// is still not a gap in the fixture — the fixture is the one that used to produce fourteen objects.
	Context("with the switch off, which used to hand the cluster over", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", legacyPopulated)
			f.ValuesSet("registry.internal.v2.enabled", false)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("still deploys nothing of the previous implementation", func() {
			for _, object := range legacyObjects {
				Expect(f.KubernetesResource(object.kind, object.namespace, object.name).Exists()).
					To(BeFalse(), "%s/%s must not be reachable through the switch any more",
						object.kind, object.name)
			}

			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:registry:nodeservices").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:registry:nodeservices").Exists()).To(BeFalse())
		})
	})

	Context("once the current implementation owns the cluster", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", legacyPopulated)
			f.ValuesSet("registry.internal.v2.enabled", true)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("deploys nothing of the previous implementation", func() {
			for _, object := range legacyObjects {
				Expect(f.KubernetesResource(object.kind, object.namespace, object.name).Exists()).
					To(BeFalse(), "%s/%s belongs to the implementation that no longer owns this cluster",
						object.kind, object.name)
			}

			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:registry:nodeservices").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:registry:nodeservices").Exists()).To(BeFalse())
		})

		// Called out separately because it is the one the gate exists for. The legacy node
		// services are the other writer of the container runtime's registry configuration;
		// the in-cluster proxy is what was actually found running on a cluster that had never
		// asked for it.
		It("puts no second writer on the nodes", func() {
			Expect(f.KubernetesResource("DaemonSet", "d8-system", "registry-nodeservices").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-system", "registry-incluster-proxy").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-incluster-proxy-config").Exists()).To(BeFalse())
		})

		// The state secret is the most consequential of them. The gate that decides which
		// implementation is active reads it, so a legacy state recorded on a cluster the
		// current implementation owns is a record that can take that cluster back on the next
		// module restart — with the current implementation already configuring every node.
		It("records no state that could take the cluster back", func() {
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-state").Exists()).To(BeFalse())
		})

		// What nodes are told still has exactly one author. The legacy state above carries a
		// bashible config of its own, and this secret is the single object both
		// implementations would write.
		It("leaves the node configuration to the current implementation", func() {
			secret := f.KubernetesResource("Secret", "d8-system", "registry-bashible-config")
			Expect(secret.Exists()).To(BeFalse(),
				"with no bashibleConfig in the v2 values there is nothing to write, and the legacy "+
					"config must not be what fills the gap")
		})
	})
})
