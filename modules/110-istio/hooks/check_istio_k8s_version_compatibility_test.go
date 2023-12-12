/*
Copyright 2023 Flant JSC

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

var _ = Describe("Istio hooks :: check_istio_k8s_version_compatibility ::", func() {
	initValues := `
istio:
  internal:
    istioToK8sCompatibilityMap:
      "1.13": ["1.20", "1.21", "1.22", "1.23"]
      "1.16": ["1.22", "1.23", "1.24", "1.25"]
      "1.19": ["1.25", "1.26", "1.27", "1.28"]
`
	f := HookExecutionConfigInit(initValues, "")
	f.RegisterCRD("install.istio.io", "v1alpha1", "IstioOperator", true)

	Context("Unknown version of istio", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", []byte(`["1.12"]`))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.25.4")

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should return errors", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
		})
	})

	Context("istio version known, but incompatible with current k8s version", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", []byte(`["1.16"]`))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.28.4")

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should return errors", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
		})
	})

	Context(" the istio version is known, and it is compatible with the current version of k8s", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", []byte(`["1.19"]`))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.28.4")

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LogrusOutput.Contents()).To(HaveLen(0))
		})
	})
})
