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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: enable_cni ::", func() {

	cniMC := func(name string) string {
		return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: %s
spec:
  enabled: true
`, name)
	}

	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Cluster has no deckhouse MC", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("No cni is explicitly enabled"))
		})
	})

	Context("Cluster has deckhouse MC with cni module enabled", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cniMC("cni-cilium")))
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Enabled CNI from Deckhouse ModuleConfig: cni-cilium"))
		})
	})

	Context("Cluster has a few CNI ModuleConfigs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cniMC("cni-cilium") + cniMC("cni-flannel")))
			f.RunHook()
		})

		It("Throw the error", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).Should(ContainSubstring("more then one CNI enabled"))
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("false"))
		})
	})
})
