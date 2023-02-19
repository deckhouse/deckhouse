/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: discovery_versions ::", func() {
	f := HookExecutionConfigInit(`{"istio":{ "internal": {} }}`, "")

	Context("Empty cluster and no settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LogrusOutput.Contents()).To(HaveLen(0))
			Expect(f.ValuesGet("istio.internal.versionMap").Map()).To(HaveLen(0))
		})
	})

	Context("Some istio images exist", func() {
		BeforeEach(func() {
			values := `
operatorV1x22x3: "operator-img"
pilotV1x22x3: "pilot-img"
operatorV1x55x6: "operator-img"
pilotV1x55x6: "pilot-img"
pilot: "old-pilot-img"
pilotV11: "old-pilot-img"
pilotV1x11: "old-pilot-img"
pilotVx11x11x11x11: "old-pilot-img"
`
			f.ValuesSetFromYaml("global.modulesImages.tags.istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LogrusOutput.Contents()).To(HaveLen(0))
			Expect(f.ValuesGet("istio.internal.versionMap").Map()).To(HaveLen(2))
			// 1.22
			Expect(f.ValuesGet("istio.internal.versionMap.1\\.22.fullVersion").String()).To(Equal("1.22.3"))
			Expect(f.ValuesGet("istio.internal.versionMap.1\\.22.revision").String()).To(Equal("v1x22"))
			Expect(f.ValuesGet("istio.internal.versionMap.1\\.22.imageSuffix").String()).To(Equal("V1x22x3"))
			Expect(f.ValuesGet("istio.internal.versionMap.1\\.22.isReady").Bool()).To(BeFalse())
			// 1.55
			Expect(f.ValuesGet("istio.internal.versionMap.1\\.55.fullVersion").String()).To(Equal("1.55.6"))
			Expect(f.ValuesGet("istio.internal.versionMap.1\\.55.revision").String()).To(Equal("v1x55"))
			Expect(f.ValuesGet("istio.internal.versionMap.1\\.55.imageSuffix").String()).To(Equal("V1x55x6"))
			Expect(f.ValuesGet("istio.internal.versionMap.1\\.55.isReady").Bool()).To(BeFalse())
		})
	})

})
