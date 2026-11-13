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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func storageFoundationModule(version string) string {
	return fmt.Sprintf(`
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: storage-foundation
properties:
  version: %q
status:
  phase: Ready
`, version)
}

var _ = Describe("Modules :: multitenancy-manager :: hooks :: storage_foundation_storageclass_grant ::", func() {
	const initValues = `
global:
  discovery: {}
multitenancyManager:
  internal:
    admissionWebhookCert: {}
`

	f := HookExecutionConfigInit(initValues, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Module", false)

	ships := func() bool {
		return f.ValuesGet(storageFoundationShipsStorageClassKey).Bool()
	}

	for _, tc := range []struct {
		version string
		ships   bool
	}{
		{"v1.0.2", false},
		{"v0.9.12", false},
		{"v1.0.3", true},
		{"v1.0.3-rc.1", true},
		{"v1.0.4", true},
		{"v1.1.0", true},
		{"main", true},
		{"mr248", true},
		{"", false},
	} {
		Context(fmt.Sprintf("storage-foundation is enabled at version %q", tc.version), func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", []byte(`["storage-foundation", "multitenancy-manager"]`))
				f.BindingContexts.Set(f.KubeStateSet(storageFoundationModule(tc.version)))
				f.RunHook()
			})

			It(fmt.Sprintf("sets the value to %t", tc.ships), func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(ships()).To(Equal(tc.ships))
			})
		})
	}

	Context("storage-foundation is enabled at a new version, before helm", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["storage-foundation", "multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(storageFoundationModule("v1.0.4")), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("sets the value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(ships()).To(BeTrue())
		})
	})

	Context("storage-foundation is disabled at a new version", func() {
		BeforeEach(func() {
			f.ValuesSet(storageFoundationShipsStorageClassKey, true)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(storageFoundationModule("v1.0.4")), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("clears the value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(ships()).To(BeFalse())
		})
	})

	Context("storage-foundation is enabled, but there is no Module object", func() {
		BeforeEach(func() {
			f.ValuesSet(storageFoundationShipsStorageClassKey, true)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["storage-foundation", "multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(""), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("clears the value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(ships()).To(BeFalse())
		})
	})

	Context("storage-foundation moves from an old version to a new one", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["storage-foundation", "multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(storageFoundationModule("v1.0.2")))
			f.RunHook()
			Expect(ships()).To(BeFalse())
			f.BindingContexts.Set(f.KubeStateSet(storageFoundationModule("v1.0.4")))
			f.RunHook()
		})

		It("sets the value on the event", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(ships()).To(BeTrue())
		})
	})
})
