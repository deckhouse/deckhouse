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
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "sigs.k8s.io/yaml"

	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	module_manager "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/module-manager"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const testGlobalHooksDir = "testdata/test-startup-sync/global-hooks"
const testModulesDir = "testdata/test-startup-sync/modules"

var _ = Describe("Global hooks :: deckhouse-config :: migrate", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	// Emulate ensure_crd hook.
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	BeforeEach(func() {
		// Load addon-operator with 3 modules: deckhouse, cert-manager and prometheus.
		mm, initErr := module_manager.InitBasic(testGlobalHooksDir, testModulesDir)
		d8config.InitService(mm)
		Expect(initErr).ShouldNot(HaveOccurred(), "should init module manager: %s", initErr)
	})

	Context("Phase 1. Migrate deployment/deckhouse to generated ConfigMap", func() {

		BeforeEach(func() {
			// Prepare non-migrated deckhouse.
			_ = os.Setenv("ADDON_OPERATOR_CONFIG_MAP", "deckhouse")
			depl := d8Deployment(d8config.DeckhouseConfigMapName)
			cm := d8ConfigMap(d8config.DeckhouseConfigMapName, `
global: |
  param1: val1
  param2: val2
deckhouse: |
  p1: val1
certManager: |
  param1: val1
certManagerEnabled: "true"
`)
			f.KubeStateSet(depl + cm)

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("should create generated cm and update deployment", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Check generated ConfigMap.
			generatedCM := f.KubernetesResource("ConfigMap", "d8-system", d8config.GeneratedConfigMapName)
			Expect(generatedCM.Exists()).Should(BeTrue())
			annotationJSON := fmt.Sprintf(`{"%s":"true"}`, migrationAnnotation)
			Expect(generatedCM.Field("metadata.annotations").String()).Should(MatchJSON(annotationJSON))
			Expect(generatedCM.Field("data.global").String()).Should(ContainSubstring("param1: val1"))
			Expect(generatedCM.Field("data.deckhouse").Exists()).Should(BeTrue())
			Expect(generatedCM.Field("data.deckhouse").String()).Should(ContainSubstring("p1: val1"))
			Expect(generatedCM.Field("data.certManager").Exists()).Should(BeTrue())
			Expect(generatedCM.Field("data.certManager").String()).Should(ContainSubstring("param1: val1"))
			Expect(generatedCM.Field("data.certManagerEnabled").Exists()).Should(BeTrue())
			Expect(generatedCM.Field("data.certManagerEnabled").String()).Should(Equal("true"))

			// Test deployment
			deckhouseDeploy := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(deckhouseDeploy.Exists()).Should(BeTrue())
			Expect(deckhouseDeploy.Field("spec.template.spec.containers.0.env.0.value").String()).Should(Equal(d8config.GeneratedConfigMapName), "should update deploy/deckhouse to use generated ConfigMap")
		})
	})

	Context("Phase 2. Migrate to ModuleConfig objects", func() {

		Context("giving valid ConfigMap", func() {
			// Register 2 conversions to test conversion chains.
			var _ = conversion.RegisterFunc("deckhouse", 1, 2, func(settings *conversion.Settings) error {
				return nil
			})
			var _ = conversion.RegisterFunc("deckhouse", 2, 3, func(settings *conversion.Settings) error {
				return nil
			})

			BeforeEach(func() {
				// Emulate migrated Deployment/deckhouse.
				_ = os.Setenv("ADDON_OPERATOR_CONFIG_MAP", d8config.GeneratedConfigMapName)
				cm := d8ConfigMap(d8config.GeneratedConfigMapName, `
global: |
  paramStr: "val1"
  paramNum: 100
deckhouse: |
  paramStr: "val1"
certManager: |
  paramStr: "val1"
certManagerEnabled: "false"
unknownModule: |
  paramBool: true
`, migrationAnnotation)
				f.KubeStateSet(cm)

				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Should run successfully and reach phase 2", func() {
				Expect(f).To(ExecuteSuccessfully())

				// Ensure phase 2.
				Expect(f.LogrusOutput).Should(gbytes.Say("Migrate Configmap to ModuleConfig"), "should run phase 2")
			})

			It("Should drop annotation from generated ConfigMap", func() {
				// Check generated ConfigMap.
				generatedCM := f.KubernetesResource("ConfigMap", "d8-system", d8config.GeneratedConfigMapName)
				Expect(generatedCM.Exists()).Should(BeTrue())

				annotations := generatedCM.Field("metadata.annotations")
				Expect(annotations.Map()).Should(HaveLen(0), "should delete annotation, got %+v", annotations.String())
			})

			It("Should create ModuleConfig objects", func() {
				cfg := f.KubernetesGlobalResource("ModuleConfig", "global")
				Expect(cfg.Exists()).Should(BeTrue(), "should create ModuleConfig/global")
				Expect(cfg.Field("spec.enabled").Exists()).Should(BeFalse())

				cfg = f.KubernetesGlobalResource("ModuleConfig", "deckhouse")
				Expect(cfg.Exists()).Should(BeTrue(), "should create ModuleConfig/deckhouse")
				Expect(cfg.Field("spec.enabled").Exists()).Should(BeFalse())
				// See registers at the Context beginning.
				Expect(cfg.Field("spec.version").Int()).Should(BeEquivalentTo(3), "should migrate to latest registered version")

				cfg = f.KubernetesGlobalResource("ModuleConfig", "cert-manager")
				Expect(cfg.Exists()).Should(BeTrue(), "should create ModuleConfig/cert-manager")
				Expect(cfg.Field("spec.enabled").Bool()).Should(BeFalse(), "cert-manager should be disabled")

				cfg = f.KubernetesGlobalResource("ModuleConfig", "unknown-module")
				Expect(cfg.Exists()).Should(BeFalse(), "should not create ModuleConfig for unknown module")
			})
		})

		Context("giving invalid ConfigMap", func() {
			BeforeEach(func() {
				// Emulate migrated Deployment/deckhouse.
				_ = os.Setenv("ADDON_OPERATOR_CONFIG_MAP", d8config.GeneratedConfigMapName)
				cm := d8ConfigMap(d8config.GeneratedConfigMapName, `
global: |
  paramStr: 100
  paramNum: "100"
deckhouse: |
  paramStr: 100
  paramNum: "100"
`, migrationAnnotation)
				f.KubeStateSet(cm)

				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Should get validation error", func() {
				Expect(f).ToNot(ExecuteSuccessfully(), "should fail on invalid values")
			})
		})
	})
})

