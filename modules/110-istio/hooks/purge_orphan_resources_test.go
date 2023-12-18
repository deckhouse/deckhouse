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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: purge_orphan_resources ::", func() {

	const (
		ns = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-istio
`
		iop = `
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  finalizers:
  - istio-finalizer.install.istio.io
  name: v1x16
  namespace: d8-istio
`
		clwRes = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: istio-reader-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: istio-reader-clusterrole-v1x16-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: istio-reader-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: istio-reader-clusterrole-v1x16-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: istio-sidecar-injector-v1x16-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: istio-validator-v1x16-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
`
		nsdRes = `
---
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: stats-filter-1.13-v1x16
  namespace: d8-istio
---
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: tcp-stats-filter-1.13-v1x16
  namespace: d8-istio
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: istiod-v1x16
  namespace: d8-istio
---
apiVersion: v1
kind: Service
metadata:
  name: istiod-v1x16
  namespace: d8-istio
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: istiod-v1x16
  namespace: d8-istio
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: istiod-v1x16
  namespace: d8-istio
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: istiod-service-account
  namespace: d8-istio
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: istiod-v1x16
  namespace: d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: istiod-d8-istio
  namespace: d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: istiod-v1x16
  namespace: d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: istiod-d8-istio
  namespace: d8-istio
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: istiod-v1x16
  namespace: d8-istio
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("install.istio.io", "v1alpha1", "IstioOperator", true)
	f.RegisterCRD("networking.istio.io", "v1alpha3", "EnvoyFilter", true)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))
		})
	})

	Context("Cluster with minimal settings and orphan iop", func() {
		BeforeEach(func() {
			f.KubeStateSet(iop)
			f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).ToNot(HaveLen(0))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Remove finalizers from IstioOperator/v1x16 in namespace d8-istio"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete IstioOperator/v1x16 in namespace d8-istio"))
			Expect(f.KubernetesResource("IstioOperator", "d8-istio", "v1x16").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with minimal settings and orphan istio resources", func() {
		BeforeEach(func() {
			f.KubeStateSet(ns + iop + clwRes + nsdRes)
			f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).ToNot(HaveLen(0))
			Expect(f.KubernetesGlobalResource("Namespace", "d8-istio").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("IstioOperator", "d8-istio", "v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Remove finalizers from IstioOperator/v1x16 in namespace d8-istio"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete IstioOperator/v1x16 in namespace d8-istio"))

			Expect(f.KubernetesResource("EnvoyFilter", "d8-istio", "stats-filter-1.13-v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete EnvoyFilter/stats-filter-1.13-v1x16 in namespace d8-istio"))
			Expect(f.KubernetesResource("EnvoyFilter", "d8-istio", "tcp-stats-filter-1.13-v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete EnvoyFilter/tcp-stats-filter-1.13-v1x16 in namespace d8-istio"))

			Expect(f.KubernetesResource("Deployment", "d8-istio", "istiod-v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete Deployment/istiod-v1x16 in namespace d8-istio"))
			Expect(f.KubernetesResource("Service", "d8-istio", "istiod-v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete Service/istiod-v1x16 in namespace d8-istio"))
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istiod-v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete ConfigMap/istiod-v1x16 in namespace d8-istio"))
			Expect(f.KubernetesResource("PodDisruptionBudget", "d8-istio", "istiod-v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete PodDisruptionBudget/istiod-v1x16 in namespace d8-istio"))

			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "istiod-service-account").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete ServiceAccount/istiod-service-account in namespace d8-istio"))
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "istiod-v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete ServiceAccount/istiod-v1x16 in namespace d8-istio"))
			Expect(f.KubernetesResource("Role", "d8-istio", "istiod-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete Role/istiod-d8-istio in namespace d8-istio"))
			Expect(f.KubernetesResource("Role", "d8-istio", "istiod-v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete Role/istiod-v1x16 in namespace d8-istio"))
			Expect(f.KubernetesResource("RoleBinding", "d8-istio", "istiod-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete RoleBinding/istiod-d8-istio in namespace d8-istio"))
			Expect(f.KubernetesResource("RoleBinding", "d8-istio", "istiod-v1x16").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete RoleBinding/istiod-v1x16 in namespace d8-istio"))

			Expect(f.KubernetesGlobalResource("ClusterRole", "istio-reader-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete ClusterRole/istio-reader-d8-istio in namespace "))
			Expect(f.KubernetesGlobalResource("ClusterRole", "istio-reader-clusterrole-v1x16-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete ClusterRole/istio-reader-clusterrole-v1x16-d8-istio in namespace "))
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "istio-reader-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete ClusterRoleBinding/istio-reader-d8-istio in namespace "))
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "istio-reader-clusterrole-v1x16-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete ClusterRoleBinding/istio-reader-clusterrole-v1x16-d8-istio in namespace "))
			Expect(f.KubernetesGlobalResource("MutatingWebhookConfiguration", "istio-sidecar-injector-v1x16-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete MutatingWebhookConfiguration/istio-sidecar-injector-v1x16-d8-istio in namespace "))
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "istio-validator-v1x16-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Delete ValidatingWebhookConfiguration/istio-validator-v1x16-d8-istio in namespace "))
		})
	})

})
