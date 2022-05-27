// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package smokemini

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: upmeter :: smokemini :: migrate_forcenostorageclass ::", func() {
	const (
		initValuesString       = `{}`
		initConfigValuesString = `{}`
	)

	Context("No config", func() {

		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not add settings", func() {
			exists := f.ConfigValuesGet("upmeter.smokeMini").Exists()
			Expect(exists).To(BeFalse())
		})
	})

	Context("Without smokeMini.storageClass but with other fields", func() {
		f := HookExecutionConfigInit(
			initValuesString,
			`
upmeter:
  smokeMini:
    auth: {}
`,
		)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not add settings", func() {
			v := f.ConfigValuesGet("upmeter")
			Expect(v.String()).To(MatchYAML(`smokeMini: { "auth": {} }`))
		})
	})

	Context("With smokeMini.storageClass and other fields", func() {
		f := HookExecutionConfigInit(
			initValuesString,
			`
upmeter:
  smokeMini:
    storageClass: "test"
    auth: {}
`,
		)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not add settings", func() {
			v := f.ConfigValuesGet("upmeter")
			Expect(v.String()).To(MatchYAML(`
smokeMini:
  auth: {}
`))
		})
	})

	Context("Only smokeMini.storageClass", func() {
		f := HookExecutionConfigInit(
			initValuesString,
			`
upmeter:
  smokeMini:
    storageClass: "rbd"
`,
		)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not store empty object when no fields left", func() {
			v := f.ConfigValuesGet("upmeter")
			Expect(v.String()).To(MatchYAML(`{}`))
		})
	})

	Context("No smokeMini", func() {
		initialConfigValues := `
upmeter:
  auth: {}
`
		f := HookExecutionConfigInit(initValuesString, initialConfigValues)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not store empty object when no fields left", func() {
			v := f.ConfigValuesGet("upmeter")
			Expect(v.String()).To(MatchYAML(`auth: {}`))
		})
	})
})
