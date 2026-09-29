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

var _ = Describe("Istio hooks :: retire_istio_operators ::", func() {
	const values = `
internal:
  versionMap:
    "1.21":
      revision: "v1x21"
    "1.25":
      revision: "v1x25"
  versionsToInstall: ["1.25"]
`

	const iopV1x21 = `
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x21
  namespace: d8-istio
spec:
  revision: v1x21
`

	const terminatingIopV1x21 = `
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x21
  namespace: d8-istio
  deletionTimestamp: "2026-09-29T09:18:00Z"
  finalizers:
  - istio-finalizer.install.istio.io
spec:
  revision: v1x21
`

	const keptTerminatingIopV1x21 = `
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x21
  namespace: d8-istio
  annotations:
    helm.sh/resource-policy: keep
  deletionTimestamp: "2026-09-29T09:18:00Z"
  finalizers:
  - istio-finalizer.install.istio.io
spec:
  revision: v1x21
`

	f := HookExecutionConfigInit(`{"istio":{"internal":{}}}`, "")
	f.RegisterCRD("install.istio.io", "v1alpha1", "IstioOperator", true)

	Context("IstioOperator of an installed version", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio", []byte(values))
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", []byte(`["1.21", "1.25"]`))
			f.BindingContexts.Set(f.KubeStateSet(iopV1x21))
			f.RunHook()
		})

		It("Should leave it alone", func() {
			Expect(f).To(ExecuteSuccessfully())
			iop := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21")
			Expect(iop.Exists()).To(BeTrue())
			Expect(iop.Field(`metadata.annotations.helm\.sh/resource-policy`).Exists()).To(BeFalse())
		})
	})

	Context("IstioOperator of a version no longer installed", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(iopV1x21))
			f.RunHook()
		})

		// The keep patch goes first: patching it after the delete would fail on
		// the missing object and fail the hook.
		It("Should keep it out of the release and delete it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("IstioOperator", "d8-istio", "v1x21").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Keeping IstioOperator of a version no longer installed out of the release"))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Deleting IstioOperator of a version no longer installed"))
		})
	})

	Context("IstioOperator of a version no longer installed that nelm already deleted", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(terminatingIopV1x21))
			f.RunHook()
		})

		It("Should only keep it out of the release", func() {
			Expect(f).To(ExecuteSuccessfully())
			iop := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21")
			Expect(iop.Exists()).To(BeTrue())
			Expect(iop.Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"))
			Expect(string(f.LoggerOutput.Contents())).ToNot(ContainSubstring("Deleting IstioOperator"))
		})
	})

	Context("IstioOperator of a version no longer installed that is already retired", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(keptTerminatingIopV1x21))
			f.RunHook()
		})

		It("Should do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("IstioOperator", "d8-istio", "v1x21").Exists()).To(BeTrue())
			Expect(string(f.LoggerOutput.Contents())).ToNot(ContainSubstring("IstioOperator of a version no longer installed"))
		})
	})

	Context("IstioOperator of an unknown revision", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio", []byte(values))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x9
  namespace: d8-istio
spec:
  revision: v1x9
`))
			f.RunHook()
		})

		It("Should leave it to discovery_operator_versions_to_install.go", func() {
			Expect(f).To(ExecuteSuccessfully())
			iop := f.KubernetesResource("IstioOperator", "d8-istio", "v1x9")
			Expect(iop.Exists()).To(BeTrue())
			Expect(iop.Field(`metadata.annotations.helm\.sh/resource-policy`).Exists()).To(BeFalse())
		})
	})
})
