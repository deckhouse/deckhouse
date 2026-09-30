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
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: istio :: hooks :: ensure_crds_istio ::", func() {
	f := HookExecutionConfigInit(`{ "istio": { "internal": { } } }`, `{"istio":{}}`)
	realMinCRDBundleVersion := minCRDBundleVersion

	// the test bundles (0.990-0.992) are older than the real floor
	BeforeEach(func() {
		minCRDBundleVersion = semver.MustParse("0.0")
	})

	AfterEach(func() {
		minCRDBundleVersion = realMinCRDBundleVersion
	})

	Context("Empty cluster, no globalVersion in values", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("istio.internal.globalVersion is not discovered by discovery_versions_to_install.go yet"))
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeFalse())
		})
	})

	Context("Only globalVersion in values", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v0.991 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.991"))
		})
	})

	Context("globalVersion in values and additionalVersion older than global", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			f.ConfigValuesSetFromYaml("istio.additionalVersions", []byte(`["0.990"]`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v0.991 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.991"))
		})
	})

	Context("globalVersion in values and additionalVersion newer than global", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			f.ConfigValuesSetFromYaml("istio.additionalVersions", []byte(`["0.992"]`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v0.992 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.992"))
		})
	})

	Context("globalVersion 1.27 installs CRDs from 1.27 directory", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "1.27")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})

var _ = Describe("Modules :: istio :: hooks :: ensure_crds_istio :: real CRD bundles ::", func() {
	f := HookExecutionConfigInit(`{ "istio": { "internal": { } } }`, `{"istio":{}}`)

	// ServiceEntry CRD as installed by the 1.27 bundle
	const serviceEntryCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: serviceentries.networking.istio.io
spec:
  group: networking.istio.io
  names:
    kind: ServiceEntry
    listKind: ServiceEntryList
    plural: serviceentries
    singular: serviceentry
  scope: Namespaced
  versions:
  - name: v1
    served: true
    storage: true
    schema: &schema
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
  - name: v1alpha3
    served: true
    storage: false
    schema: *schema
  - name: v1beta1
    served: true
    storage: false
    schema: *schema
status:
  storedVersions:
  - v1alpha3
  - v1beta1
  - v1
`

	Context("Empty cluster, globalVersion 1.21", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "1.21")
			f.ValuesSetFromYaml("istio.internal.versionMap", []byte(`{"1.21": {"supportsOperator": true}, "1.25": {"supportsOperator": true}}`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRDs from the 1.25 bundle should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			serviceEntry := f.KubernetesGlobalResource("CustomResourceDefinition", "serviceentries.networking.istio.io")
			Expect(serviceEntry.Field("spec.versions.#.name").String()).To(MatchJSON(`["v1","v1alpha3","v1beta1"]`))
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "istios.sailoperator.io").Exists()).To(BeTrue())
		})
	})

	Context("CRDs of a newer bundle in the cluster, globalVersion goes back to 1.21", func() {
		BeforeEach(func() {
			f.KubeStateSet(serviceEntryCRD)
			f.ValuesSet("istio.internal.globalVersion", "1.21")
			f.ValuesSetFromYaml("istio.internal.versionMap", []byte(`{"1.21": {"supportsOperator": true}, "1.25": {"supportsOperator": true}}`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, the v1 API must stay in the CRD", func() {
			Expect(f).To(ExecuteSuccessfully())
			serviceEntry := f.KubernetesGlobalResource("CustomResourceDefinition", "serviceentries.networking.istio.io")
			Expect(serviceEntry.Field("spec.versions.#.name").String()).To(MatchJSON(`["v1","v1alpha3","v1beta1"]`))
			Expect(serviceEntry.Field("status.storedVersions").String()).To(MatchJSON(`["v1alpha3","v1beta1","v1"]`))
		})
	})
})
