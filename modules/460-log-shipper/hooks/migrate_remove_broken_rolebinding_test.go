/*
Copyright 2021 Flant JSC

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

var _ = Describe("Log Shipper :: remove rolebinding ::", func() {
	f := HookExecutionConfigInit(`{ "logShipper": { "internal": {"activated": false }}}`, ``)

	Context("Remove rolebinding", func() {
		BeforeEach(func() {
			f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: log-shipper
  namespace: d8-log-shipper
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: d8:log-shipper
subjects:
- kind: ServiceAccount
  name: log-shipper
  namespace: d8-log-shipper
`, 1)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("RoleBinding", "d8-log-shipper", "log-shipper").Exists()).To(BeFalse())
		})

	})

	Context("Normal rolebinding", func() {
		BeforeEach(func() {
			f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: log-shipper
  namespace: d8-log-shipper
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: log-shipper
subjects:
- kind: ServiceAccount
  name: log-shipper
  namespace: d8-log-shipper
`, 1)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("RoleBinding", "d8-log-shipper", "log-shipper").Exists()).To(BeTrue())
		})

	})
})
