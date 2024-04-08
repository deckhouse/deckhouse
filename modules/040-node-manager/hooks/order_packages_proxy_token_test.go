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

var _ = FDescribe("Modules :: node-group :: hooks :: order_packages_proxy_token ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must not fail, new token must have been generated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.packagesProxyToken").String()).To(Equal("{}"))
		})
	})
})
