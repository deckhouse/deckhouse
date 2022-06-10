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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: calculate_resources_requests", func() {

	f := HookExecutionConfigInit(`{"global": {"enabledModules": []}}`, `{}`)

	Context("Cluster without supported modules", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Hook should not enable snapshot-controller, because no supported modules are enabled", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("snapshotControllerCrdEnabled").Bool()).ToNot(BeTrue())
			Expect(f.ValuesGet("snapshotControllerEnabled").Bool()).ToNot(BeTrue())
		})
	})

	Context("Cluster with linstor enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("global.enabledModules", []string{"linstor"})
			f.RunHook()
		})

		It("Hook should enable snapshot-controller, because linstor is one of supported modules", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("snapshotControllerCrdEnabled").Bool()).To(BeTrue())
			Expect(f.ValuesGet("snapshotControllerEnabled").Bool()).To(BeTrue())
		})
	})

})
