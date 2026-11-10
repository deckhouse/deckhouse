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

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: create_module_config ::", func() {
	const (
		emptyValues = `
global:
  discovery: {}
cloudProviderDvp:
  internal: {}
  nodes: {}
  provider: {}
`
	)

	assertMinimalModuleConfig := func(f *HookExecutionConfig) {
		moduleConfig := f.KubernetesGlobalResource("ModuleConfig", dvpModuleName)
		Expect(moduleConfig.Exists()).To(BeTrue())
		Expect(moduleConfig.Field("apiVersion").String()).To(Equal(moduleConfigAPIVersion))
		Expect(moduleConfig.Field("kind").String()).To(Equal("ModuleConfig"))
		Expect(moduleConfig.Field("metadata.name").String()).To(Equal(dvpModuleName))
		Expect(moduleConfig.Field("spec").String()).To(MatchYAML(`
enabled: true
version: 1
`))
		Expect(moduleConfig.Field("spec.version").Exists()).To(BeTrue())
		Expect(moduleConfig.Field("spec.settings").Exists()).To(BeFalse())
	}

	Context("when ModuleConfig/cloud-provider-dvp is absent", func() {
		f := HookExecutionConfigInit(emptyValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should create minimal enabled ModuleConfig", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertMinimalModuleConfig(f)
		})
	})

	Context("when ModuleConfig/cloud-provider-dvp already exists", func() {
		f := HookExecutionConfigInit(emptyValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

		existingModuleConfig := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: false
  version: 2
  maintenance: NoResourceReconciliation
  settings:
    provider:
      parameters:
        namespace: cloud-provider01
    nodes:
      parameters:
        layout: Standard
`

		expectedModuleConfig := f.ApplyCRDefaults(existingModuleConfig)

		BeforeEach(func() {
			f.KubeStateSet(existingModuleConfig)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should preserve existing ModuleConfig byte-for-byte semantically", func() {
			Expect(f).To(ExecuteSuccessfully())

			moduleConfig := f.KubernetesGlobalResource("ModuleConfig", dvpModuleName)
			Expect(moduleConfig.Exists()).To(BeTrue())
			Expect(moduleConfig.ToYaml()).To(MatchYAML(expectedModuleConfig))
		})
	})

	Context("when legacy ProviderClusterConfiguration Secret exists and ModuleConfig is absent", func() {
		f := HookExecutionConfigInit(emptyValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

		legacyPCCSecret := `
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  cloud-provider-cluster-configuration.yaml: ZHVtbXk=
`

		BeforeEach(func() {
			f.KubeStateSet(legacyPCCSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should create only minimal ModuleConfig and no migration artifacts", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertMinimalModuleConfig(f)

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", dvpMigrationResourcesName)
			Expect(migrationSecret.Exists()).To(BeFalse())

			migrationCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", dvpMigrationConfigMapName)
			Expect(migrationCM.Exists()).To(BeFalse())
		})
	})
})
