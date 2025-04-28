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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: purge_orphan_resources ::", func() {

	const (
		istioSystemNs = "d8-istio"
		nsYAML        = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-istio
`
		iopYAML = `
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  finalizers:
  - istio-finalizer.install.istio.io
  name: v1x16
  namespace: d8-istio
`
		cr1YAML = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: istio-reader-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
`
		cr2YAML = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: istio-reader-clusterrole-v1x16-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
`
		crb1YAML = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: istio-reader-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
`
		crb2YAML = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: istio-reader-clusterrole-v1x16-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
`
		mwcYAML = `
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: istio-sidecar-injector-v1x16-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
`
		vwcYAML = `
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: istio-validator-v1x16-d8-istio
  labels:
    install.operator.istio.io/owning-resource-namespace: d8-istio
`
	)

	var (
		iopGVR = schema.GroupVersionResource{
			Group:    "install.istio.io",
			Version:  "v1alpha1",
			Resource: "istiooperators",
		}
		iopGVK = schema.GroupVersionKind{
			Group:   "install.istio.io",
			Version: "v1alpha1",
			Kind:    "IstioOperator",
		}
		ns   *corev1.Namespace
		cr1  *rbacv1.ClusterRole
		cr2  *rbacv1.ClusterRole
		crb1 *rbacv1.ClusterRoleBinding
		crb2 *rbacv1.ClusterRoleBinding
		mwc  *admissionregistrationv1.MutatingWebhookConfiguration
		vwc  *admissionregistrationv1.ValidatingWebhookConfiguration
		iop  *unstructured.Unstructured
	)
	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(iopYAML), &iop)
		_ = yaml.Unmarshal([]byte(nsYAML), &ns)
		_ = yaml.Unmarshal([]byte(cr1YAML), &cr1)
		_ = yaml.Unmarshal([]byte(cr2YAML), &cr2)
		_ = yaml.Unmarshal([]byte(crb1YAML), &crb1)
		_ = yaml.Unmarshal([]byte(crb2YAML), &crb2)
		_ = yaml.Unmarshal([]byte(mwcYAML), &mwc)
		_ = yaml.Unmarshal([]byte(vwcYAML), &vwc)
	})

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD(iopGVK.Group, iopGVK.Version, iopGVK.Kind, true)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
			f.KubeStateSet(``)
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(HaveLen(0))
		})
	})

	Context("Cluster with minimal settings and orphan iop", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
			f.KubeStateSet("")

			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			_, _ = f.KubeClient().Dynamic().Resource(iopGVR).Namespace(istioSystemNs).Create(context.TODO(), iop, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).ToNot(HaveLen(0))
			Expect(f.KubernetesGlobalResource("Namespace", "d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Namespace d8-istio deleted"))
			Expect(f.KubernetesResource("IstioOperator", "d8-istio", "v1x16").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Finalizers from IstioOperator/v1x16 in namespace d8-istio removed"))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("IstioOperator/v1x16 deleted from namespace d8-istio"))
		})
	})

	Context("Cluster with minimal settings and orphan istio resources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
			f.KubeStateSet("")

			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			_, _ = f.KubeClient().Dynamic().Resource(iopGVR).Namespace(istioSystemNs).Create(context.TODO(), iop, metav1.CreateOptions{})
			_, _ = f.KubeClient().RbacV1().ClusterRoles().Create(context.TODO(), cr1, metav1.CreateOptions{})
			_, _ = f.KubeClient().RbacV1().ClusterRoles().Create(context.TODO(), cr2, metav1.CreateOptions{})
			_, _ = f.KubeClient().RbacV1().ClusterRoleBindings().Create(context.TODO(), crb1, metav1.CreateOptions{})
			_, _ = f.KubeClient().RbacV1().ClusterRoleBindings().Create(context.TODO(), crb2, metav1.CreateOptions{})
			_, _ = f.KubeClient().AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), mwc, metav1.CreateOptions{})
			_, _ = f.KubeClient().AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.TODO(), vwc, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).ToNot(HaveLen(0))
			Expect(f.KubernetesGlobalResource("Namespace", "d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Namespace d8-istio deleted"))

			Expect(f.KubernetesResource("IstioOperator", "d8-istio", "v1x16").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Finalizers from IstioOperator/v1x16 in namespace d8-istio removed"))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("IstioOperator/v1x16 deleted from namespace d8-istio"))

			Expect(f.KubernetesGlobalResource("ClusterRole", "istio-reader-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("ClusterRole/istio-reader-d8-istio deleted"))
			Expect(f.KubernetesGlobalResource("ClusterRole", "istio-reader-clusterrole-v1x16-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("ClusterRole/istio-reader-clusterrole-v1x16-d8-istio deleted"))
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "istio-reader-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("ClusterRoleBinding/istio-reader-d8-istio deleted"))
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "istio-reader-clusterrole-v1x16-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("ClusterRoleBinding/istio-reader-clusterrole-v1x16-d8-istio deleted"))
			Expect(f.KubernetesGlobalResource("MutatingWebhookConfiguration", "istio-sidecar-injector-v1x16-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("MutatingWebhookConfiguration/istio-sidecar-injector-v1x16-d8-istio deleted"))
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "istio-validator-v1x16-d8-istio").Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("ValidatingWebhookConfiguration/istio-validator-v1x16-d8-istio deleted"))
		})
	})

	Context("Cluster with minimal settings and orphan istio resources including multicluster and federation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
			f.KubeStateSet("")
			f := HookExecutionConfigInit(`{}`, `{}`)
			f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", true)
			f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", true)

			// Create test resources
			imc := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "deckhouse.io/v1alpha1",
					"kind":       "IstioMulticluster",
					"metadata": map[string]interface{}{
						"name": "test-multicluster",
					},
				},
			}

			ifed := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "deckhouse.io/v1alpha1",
					"kind":       "IstioFederation",
					"metadata": map[string]interface{}{
						"name": "test-federation",
					},
				},
			}

			imcGVR := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "istiomulticlusters"}
			ifedGVR := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "istiofederations"}

			_, _ = f.KubeClient().Dynamic().Resource(imcGVR).Namespace("").Create(context.TODO(), imc, metav1.CreateOptions{})
			_, _ = f.KubeClient().Dynamic().Resource(ifedGVR).Namespace("").Create(context.TODO(), ifed, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Should delete Deckhouse Istio resources", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Verify deletions
			imcGVR := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "istiomulticlusters"}
			ifedGVR := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "istiofederations"}

			imcList, err := f.KubeClient().Dynamic().Resource(imcGVR).Namespace("").List(context.TODO(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(imcList.Items).To(HaveLen(0))

			ifedList, err := f.KubeClient().Dynamic().Resource(ifedGVR).Namespace("").List(context.TODO(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(ifedList.Items).To(HaveLen(0))
		})
	})
})