var _ = Describe("Global hooks :: deckhouse-config :: sync", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	// Emulate ensure_crd hook.
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	BeforeEach(func() {
		// Load addon-operator with 3 modules: deckhouse, cert-manager and prometheus.
		mm, initErr := module_manager.InitBasic(testGlobalHooksDir, testModulesDir)
		d8config.InitService(mm)
		Expect(initErr).ShouldNot(HaveOccurred(), "should init module manager: %s", initErr)
	})

	Context("Giving absent ConfigMap", func() {
		BeforeEach(func() {
			// See openapi schemas in testdata/test-deckhouse directory.
			existingModuleConfigs := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    paramStr: val1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    paramStr: Debug
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: unknown-module-name
spec:
  version: 1
  settings:
    param1: val1
`

			// Emulate migrated Deployment/deckhouse with absent ConfigMap.
			_ = os.Setenv("ADDON_OPERATOR_CONFIG_MAP", d8config.GeneratedConfigMapName)

			f.KubeStateSet(existingModuleConfigs)

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("should create ConfigMap from ModuleConfig objects", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Ensure phase 3.
			Expect(f.LogrusOutput).Should(gbytes.Say("Sync ModuleConfig resources"))

			gcm := f.KubernetesResource("ConfigMap", "d8-system", d8config.GeneratedConfigMapName)
			Expect(gcm.Exists()).Should(BeTrue(), "should create ConfigMap from ModuleConfig")
			data := gcm.Field("data").Map()
			Expect(data).ShouldNot(HaveLen(0), "generated ConfigMap should have sections for known ModuleConfig objects")
			for moduleName, vals := range data {
				switch moduleName {
				case "global":
					Expect(vals.String()).Should(Equal("paramStr: val1\n"), "should update 'global' section")
				case "deckhouse":
					Expect(vals.String()).Should(Equal("paramStr: Debug\n"), "should update 'deckhouse' section")
				default:
					Expect(data).ShouldNot(HaveKey(moduleName), "ConfigMap should not have module sections for unknown modules, got '%s'", moduleName)
				}
			}
		})
	})

	Context("Giving ConfigMap with some sections", func() {
		BeforeEach(func() {
			// See openapi schemas in testdata/test-deckhouse directory.
			existingConfigs := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    paramStr: val1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    paramStr: val1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 1
  settings:
    paramNum: 10
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: unknown-module-name
spec:
  version: 1
  settings:
    param1: val1
`

			// Emulate migrated Deployment/deckhouse.
			_ = os.Setenv("ADDON_OPERATOR_CONFIG_MAP", d8config.GeneratedConfigMapName)
			cm := d8ConfigMap(d8config.GeneratedConfigMapName, `
global: |
  param2: val4
deckhouse: |
  logLevel: Info
`)
			f.KubeStateSet(existingConfigs + cm)

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("should create new sections and update values for existing", func() {
			Expect(f).To(ExecuteSuccessfully())

			gcm := f.KubernetesResource("ConfigMap", "d8-system", d8config.GeneratedConfigMapName)
			Expect(gcm.Exists()).Should(BeTrue(), "should not delete generated ConfigMap")

			data := gcm.Field("data").Map()
			Expect(data).ShouldNot(HaveLen(0), "generated ConfigMap should have sections for known ModuleConfig objects")
			for moduleName, vals := range data {
				switch moduleName {
				case "global":
					Expect(vals.String()).Should(Equal("paramStr: val1\n"))
				case "deckhouse":
					Expect(vals.String()).Should(Equal("paramStr: val1\n"))
				case "prometheus":
					Expect(vals.String()).Should(Equal("paramNum: 10\n"), "should create new module section for %s", moduleName)
				default:
					Expect(data).ShouldNot(HaveKey(moduleName), "ConfigMap should not have module sections for unknown modules")
				}
			}
		})
	})

	Context("Giving ModuleConfig with invalid values", func() {
		BeforeEach(func() {
			existingConfigs := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    paramStr: 100
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    logLevel: Debug
`

			// Emulate migrated Deployment/deckhouse.
			_ = os.Setenv("ADDON_OPERATOR_CONFIG_MAP", d8config.GeneratedConfigMapName)
			cm := d8ConfigMap(d8config.GeneratedConfigMapName, `
global: |
  param2: val4
deckhouse: |
  logLevel: Info
`)
			f.KubeStateSet(existingConfigs + cm)

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("should fail on validating ModuleConfig spec.settings", func() {
			Expect(f).ToNot(ExecuteSuccessfully(), "should fail on invalid values in ModuleConfig object")
		})
	})
})

func d8ConfigMap(cmName string, values string, annotation ...string) string {
	var data map[string]string
	_ = k8syaml.Unmarshal([]byte(values), &data)

	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: "d8-system",
		},
		Data: data,
	}
	if len(annotation) > 0 {
		cm.SetAnnotations(map[string]string{
			annotation[0]: "true",
		})
	}
	cmDump, _ := k8syaml.Marshal(cm)
	return "---\n" + string(cmDump)
}

func d8Deployment(cmName string) string {
	return fmt.Sprintf(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        env:
        - name: ADDON_OPERATOR_CONFIG_MAP
          value: %s
        - name: LOG_LEVEL
          value: Info
`, cmName)
}
