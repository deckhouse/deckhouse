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

var _ = Describe("Modules :: openvpn :: hooks :: set_search_domains ", func() {
	const globalClusterDomain = "test.test"

	f := HookExecutionConfigInit(
		`{ "global": {"discovery":{}}, "openvpn":{"internal":{"auth": {}}} }`,
		`{"openvpn":{}}`)
	Context("openvpn.pushToClientSearchDomains is not set", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet(clusterDomainGlobalPath, globalClusterDomain)
			f.RunHook()
		})

		It("domain should be set to global value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(clusterDomainsInternalValuesPath + ".0").String()).Should(Equal(globalClusterDomain))
		})
	})

	Context("openvpn.pushToClientSearchDomains is set in configuration", func() {
		const userDefinedDomain = "example.example"
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet(clusterDomainGlobalPath, globalClusterDomain)
			f.ConfigValuesSet("openvpn.pushToClientSearchDomains", []string{userDefinedDomain})
			f.RunHook()
		})

		It("domain should be set to user defined value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(clusterDomainsInternalValuesPath + ".0").String()).Should(Equal(userDefinedDomain))
		})
	})
})
