/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler-crd", "static-routing-manager"]
`
	goodModuleValuesA = `
internal:
  nodeIPRuleSets:
    - name: myiprule-69028a3136
      nodeName: sandbox-worker-02334ee2-7694f-mt9rm
      ownerIRSName: myiprule
      ownerIRSUID: 641bfb93-a25a-483a-a433-a8e6dab7dd50
      rules:
        - actions:
            lookup:
              ipRoutingTableID: 100500
          priority: 50
          selectors:
            to:
              - 8.8.8.8/32
  nodeRoutingTables:
    - ipRoutingTableID: 100500
      name: external-952302c494
      nodeName: sandbox-worker-02334ee2-7694f-mt9rm
      ownerRTName: external
      ownerRTUID: 4d734e48-21aa-4cb3-ac95-138d30246bd6
      routes:
        - destination: 0.0.0.0/0
          gateway: 192.168.199.1
`
	desiredNRTSpecA = `
ipRoutingTableID: 100500
nodeName: sandbox-worker-02334ee2-7694f-mt9rm
routes:
  - destination: 0.0.0.0/0
    gateway: 192.168.199.1
`
	desiredNIRSSpecA = `
nodeName: sandbox-worker-02334ee2-7694f-mt9rm
rules:
  - actions:
      lookup:
        ipRoutingTableID: 100500
    priority: 50
    selectors:
      to:
        - 8.8.8.8/32
`
	goodModuleValuesB = `
internal:
  nodeIPRuleSets:
    - name: myiprule-69028a3136
      nodeName: sandbox-worker-02334ee2-7694f-mt9rm
      ownerIRSName: myiprule
      ownerIRSUID: 641bfb93-a25a-483a-a433-a8e6dab7dd50
      rules:
        - actions:
            lookup:
              ipRoutingTableID: 100500
              routingTableName: external
          priority: 50
          selectors:
            dportRange:
              end: 400
              start: 300
            from:
              - 192.168.111.0/24
              - 192.168.222.0/24
            fwMark: 0x42/0xff
            iif: eth1
            ipProto: 6
            oif: cilium_net
            sportRange:
              end: 200
              start: 100
            to:
              - 8.8.8.8/32
              - 172.16.8.0/21
            tos: "0x10"
            uidRange:
              start: 1001
              end: 2000
  nodeRoutingTables:
    - ipRoutingTableID: 100500
      name: external-952302c494
      nodeName: sandbox-worker-02334ee2-7694f-mt9rm
      ownerRTName: external
      ownerRTUID: 4d734e48-21aa-4cb3-ac95-138d30246bd6
      routes:
        - destination: 0.0.0.0/0
          gateway: 192.168.199.1
        - destination: 192.168.0.0/24
          gateway: 192.168.199.1
`
	desiredNRTSpecB = `
ipRoutingTableID: 100500
nodeName: sandbox-worker-02334ee2-7694f-mt9rm
routes:
  - destination: 0.0.0.0/0
    gateway: 192.168.199.1
  - destination: 192.168.0.0/24
    gateway: 192.168.199.1
`
	desiredNIRSSpecB = `
nodeName: sandbox-worker-02334ee2-7694f-mt9rm
rules:
- actions:
    lookup:
        ipRoutingTableID: 100500
  priority: 50
  selectors:
    dportRange:
        end: 400
        start: 300
    from:
        - 192.168.111.0/24
        - 192.168.222.0/24
    fwMark: 0x42/0xff
    iif: eth1
    ipProto: 6
    oif: cilium_net
    sportRange:
        end: 200
        start: 100
    to:
        - 8.8.8.8/32
        - 172.16.8.0/21
    tos: "0x10"
    uidRange:
        end: 2000
        start: 1001
`
)

var _ = Describe("Module :: staticRoutingManager :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Good test A (minimum number of parameters)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("staticRoutingManager", goodModuleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			nrt := f.KubernetesGlobalResource("SDNInternalNodeRoutingTable", "external-952302c494")
			nirs := f.KubernetesGlobalResource("SDNInternalNodeIPRuleSet", "myiprule-69028a3136")
			Expect(nrt.Exists()).To(BeTrue())
			Expect(nirs.Exists()).To(BeTrue())

			Expect(nrt.Field("spec").String()).To(MatchYAML(desiredNRTSpecA))
			Expect(nirs.Field("spec").String()).To(MatchYAML(desiredNIRSSpecA))
		})
	})

	Context("Good test B (maximum number of parameters)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("staticRoutingManager", goodModuleValuesB)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			nrt := f.KubernetesGlobalResource("SDNInternalNodeRoutingTable", "external-952302c494")
			nirs := f.KubernetesGlobalResource("SDNInternalNodeIPRuleSet", "myiprule-69028a3136")
			Expect(nrt.Exists()).To(BeTrue())
			Expect(nirs.Exists()).To(BeTrue())

			Expect(nrt.Field("spec").String()).To(MatchYAML(desiredNRTSpecB))
			Expect(nirs.Field("spec").String()).To(MatchYAML(desiredNIRSSpecB))
		})
	})
})
