/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// TODO: remove after 1.82.

package hooks

import (
	"fmt"
	"strings"

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
autoAssign: true
avoidBuggyIPs: false
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

	Context("With many BGP pools sharing a single IP address", func() {
		const (
			poolCount = 80
			sharedIP  = "192.168.100.100/32"
		)

		BeforeEach(func() {
			var sb strings.Builder
			sb.WriteString(`
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
        dedicated: metallb
    bgpPeers:
    - my-asn: 64500
      peer-address: 192.168.0.1
      peer-asn: 64500
      hold-time: 3s
    - my-asn: 64500
      peer-address: 192.168.0.2
      peer-asn: 64500
      hold-time: 3s
    addressPools:
`)
			localPrefs := []int{100, 150, 200}
			for i := 0; i < poolCount; i++ {
				sb.WriteString(fmt.Sprintf("    - name: pool-%02d-bgp\n", i))
				sb.WriteString("      protocol: bgp\n")
				sb.WriteString("      addresses:\n")
				sb.WriteString(fmt.Sprintf("      - %s\n", sharedIP))
				// Every 4th pool has no bgp-advertisements (like haproxy pools in the example).
				if i%4 != 0 {
					sb.WriteString("      bgp-advertisements:\n")
					sb.WriteString("      - aggregation-length: 32\n")
					sb.WriteString(fmt.Sprintf("        localpref: %d\n", localPrefs[i%len(localPrefs)]))
				}
			}

			f.BindingContexts.Set(f.KubeStateSet(sb.String()))
			f.RunHook()
		})

		It("Should migrate all pools that share the same IP", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Every pool must be migrated into its own MetalLoadBalancerPool with the shared address.
			for i := 0; i < poolCount; i++ {
				name := fmt.Sprintf("pool-%02d-bgp", i)
				pool := f.KubernetesResource("MetalLoadBalancerPool", "", name)
				Expect(pool.Exists()).To(BeTrue(), "pool %q should exist", name)
				Expect(pool.Field("spec.addresses").AsStringSlice()).To(Equal([]string{sharedIP}))
			}

			// The single configuration must contain one advertisement per pool and both peers.
			config := f.KubernetesResource("MetalLoadBalancerConfiguration", "", "migrated-bgp")
			Expect(config.Exists()).To(BeTrue())
			Expect(config.Field("spec.mode").String()).To(Equal("BGP"))
			Expect(config.Field("spec.advertisements.#").Int()).To(BeEquivalentTo(poolCount))
			Expect(config.Field("spec.bgp.peerNames").AsStringSlice()).To(Equal([]string{"peer-0", "peer-1"}))
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
