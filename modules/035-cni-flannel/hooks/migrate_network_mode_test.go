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

var _ = Describe("Modules :: cniFlannel :: hooks :: migrate_network_mode ::", func() {
	f := HookExecutionConfigInit(`{"cniFlannel":{"internal":{}}}`, ``)

	Context("Cluster with old value", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "host-gw")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})
		It("Must migrate value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ConfigValuesGet("cniFlannel.podNetworkMode").String()).To(BeEquivalentTo("HostGW"))
		})
	})

	Context("Cluster with new value", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})
		It("Value should not change", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ConfigValuesGet("cniFlannel.podNetworkMode").String()).To(BeEquivalentTo("VXLAN"))
		})
	})

})
