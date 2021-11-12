/*
Copyright 2021 Flant JSC

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

var _ = Describe("Modules :: deckhouse-web :: hooks :: get_deckhouse_tag_and_version ::", func() {

	const (
		initValuesString       = `{"deckhouseWeb":{"internal":{"deckhouseReleaseChannel":"","deckhouseVersion":"","deckhouseEdition":""}}}`
		initConfigValuesString = `{}`

		stateWithStableChannel = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    core.deckhouse.io/version: "1.25.1"
    core.deckhouse.io/edition: "CE"
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.deckhouse.io/deckhouse/ce:stable
---
apiVersion: v1
data:
  deckhouse: |
    bundle: Default
    logLevel: Info
    releaseChannel: Stable
kind: ConfigMap
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse
  namespace: d8-system
`
		stateWithAbsentAnnotations = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.deckhouse.io/deckhouse/ce:sometag
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouseWeb.internal.deckhouseVersion").String()).To(Equal(""))
			Expect(f.ValuesGet("deckhouseWeb.internal.deckhouseEdition").String()).To(Equal(""))
		})
	})

	Context("Absent core.deckhouse.io/version annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateWithAbsentAnnotations))
			f.RunHook()
		})

		It("Hook must not fail with an absent version annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("deckhouseWeb.internal.deckhouseVersion").String()).To(Equal("unknown"))
		})
	})

	Context("Absent core.deckhouse.io/edition annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateWithAbsentAnnotations))
			f.RunHook()
		})

		It("Hook must not fail with an absent edition annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("deckhouseWeb.internal.deckhouseEdition").String()).To(Equal("unknown"))
		})
	})

	Context("Deckhouse on a release channel", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateWithStableChannel))
			f.ConfigValuesSetFromYaml("deckhouse", []byte(`{
				"bundle": "Default", 
				"logLevel": "Info", 
				"releaseChannel": "Stable"
			}`))

			f.RunHook()
		})

		It("Hook must not fail, version, edition, tag and channel should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouseWeb.internal.deckhouseVersion").String()).To(Equal("1.25.1"))
			Expect(f.ValuesGet("deckhouseWeb.internal.deckhouseEdition").String()).To(Equal("CE"))
			Expect(f.ValuesGet("deckhouseWeb.internal.deckhouseReleaseChannel").String()).To(Equal("Stable"))
		})
	})

})
