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

var _ = Describe("Istio hooks :: cleanup_retired_istio_operator ::", func() {
	f := HookExecutionConfigInit(`{"istio":{}}`, "")
	f.RegisterCRD("install.istio.io", "v1alpha1", "IstioOperator", true)

	Context("A retired and a supported IstioOperator exist", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.KubeStateSet(`
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x21
  namespace: d8-istio
spec:
  revision: v1x21
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x25
  namespace: d8-istio
spec:
  revision: v1x25
`)
			f.RunGoHook()
		})

		It("deletes the retired resource and waits before Helm", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("waiting for retired Istio revision v1x21 to be cleaned up"))
			Expect(f.KubernetesResource("IstioOperator", "d8-istio", "v1x21").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("IstioOperator", "d8-istio", "v1x25").Exists()).To(BeTrue())
		})
	})

	Context("A retired IstioOperator is already terminating", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.KubeStateSet(`
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: v1x21
  namespace: d8-istio
  deletionTimestamp: "2026-08-14T00:00:00Z"
  finalizers:
  - istio-finalizer.install.istio.io
spec:
  revision: v1x21
`)
			f.RunGoHook()
		})

		It("keeps waiting without stripping the finalizer", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("waiting for retired Istio revision v1x21 to be cleaned up"))
			resource := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21")
			Expect(resource.Exists()).To(BeTrue())
			Expect(resource.Field("metadata.finalizers.0").String()).To(Equal("istio-finalizer.install.istio.io"))
		})
	})

	Context("No retired IstioOperator exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.KubeStateSet(``)
			f.RunGoHook()
		})

		It("allows Helm to proceed", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})
