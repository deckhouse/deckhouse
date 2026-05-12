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
		// All four combinations of (user-authz on/off) × (bootstrapped true/false).
		valuesUserAuthzOffNotBootstrapped = `{"global": {"enabledModules": [], "clusterIsBootstrapped": false}, "controlPlaneManager":{"internal": {}}}`
		valuesUserAuthzOffBootstrapped    = `{"global": {"enabledModules": [], "clusterIsBootstrapped": true}, "controlPlaneManager":{"internal": {}}}`
		valuesUserAuthzOnNotBootstrapped  = `{"global": {"enabledModules": ["user-authz"], "clusterIsBootstrapped": false}, "controlPlaneManager":{"internal": {}}}`
		valuesUserAuthzOnBootstrapped     = `{"global": {"enabledModules": ["user-authz"], "clusterIsBootstrapped": true}, "controlPlaneManager":{"internal": {}}}`

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

		userAuthzClusterAdminCR = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: user-authz:cluster-admin
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get"]
`
	)

	const (
		targetRolePath        = "controlPlaneManager.internal.kubeadmClusterAdminsTargetRoleName"
		supplementEnabledPath = "controlPlaneManager.internal.kubeadmClusterAdminsSupplementEnabled"
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

	// expectInternalDecision asserts that the hook published the same decision into Helm values
	// that it actually applied to the CRB — keeping the template-rendered shape aligned with the
	// hook-rendered shape (single source of truth invariant).
	expectInternalDecision := func(f *HookExecutionConfig, targetRole string, supplementEnabled bool) {
		Expect(f.ValuesGet(targetRolePath).String()).To(Equal(targetRole))
		Expect(f.ValuesGet(supplementEnabledPath).Bool()).To(Equal(supplementEnabled))
	}

	// ── user-authz disabled: binding must always stay on cluster-admin (kubeadm-default) ──
	Context("user-authz disabled and not bootstrapped, no CRB", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOffNotBootstrapped, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("creates the binding pointing to cluster-admin and publishes target=cluster-admin, supplement=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "cluster-admin")
			expectInternalDecision(f, "cluster-admin", false)
		})
	})

	Context("user-authz disabled and bootstrapped, CRB already on cluster-admin", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOffBootstrapped, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crbCurrentClusterAdmin))
			f.RunHook()
		})
		It("is a no-op and publishes target=cluster-admin, supplement=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "cluster-admin")
			expectInternalDecision(f, "cluster-admin", false)
		})
	})

	Context("user-authz disabled, CRB on user-authz:cluster-admin (stale state)", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOffBootstrapped, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crbCurrentUserAuthzClusterAdmin + userAuthzClusterAdminCR))
			f.RunHook()
		})
		It("rebinds back to cluster-admin (immutable roleRef → Delete+Create) and publishes target=cluster-admin, supplement=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "cluster-admin")
			expectInternalDecision(f, "cluster-admin", false)
		})
	})

	// ── user-authz enabled but cluster not yet bootstrapped: hold cluster-admin ──
	Context("user-authz enabled but cluster not bootstrapped, granular role already in API", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOnNotBootstrapped, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(userAuthzClusterAdminCR))
			f.RunHook()
		})
		It("keeps cluster-admin (gate 2 false) and publishes target=cluster-admin, supplement=true", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "cluster-admin")
			expectInternalDecision(f, "cluster-admin", true)
		})
	})

	Context("user-authz enabled, NOT bootstrapped, CRB already pointing at user-authz:cluster-admin", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOnNotBootstrapped, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crbCurrentUserAuthzClusterAdmin + userAuthzClusterAdminCR))
			f.RunHook()
		})
		It("rolls the binding back to cluster-admin while bootstrap is still in flight, supplement stays on", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "cluster-admin")
			expectInternalDecision(f, "cluster-admin", true)
		})
	})

	// ── user-authz enabled and bootstrapped, granular role missing in API: hold cluster-admin ──
	Context("user-authz enabled and bootstrapped, granular ClusterRole not yet in API", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOnBootstrapped, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("keeps cluster-admin (gate 3 false) and publishes target=cluster-admin, supplement=true", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "cluster-admin")
			expectInternalDecision(f, "cluster-admin", true)
		})
	})

	// ── user-authz enabled, bootstrapped, granular role in API: switch happens ──
	Context("user-authz enabled, bootstrapped, granular role in API, no CRB", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOnBootstrapped, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(userAuthzClusterAdminCR))
			f.RunHook()
		})
		It("creates the binding pointing to user-authz:cluster-admin and publishes target=user-authz:cluster-admin, supplement=true", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "user-authz:cluster-admin")
			expectInternalDecision(f, "user-authz:cluster-admin", true)
		})
	})

	Context("user-authz enabled, bootstrapped, granular role in API, CRB on cluster-admin", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOnBootstrapped, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crbCurrentClusterAdmin + userAuthzClusterAdminCR))
			f.RunHook()
		})
		It("rebinds to user-authz:cluster-admin (immutable roleRef → Delete+Create) and publishes target=user-authz:cluster-admin, supplement=true", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "user-authz:cluster-admin")
			expectInternalDecision(f, "user-authz:cluster-admin", true)
		})
	})

	Context("user-authz enabled, bootstrapped, granular role in API, CRB already on user-authz:cluster-admin", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOnBootstrapped, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crbCurrentUserAuthzClusterAdmin + userAuthzClusterAdminCR))
			f.RunHook()
		})
		It("is a no-op and keeps target=user-authz:cluster-admin, supplement=true", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "user-authz:cluster-admin")
			expectInternalDecision(f, "user-authz:cluster-admin", true)
		})
	})

	Context("OnBeforeHelm tick (Helm-driven reconcile) with all three gates satisfied", func() {
		f := HookExecutionConfigInit(valuesUserAuthzOnBootstrapped, "")
		BeforeEach(func() {
			f.KubeStateSet(crbCurrentClusterAdmin + userAuthzClusterAdminCR)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("rebinds the snapshot CRB to user-authz:cluster-admin on OnBeforeHelm too and publishes target=user-authz:cluster-admin, supplement=true", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectDesiredCRB(f, "user-authz:cluster-admin")
			expectInternalDecision(f, "user-authz:cluster-admin", true)
		})
	})
})
