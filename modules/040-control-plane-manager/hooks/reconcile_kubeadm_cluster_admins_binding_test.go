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

var _ = Describe("Modules :: control-plane-manager :: hooks :: reconcile_kubeadm_cluster_admins_binding ::", func() {
	const (
		valuesUserAuthzOff = `{"global": {"enabledModules": []}, "controlPlaneManager":{"internal": {}}}`
		valuesUserAuthzOn  = `{"global": {"enabledModules": ["user-authz"]}, "controlPlaneManager":{"internal": {}}}`

		crbCurrentClusterAdmin = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubeadm:cluster-admins
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: Group
  name: kubeadm:cluster-admins
`

		crbCurrentUserAuthzClusterAdmin = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubeadm:cluster-admins
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: user-authz:cluster-admin
subjects:
- kind: Group
  name: kubeadm:cluster-admins
`
	)

	expectDesiredCRB := func(f *HookExecutionConfig, roleName string) {
		crb := f.KubernetesGlobalResource("ClusterRoleBinding", "kubeadm:cluster-admins")
		Expect(crb.Exists()).To(BeTrue(), "ClusterRoleBinding kubeadm:cluster-admins must exist after reconcile")
		Expect(crb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
		Expect(crb.Field("roleRef.apiGroup").String()).To(Equal("rbac.authorization.k8s.io"))
		Expect(crb.Field("roleRef.name").String()).To(Equal(roleName))
		Expect(crb.Field("subjects.0.kind").String()).To(Equal("Group"))
		Expect(crb.Field("subjects.0.name").String()).To(Equal("kubeadm:cluster-admins"))
	}

	Context("user-authz disabled", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOff, "")

		Context("ClusterRoleBinding kubeadm:cluster-admins is missing", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				f.RunHook()
			})

			It("creates the binding pointing to cluster-admin (kubeadm-default wildcard)", func() {
				Expect(f).To(ExecuteSuccessfully())
				expectDesiredCRB(f, "cluster-admin")
			})
		})

		Context("ClusterRoleBinding kubeadm:cluster-admins already targets cluster-admin", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(crbCurrentClusterAdmin))
				f.RunHook()
			})

			It("is a no-op (binding stays pointed at cluster-admin)", func() {
				Expect(f).To(ExecuteSuccessfully())
				expectDesiredCRB(f, "cluster-admin")
			})
		})

		Context("ClusterRoleBinding kubeadm:cluster-admins targets user-authz:cluster-admin", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(crbCurrentUserAuthzClusterAdmin))
				f.RunHook()
			})

			It("rebinds back to cluster-admin (Delete + Create — roleRef is immutable)", func() {
				Expect(f).To(ExecuteSuccessfully())
				expectDesiredCRB(f, "cluster-admin")
			})
		})
	})

	Context("user-authz enabled", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOn, "")

		Context("ClusterRoleBinding kubeadm:cluster-admins is missing", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				f.RunHook()
			})

			It("creates the binding pointing to user-authz:cluster-admin (granular)", func() {
				Expect(f).To(ExecuteSuccessfully())
				expectDesiredCRB(f, "user-authz:cluster-admin")
			})
		})

		Context("ClusterRoleBinding kubeadm:cluster-admins targets cluster-admin", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(crbCurrentClusterAdmin))
				f.RunHook()
			})

			It("rebinds to user-authz:cluster-admin (Delete + Create — roleRef is immutable)", func() {
				Expect(f).To(ExecuteSuccessfully())
				expectDesiredCRB(f, "user-authz:cluster-admin")
			})
		})

		Context("ClusterRoleBinding kubeadm:cluster-admins already targets user-authz:cluster-admin", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(crbCurrentUserAuthzClusterAdmin))
				f.RunHook()
			})

			It("is a no-op (binding stays pointed at user-authz:cluster-admin)", func() {
				Expect(f).To(ExecuteSuccessfully())
				expectDesiredCRB(f, "user-authz:cluster-admin")
			})
		})

		Context("OnBeforeHelm tick after KubeStateSet (Helm-driven reconcile)", func() {
			BeforeEach(func() {
				f.KubeStateSet(crbCurrentClusterAdmin)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("rebinds the snapshot CRB to user-authz:cluster-admin on OnBeforeHelm too", func() {
				Expect(f).To(ExecuteSuccessfully())
				expectDesiredCRB(f, "user-authz:cluster-admin")
			})
		})
	})
})
