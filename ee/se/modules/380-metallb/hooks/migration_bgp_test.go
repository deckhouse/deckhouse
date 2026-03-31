/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// TODO: remove after 1.82.

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: metallb :: hooks :: migration_bgp", func() {
	f := HookExecutionConfigInit(`{"metallb":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "MetalLoadBalancerPool", false)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "MetalLoadBalancerBGPPeer", false)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "MetalLoadBalancerConfiguration", false)

	Context("With version 2 configuration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  version: 2
  settings:
    speaker:
      nodeSelector:
        node-role.deckhouse.io/frontend: ""
    bgpPeers:
    - my-asn: 65000
      peer-address: 192.168.1.1
      peer-asn: 65001
      peer-port: 179
      router-id: 10.0.0.254
      hold-time: 90s
    bgpCommunities:
      no-advertise: "65535:65282"
    addressPools:
    - name: bgp-pool
      protocol: bgp
      addresses:
      - 10.0.0.1/32
      bgp-advertisements:
      - localpref: 100
        communities: ["no-advertise"]
        aggregation-length: 24
    - name: l2-pool
      protocol: layer2
      addresses:
      - 10.0.0.2/32
`))
			f.RunHook()
		})

		It("Should migrate to CRDs and NOT update ModuleConfig version", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Check ModuleConfig version and settings were NOT updated
			mc := f.KubernetesResource("ModuleConfig", "", "metallb")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec").String()).To(MatchYAML(`
settings:
  addressPools:
  - addresses:
    - 10.0.0.1/32
    bgp-advertisements:
    - aggregation-length: 24
      communities:
      - no-advertise
      localpref: 100
    name: bgp-pool
    protocol: bgp
  - addresses:
    - 10.0.0.2/32
    name: l2-pool
    protocol: layer2
  bgpCommunities:
    no-advertise: 65535:65282
  bgpPeers:
  - my-asn: 65000
    peer-address: 192.168.1.1
    peer-asn: 65001
    peer-port: 179
    router-id: 10.0.0.254
    hold-time: 90s
  speaker:
    nodeSelector:
      node-role.deckhouse.io/frontend: ""
version: 2
`))

			// Check created Pool
			pool := f.KubernetesResource("MetalLoadBalancerPool", "", "bgp-pool")
			Expect(pool.Exists()).To(BeTrue())
			Expect(pool.Field("spec").String()).To(MatchYAML(`
addresses:
- 10.0.0.1/32
`))

			// Check created Peer
			peer := f.KubernetesResource("MetalLoadBalancerBGPPeer", "", "peer-0")
			Expect(peer.Exists()).To(BeTrue())
			Expect(peer.Field("spec").String()).To(MatchYAML(`
myASN: 65000
peerASN: 65001
peerAddress: 192.168.1.1
peerPort: 179
routerID: 10.0.0.254
holdTime: 90s
`))

			// Check created Configuration
			config := f.KubernetesResource("MetalLoadBalancerConfiguration", "", "migrated-bgp")
			Expect(config.Exists()).To(BeTrue())
			Expect(config.Field("spec").String()).To(MatchYAML(`
advertisements:
- bgp:
    aggregationLength: 24
    communities:
    - 65535:65282
    localPref: 100
  poolNames:
  - bgp-pool
bgp:
  peerNames:
  - peer-0
mode: BGP
nodeSelector:
  node-role.deckhouse.io/frontend: ""
`))
		})
	})

	Context("Migration idempotency", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerPool
metadata:
  name: bgp-pool
spec:
  addresses: ["1.1.1.1/32"]
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  version: 2
  settings:
    addressPools:
    - name: bgp-pool
      protocol: bgp
      addresses: ["10.0.0.1/32"]
`))
			f.RunHook()
		})

		It("Should skip migration if resources already exist", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Check Pool was NOT updated from ModuleConfig
			pool := f.KubernetesResource("MetalLoadBalancerPool", "", "bgp-pool")
			Expect(pool.Field("spec").String()).To(MatchYAML(`
addresses:
- 1.1.1.1/32
`))

			// Version should NOT be updated
			mc := f.KubernetesResource("ModuleConfig", "", "metallb")
			Expect(mc.Field("spec").String()).To(MatchYAML(`
settings:
  addressPools:
  - addresses:
    - 10.0.0.1/32
    name: bgp-pool
    protocol: bgp
version: 2
`))
		})
	})

	Context("With version 3 configuration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  version: 3
  settings:
    speaker:
      nodeSelector:
        node-role.deckhouse.io/frontend: ""
`))
			f.RunHook()
		})

		It("Should skip migration", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("MetalLoadBalancerConfiguration", "migrated-bgp").Exists()).To(BeFalse())
		})
	})
})
