// Copyright 2023 Flant JSC
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
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: external module manager :: hooks :: create deckhouse module source ::", func() {
	initValues := `
global:
  modulesImages:
    registry:
      address: registry.deckhouse.io
      base: registry.deckhouse.io/deckhouse/fe
      dockercfg: "PGI2ND4K"
`

	f := HookExecutionConfigInit(initValues, `{}`)

	var msResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "modulesources"}
	f.RegisterCRD(msResource.Group, msResource.Version, "ModuleSource", false)
	f.RegisterCRD(msResource.Group, msResource.Version, "ModuleUpdatePolicy", false)

	const (
		discoverySecret = `
---
apiVersion: v1
data:
  bundle: RGVmYXVsdA==
  releaseChannel: QWxwaGE=
kind: Secret
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse-discovery
  namespace: d8-system
type: Opaque
`
	)

	Context("Without deckhouse-discovery secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should deploy the module source and update policy", func() {
			Expect(f).To(ExecuteSuccessfully())

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Exists()).To(BeTrue())
			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Exists()).To(BeTrue())
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.CA").String()).To(Equal(""))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal(""))
		})
	})

	Context("With deckhouse-discovery secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(discoverySecret))
			f.RunHook()
		})

		It("Should deploy the module source and updatep policy", func() {
			Expect(f).To(ExecuteSuccessfully())

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Exists()).To(BeTrue())
			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Exists()).To(BeTrue())
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.CA").String()).To(Equal(""))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal("alpha"))
		})
	})

	Context("With different registry than registry.deckhouse.io", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modulesImages.registry.address", "registry.my-company.com")
			f.BindingContexts.Set(f.KubeStateSet(discoverySecret))
			f.RunHook()
		})

		It("Should deploy the module source and update policy", func() {
			Expect(f).To(ExecuteSuccessfully())

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Exists()).To(BeTrue())
			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Exists()).To(BeTrue())
		})
	})

	Context("With CE", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modulesImages.registry.path", "/deckhouse/ce")
			f.BindingContexts.Set(f.KubeStateSet(discoverySecret))
			f.RunHook()
		})

		It("Should deploy the module source and update policy", func() {
			Expect(f).To(ExecuteSuccessfully())

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Exists()).To(BeTrue())
			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Exists()).To(BeTrue())
		})
	})

	Context("With CA", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modulesImages.registry.CA", "--- BEGIN ...")
			f.BindingContexts.Set(f.KubeStateSet(discoverySecret))
			f.RunHook()
		})

		It("Should deploy the module source with CA and update policy", func() {
			Expect(f).To(ExecuteSuccessfully())

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Exists()).To(BeTrue())
			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.ca").String()).To(Equal("--- BEGIN ..."))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal("alpha"))
		})
	})

	Context("With existed resource", func() {
		existedModuleSource := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse
spec:
  releaseChannel: test
  registry:
    repo: xxx
    dockerCfg: yyy
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(discoverySecret + existedModuleSource))
			f.RunHook()
		})

		It("Should update the module source and update policy", func() {
			Expect(f).To(ExecuteSuccessfully())

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Exists()).To(BeTrue())
			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal("test"))
		})
	})

})
