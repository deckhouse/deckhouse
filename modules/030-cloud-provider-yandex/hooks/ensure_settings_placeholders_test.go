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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: ensure_settings_placeholders ::", func() {
	const (
		// A cluster bootstrapped from a YandexClusterConfiguration alone: no ModuleConfig,
		// so nothing has put the settings sections into values yet.
		pccOnlyValues = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
`

		// A cluster whose ModuleConfig already supplies the settings — either written by
		// hand or produced by the v1 to v2 conversion.
		moduleConfigValues = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
  provider:
    parameters:
      cloudID: real-cloud-id
      folderID: real-folder-id
  nodes:
    parameters:
      layout: WithoutNAT
      nodeNetworkCIDR: 192.168.0.0/16
      sshPublicKey: ssh-rsa REAL
`
	)

	Context("with neither a ModuleConfig nor the sections in values", func() {
		f := HookExecutionConfigInit(pccOnlyValues, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("seeds both sections so later values patches pass schema validation", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.cloudID").String()).
				To(Equal(placeholderCloudID))
			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.folderID").String()).
				To(Equal(placeholderFolderID))

			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.layout").String()).
				To(Equal(placeholderLayout))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.nodeNetworkCIDR").String()).
				To(Equal(placeholderNodeNetworkCIDR))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.sshPublicKey").String()).
				To(Equal(placeholderSSHPublicKey))
		})
	})

	Context("with the sections already coming from a ModuleConfig", func() {
		f := HookExecutionConfigInit(moduleConfigValues, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("leaves the real settings untouched", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.cloudID").String()).
				To(Equal("real-cloud-id"))
			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.folderID").String()).
				To(Equal("real-folder-id"))

			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.layout").String()).
				To(Equal("WithoutNAT"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.nodeNetworkCIDR").String()).
				To(Equal("192.168.0.0/16"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.sshPublicKey").String()).
				To(Equal("ssh-rsa REAL"))
		})
	})

	// A ModuleConfig that fills only one of the two sections still leaves the other
	// missing, and a values patch would fail on it. Seeding has to be per-section.
	Context("with only the provider section coming from a ModuleConfig", func() {
		f := HookExecutionConfigInit(`
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
  provider:
    parameters:
      cloudID: real-cloud-id
      folderID: real-folder-id
`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("seeds nodes while keeping the real provider", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.cloudID").String()).
				To(Equal("real-cloud-id"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.layout").String()).
				To(Equal(placeholderLayout))
		})
	})
})
