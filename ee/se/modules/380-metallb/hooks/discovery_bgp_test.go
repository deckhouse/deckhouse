/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: metallb :: hooks :: discovery_bgp ::", func() {
	f := HookExecutionConfigInit(`{"metallb":{"internal": {}}}`, "")
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "MetalLoadBalancerPool", false)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "MetalLoadBalancerBGPPeer", false)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "MetalLoadBalancerConfiguration", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should execute successfully and set empty arrays with default affinity", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("metallb.internal.addressPools").String()).To(MatchYAML(`[]`))
			Expect(f.ValuesGet("metallb.internal.bgpPeers").String()).To(MatchYAML(`[]`))
			Expect(f.ValuesGet("metallb.internal.bgpAdvertisements").String()).To(MatchYAML(`[]`))
			Expect(f.ValuesGet("metallb.internal.bfdProfiles").String()).To(MatchYAML(`[]`))
			Expect(f.ValuesGet("metallb.internal.secretsToCopy").String()).To(MatchYAML(`[]`))

			// Check default affinity
			Expect(f.ValuesGet("metallb.internal.speakerNodeAffinity").String()).To(MatchYAML(`{}`))
		})
	})

	Context("With pools, peers, configuration and secrets", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  labels:
    network.deckhouse.io/metallb-bgp-password: "true"
  name: secret1
  namespace: ns1
data:
  password: cGFzc3dvcmQ= # "password"
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerPool
metadata:
  name: test-pool
spec:
  addresses:
  - 10.0.0.1-10.0.0.10
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerBGPPeer
metadata:
  name: test-peer
spec:
  peerAddress: 192.168.1.1
  peerASN: 65001
  myASN: 65000
  passwordSecretRef:
    name: secret1
    namespace: ns1
  bfd:
    receiveInterval: 300
    transmitInterval: 300
  sourceAddresses:
  - nodeName: node-1
    address: 10.10.10.1
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerConfiguration
metadata:
  name: test-config
spec:
  mode: BGP
  nodeSelector:
    role: worker
  bgp:
    peerNames:
    - test-peer
  advertisements:
  - poolNames:
    - test-pool
    bgp:
      localPref: 100
      communities:
      - "1111:2222"
`))
			f.RunHook()
		})

		It("Should generate correct internal values with secret data", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("metallb.internal.addressPools").String()).To(MatchYAML(`
- addresses:
  - 10.0.0.1-10.0.0.10
  name: test-pool
`))

			Expect(f.ValuesGet("metallb.internal.bgpPeers").String()).To(MatchYAML(`
- bfdProfile: bfd-test-peer
  myASN: 65000
  name: test-peer-node-node-1
  nodeSelectors:
  - matchLabels:
      kubernetes.io/hostname: node-1
  passwordSecret: bgp-pwd-ns1-secret1
  peerASN: 65001
  peerAddress: 192.168.1.1
  sourceAddress: 10.10.10.1
- bfdProfile: bfd-test-peer
  myASN: 65000
  name: test-peer-test-config
  nodeSelectors:
  - matchExpressions:
    - key: kubernetes.io/hostname
      operator: NotIn
      values:
      - node-1
    matchLabels:
      role: worker
  passwordSecret: bgp-pwd-ns1-secret1
  peerASN: 65001
  peerAddress: 192.168.1.1
`))

			Expect(f.ValuesGet("metallb.internal.bgpAdvertisements").String()).To(MatchYAML(`
- communities:
  - 1111:2222
  ipAddressPools:
  - test-pool
  localPref: 100
  name: test-config-adv-0
  nodeSelectors:
  - matchLabels:
      role: worker
  peers:
  - test-peer
`))

			Expect(f.ValuesGet("metallb.internal.bfdProfiles").String()).To(MatchYAML(`
- name: bfd-test-peer
  receiveInterval: 300
  transmitInterval: 300
`))

			Expect(f.ValuesGet("metallb.internal.secretsToCopy").String()).To(MatchYAML(`
- data:
    password: password
  name: bgp-pwd-ns1-secret1
  namespace: ns1
`))

			Expect(f.ValuesGet("metallb.internal.speakerNodeAffinity").String()).To(MatchYAML(`
requiredDuringSchedulingIgnoredDuringExecution:
  nodeSelectorTerms:
  - matchExpressions:
    - key: role
      operator: In
      values:
      - worker
  - matchExpressions:
    - key: kubernetes.io/hostname
      operator: In
      values:
      - node-1
`))
		})
	})

	Context("Sorting stability", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerPool
metadata:
  name: z-pool
spec:
  addresses: ["1.1.1.1/32"]
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerPool
metadata:
  name: a-pool
spec:
  addresses: ["2.2.2.2/32"]
`))
			f.RunHook()
		})

		It("Should always sort outputs by name", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("metallb.internal.addressPools").String()).To(MatchYAML(`
- addresses:
  - 2.2.2.2/32
  name: a-pool
- addresses:
  - 1.1.1.1/32
  name: z-pool
`))
		})
	})
})