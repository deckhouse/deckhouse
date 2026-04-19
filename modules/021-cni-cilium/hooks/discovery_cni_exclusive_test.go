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
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: cni-cilium :: hooks :: set-exclusive", func() {
	f := HookExecutionConfigInit(`{
  "cniCilium": {
    "exclusiveCNIPlugin": true,
    "internal": {}
  }
}`, `{}`)

	Context("empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should set internal.exclusiveCNIPlugin from cniCilium.exclusiveCNIPlugin", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.exclusiveCNIPlugin").Bool()).To(BeTrue())
		})
	})

	Context("DaemonSet istio-cni-node exists", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: istio-cni-node
  namespace: d8-istio
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should set internal.exclusiveCNIPlugin=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.exclusiveCNIPlugin").Bool()).To(BeFalse())
		})
	})

	Context("DaemonSet agent (sdn) exists", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: agent
  namespace: d8-sdn
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should set internal.exclusiveCNIPlugin=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.exclusiveCNIPlugin").Bool()).To(BeFalse())
		})
	})

	Context("both istio-cni and sdn daemonsets exist", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: istio-cni-node
  namespace: d8-istio
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: agent
  namespace: d8-sdn
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should set internal.exclusiveCNIPlugin=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.exclusiveCNIPlugin").Bool()).To(BeFalse())
		})
	})

	Context("empty cluster and exclusiveCNIPlugin is false", func() {
		f2 := HookExecutionConfigInit(`{"cniCilium":{"exclusiveCNIPlugin":false,"internal":{}}}`, `{}`)

		BeforeEach(func() {
			f2.KubeStateSet(``)
			f2.BindingContexts.Set(f2.GenerateBeforeHelmContext())
			f2.RunHook()
		})

		It("should set internal.exclusiveCNIPlugin to false", func() {
			Expect(f2).To(ExecuteSuccessfully())
			Expect(f2.ValuesGet("cniCilium.internal.exclusiveCNIPlugin").Bool()).To(BeFalse())
		})
	})
})
