/*
Copyright 2023 Flant JSC

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
			f.ValuesSetFromYaml("global.modulesImages.digests.istio", []byte(values))
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
