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
	var mupResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "moduleupdatepolicy"}

	f.RegisterCRD(msResource.Group, msResource.Version, "ModuleSource", false)
	f.RegisterCRD(msResource.Group, mupResource.Version, "ModuleUpdatePolicy", false)

	Context("Without deckhouse-discovery secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should deploy the module source", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Exists()).To(BeTrue())
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.CA").String()).To(Equal(""))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.registry.scheme").String()).To(Equal("HTTPS"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal(""))
		})
	})

	Context("With different registry than registry.deckhouse.io", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modulesImages.registry.address", "registry.my-company.com")
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should deploy the module source", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Exists()).To(BeTrue())
		})
	})

	Context("With CE", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modulesImages.registry.path", "/deckhouse/ce")
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should deploy the module source", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Exists()).To(BeTrue())
		})
	})

	Context("With CA", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modulesImages.registry.CA", "--- BEGIN ...")
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should deploy the module source with CA", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.ca").String()).To(Equal("--- BEGIN ..."))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
		})
	})

	Context("With HTTP scheme", func() {
		BeforeEach(func() {
			f.ValuesSet("global.modulesImages.registry.scheme", "http")
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should deploy the module source with HTTP scheme", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.registry.scheme").String()).To(Equal("HTTP"))
		})
	})

	Context("With existing ModuleSource", func() {
		existingModuleSource := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse
spec:
  releaseChannel: EarlyAccess
  registry:
    repo: xxx
    dockerCfg: yyy
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(existingModuleSource))
			f.RunHook()
		})

		It("Should update the module source ", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal(""))

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Field("spec.moduleReleaseSelector").String()).To(Equal("{\"labelSelector\":{\"matchLabels\":{\"source\":\"deckhouse\"}}}"))
			Expect(mup.Field("spec.releaseChannel").String()).To(Equal("Stable"))
			Expect(mup.Field("spec.update.mode").String()).To(Equal("Auto"))
			Expect(mup.Field("spec.update.windows").String()).To(Equal(""))
		})
	})

	Context("With existing ModuleSource and releaseChannel not set", func() {
		existingModuleSource := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse
spec:
  registry:
    repo: xxx
    dockerCfg: yyy
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(existingModuleSource))
			f.RunHook()
		})

		It("Should update the module source ", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal(""))
		})
	})

	Context("With existing ModuleSource", func() {
		existingResources := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse
spec:
  releaseChannel: EarlyAccess
  registry:
    repo: xxx
    dockerCfg: yyy
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(existingResources))
			f.RunHook()
		})

		It("Should update the module source", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal(""))
		})
	})

	Context("With existing ModuleSource and releaseChannel not set", func() {
		existingResources := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse
spec:
  registry:
    repo: xxx
    dockerCfg: yyy
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(existingResources))
			f.RunHook()
		})

		It("Should update the module source", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal(""))
		})
	})

	Context("No ms but with discovery secret", func() {
		existingResources := `
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-discovery
  namespace: d8-system
data:
  releaseChannel: QWxwaGE= # Alpha
  updateSettings.json: eyJtb2RlIjogIk1hbnVhbCIsICJ3aW5kb3dzIjogW3siZnJvbSI6ICIwODowMCIsICJ0byI6ICIxMDowMCJ9XX0= # {"mode": "Manual", "windows": [{"from": "08:00", "to": "10:00"}]}
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(existingResources))
			f.RunHook()
		})

		It("Should update the module source", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal(""))

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Field("spec.moduleReleaseSelector").String()).To(Equal("{\"labelSelector\":{\"matchLabels\":{\"source\":\"deckhouse\"}}}"))
			Expect(mup.Field("spec.releaseChannel").String()).To(Equal("Alpha"))
			Expect(mup.Field("spec.update.mode").String()).To(Equal("Manual"))
			Expect(mup.Field("spec.update.windows").String()).To(Equal("[{\"days\":null,\"from\":\"08:00\",\"to\":\"10:00\"}]"))
		})
	})

	Context("With existing MUP", func() {
		existingResources := `
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-discovery
  namespace: d8-system
data:
  releaseChannel: QWxwaGE= # Alpha
  updateSettings.json: eyJtb2RlIjogIk1hbnVhbCIsICJ3aW5kb3dzIjogW3siZnJvbSI6ICIwODowMCIsICJ0byI6ICIxMDowMCJ9XX0= # {"mode": "Manual", "windows": [{"from": "08:00", "to": "10:00"}]}
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleUpdatePolicy
metadata:
  name: deckhouse
spec:
  releaseChannel: Stable
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        source: non-deckhouse
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(existingResources))
			f.RunHook()
		})

		It("Should update the module update policy", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal(""))

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Field("spec.moduleReleaseSelector").String()).To(Equal("{\"labelSelector\":{\"matchLabels\":{\"source\":\"deckhouse\"}}}"))
			Expect(mup.Field("spec.releaseChannel").String()).To(Equal("Alpha"))
			Expect(mup.Field("spec.update.mode").String()).To(Equal("Manual"))
			Expect(mup.Field("spec.update.windows").String()).To(Equal("[{\"days\":null,\"from\":\"08:00\",\"to\":\"10:00\"}]"))
		})
	})

	Context("With Unknown release channel", func() {
		existingResources := `
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-discovery
  namespace: d8-system
data:
  releaseChannel: VW5rbm93bg== # Unknown
  updateSettings.json: eyJtb2RlIjogIk1hbnVhbCIsICJ3aW5kb3dzIjogW3siZnJvbSI6ICIwODowMCIsICJ0byI6ICIxMDowMCJ9XX0= # {"mode": "Manual", "windows": [{"from": "08:00", "to": "10:00"}]}
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(existingResources))
			f.RunHook()
		})

		It("Should update the module update policy", func() {
			Expect(f).To(ExecuteSuccessfully())

			ms := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(ms.Field("spec.registry.repo").String()).To(Equal("registry.deckhouse.io/deckhouse/fe/modules"))
			Expect(ms.Field("spec.registry.dockerCfg").String()).To(Equal("PGI2ND4K"))
			Expect(ms.Field("spec.releaseChannel").String()).To(Equal(""))

			mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mup.Field("spec.moduleReleaseSelector").String()).To(Equal("{\"labelSelector\":{\"matchLabels\":{\"source\":\"deckhouse\"}}}"))
			Expect(mup.Field("spec.releaseChannel").String()).To(Equal("Stable"))
			Expect(mup.Field("spec.update.mode").String()).To(Equal("Manual"))
			Expect(mup.Field("spec.update.windows").String()).To(Equal("[{\"days\":null,\"from\":\"08:00\",\"to\":\"10:00\"}]"))
		})
	})
})
