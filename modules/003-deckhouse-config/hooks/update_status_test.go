/*
Copyright 2022 Flant JSC

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

	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/module-manager/test/mock"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var moduleConfigYaml = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-one
spec:
  version: 1
  settings:
    param1: val1
`

var enabledModuleConfigYaml = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-one
spec:
  version: 1
  settings:
    param1: val1
  enabled: true
`

var _ = Describe("Module :: deckhouse-config :: hooks :: update ModuleConfig status ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Known module enabled", func() {

		BeforeEach(func() {
			f.KubeStateSet(moduleConfigYaml)

			mm := mock.NewModuleManager(
				mock.NewModule("module-one", nil, mock.EnabledByScript),
			)
			d8config.InitService(mm)

			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should be enabled in status", func() {
			Expect(f).To(ExecuteSuccessfully())

			promCfg := f.KubernetesGlobalResource("ModuleConfig", "module-one")
			Expect(promCfg.Field("status.state").String()).To(Equal("Enabled"), "should update status")
		})
	})

	Context("Enabled by bundle, disabled by enabled script", func() {
		BeforeEach(func() {
			f.KubeStateSet(enabledModuleConfigYaml)

			// status.go doesn't validate values, mock is sufficient here.
			mm := mock.NewModuleManager(
				mock.NewModule("module-one", mock.EnabledByBundle, mock.DisabledByScript),
			)
			d8config.InitService(mm)

			f.BindingContexts.Set(f.GenerateScheduleContext("*/15 * * * * *"))
			f.RunHook()
		})

		It("Should not be enabled in status", func() {
			Expect(f).To(ExecuteSuccessfully())

			promCfg := f.KubernetesGlobalResource("ModuleConfig", "module-one")
			Expect(promCfg.Field("status.state").String()).To(ContainSubstring("Disabled"), "should be disabled by script, got %s", promCfg.Field("status").String())
		})
	})
})
