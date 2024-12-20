/*
Copyright 2024 Flant JSC

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

var _ = Describe("Istio hooks :: delete_legacy_RBACs ::", func() {
	f := HookExecutionConfigInit(`{"global":{"discovery":{"clusterDomain":"cluster.flomaster"}},"istio":{"internal":{}}}`, "")

	Context("Legacy RBACs are deleted by platform", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app: istio-reader
    release: istio
  name: istio-reader-clusterrole-v1x16-d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app: istio-reader
    release: istio
  name: istio-reader-clusterrole-v1x16-d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app: istio-reader-custom
    release: istio-custom
  name: istio-reader-clusterrole-dont-delete
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    app: istiod
    release: istio
  name: istiod-v1x16
  namespace: d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    app: istiod
    release: istio
  name: istiod-v1x16
  namespace: d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    app: istiod
    release: istio
  name: istiod-binding-dont-delete
  namespace: istio-system
`))
			f.RunHook()
		})

		It("Legacy roles and rolebindings are removed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			cr := f.KubernetesGlobalResource("ClusterRole", "istio-reader-clusterrole-v1x16-d8-istio")
			Expect(cr.Exists()).To(BeFalse())

			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "istio-reader-clusterrole-v1x16")
			Expect(crb.Exists()).To(BeFalse())

			crNotForDeletion := f.KubernetesGlobalResource("ClusterRole", "istio-reader-clusterrole-dont-delete")
			Expect(crNotForDeletion.Exists()).To(BeTrue())

			r := f.KubernetesResource("Role", "d8-istio", "istiod-v1x16")
			Expect(r.Exists()).To(BeFalse())

			rb := f.KubernetesResource("RoleBinding", "d8-istio", "istiod-v1x16")
			Expect(rb.Exists()).To(BeFalse())

			rbNotForDeletion := f.KubernetesResource("RoleBinding", "istio-system", "istiod-binding-dont-delete")
			Expect(rbNotForDeletion.Exists()).To(BeTrue())
		})
	})
})
