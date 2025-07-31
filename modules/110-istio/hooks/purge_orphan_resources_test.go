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
		istioYAML = `
---
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  finalizers:
  - istio-finalizer.sailoperator.io
  name: v1x16
  namespace: d8-istio
`
		federationYAML = `
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  finalizers:
  - istio-finalizer.deckhouse.io
  name: federation-1
`
		multiclusterYAML = `
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  finalizers:
  - istio-finalizer.deckhouse.io
  name: multicluster-1
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
		rootCertYAML = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: istio-ca-root-cert
  namespace: d8-istio
  creationTimestamp: "2023-01-01T00:00:00Z"
`
		otherNs1YAML = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns1
`
		otherNs1RootCertYAML = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: istio-ca-root-cert
  namespace: ns1
  creationTimestamp: "2023-01-01T00:00:00Z"
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
		istioGVR = schema.GroupVersionResource{
			Group:    "sailoperator.io",
			Version:  "v1",
			Resource: "istios",
		}
		istioGVK = schema.GroupVersionKind{
			Group:   "sailoperator.io",
			Version: "v1",
			Kind:    "Istio",
		}
		federationGVR = schema.GroupVersionResource{
			Group:    "deckhouse.io",
			Version:  "v1alpha1",
			Resource: "istiofederations",
		}
		multiclusterGVR = schema.GroupVersionResource{
			Group:    "deckhouse.io",
			Version:  "v1alpha1",
			Resource: "istiomulticlusters",
		}
		ns               *corev1.Namespace
		cr1              *rbacv1.ClusterRole
		cr2              *rbacv1.ClusterRole
		crb1             *rbacv1.ClusterRoleBinding
		crb2             *rbacv1.ClusterRoleBinding
		mwc              *admissionregistrationv1.MutatingWebhookConfiguration
		vwc              *admissionregistrationv1.ValidatingWebhookConfiguration
		iop              *unstructured.Unstructured
		istio            *unstructured.Unstructured
		federation       *unstructured.Unstructured
		multicluster     *unstructured.Unstructured
		rootCert         *corev1.ConfigMap
		otherNs1         *corev1.Namespace
		otherNs1RootCert *corev1.ConfigMap
	)
	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(iopYAML), &iop)
		_ = yaml.Unmarshal([]byte(istioYAML), &istio)
		_ = yaml.Unmarshal([]byte(federationYAML), &federation)
		_ = yaml.Unmarshal([]byte(multiclusterYAML), &multicluster)
		_ = yaml.Unmarshal([]byte(nsYAML), &ns)
		_ = yaml.Unmarshal([]byte(cr1YAML), &cr1)
		_ = yaml.Unmarshal([]byte(cr2YAML), &cr2)
		_ = yaml.Unmarshal([]byte(crb1YAML), &crb1)
		_ = yaml.Unmarshal([]byte(crb2YAML), &crb2)
		_ = yaml.Unmarshal([]byte(mwcYAML), &mwc)
		_ = yaml.Unmarshal([]byte(vwcYAML), &vwc)
		_ = yaml.Unmarshal([]byte(rootCertYAML), &rootCert)
		_ = yaml.Unmarshal([]byte(otherNs1YAML), &otherNs1)
		_ = yaml.Unmarshal([]byte(otherNs1RootCertYAML), &otherNs1RootCert)
	})

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD(iopGVK.Group, iopGVK.Version, iopGVK.Kind, true)
	f.RegisterCRD(istioGVK.Group, istioGVK.Version, istioGVK.Kind, true)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)   // cluster-scoped
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", false) // cluster-scoped

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

	Context("Cluster with IstioFederation and IstioMulticluster resources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
			f.KubeStateSet("")

			// Create cluster-wide resources
			_, _ = f.KubeClient().Dynamic().Resource(federationGVR).Create(context.TODO(), federation, metav1.CreateOptions{})
			_, _ = f.KubeClient().Dynamic().Resource(multiclusterGVR).Create(context.TODO(), multicluster, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Should delete cluster-wide Istio resources", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).ToNot(HaveLen(0))

			// Verify IstioFederation is deleted
			_, err := f.KubeClient().Dynamic().Resource(federationGVR).Get(context.TODO(), "federation-1", metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"Finalizers from IstioFederation removed\",\"name\":\"federation-1\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"IstioFederation deleted\",\"name\":\"federation-1\""))

			// Verify IstioMulticluster is deleted
			_, err = f.KubeClient().Dynamic().Resource(multiclusterGVR).Get(context.TODO(), "multicluster-1", metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"Finalizers from IstioMulticluster removed\",\"name\":\"multicluster-1\""))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("\"msg\":\"IstioMulticluster deleted\",\"name\":\"multicluster-1\""))
		})
	})

	Context("Cluster with all types of Istio resources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
			f.KubeStateSet("")

			// Create all test resources
			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), otherNs1, metav1.CreateOptions{})
			_, _ = f.KubeClient().Dynamic().Resource(iopGVR).Namespace(istioSystemNs).Create(context.TODO(), iop, metav1.CreateOptions{})
			_, _ = f.KubeClient().Dynamic().Resource(istioGVR).Namespace(istioSystemNs).Create(context.TODO(), istio, metav1.CreateOptions{})
			_, _ = f.KubeClient().Dynamic().Resource(federationGVR).Create(context.TODO(), federation, metav1.CreateOptions{})
			_, _ = f.KubeClient().Dynamic().Resource(multiclusterGVR).Create(context.TODO(), multicluster, metav1.CreateOptions{})
			_, _ = f.KubeClient().RbacV1().ClusterRoles().Create(context.TODO(), cr1, metav1.CreateOptions{})
			_, _ = f.KubeClient().RbacV1().ClusterRoleBindings().Create(context.TODO(), crb1, metav1.CreateOptions{})
			_, _ = f.KubeClient().AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), mwc, metav1.CreateOptions{})
			_, _ = f.KubeClient().AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.TODO(), vwc, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().ConfigMaps(istioSystemNs).Create(context.TODO(), rootCert, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().ConfigMaps(otherNs1.Name).Create(context.TODO(), otherNs1RootCert, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Should delete all Istio resources", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Verify cluster-wide resources are deleted
			_, err := f.KubeClient().Dynamic().Resource(federationGVR).Get(context.TODO(), "federation-1", metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
			_, err = f.KubeClient().Dynamic().Resource(multiclusterGVR).Get(context.TODO(), "multicluster-1", metav1.GetOptions{})
			Expect(err).To(HaveOccurred())

			// Verify namespace-scoped resources are deleted
			Expect(f.KubernetesResource("IstioOperator", "d8-istio", "v1x16").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Istio", "d8-istio", "v1x16").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("Namespace", "d8-istio").Exists()).To(BeFalse())

			// Verify other resources are deleted
			Expect(f.KubernetesGlobalResource("ClusterRole", "istio-reader-d8-istio").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "istio-reader-d8-istio").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("MutatingWebhookConfiguration", "istio-sidecar-injector-v1x16-d8-istio").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "istio-validator-v1x16-d8-istio").Exists()).To(BeFalse())

			// Verify ConfigMaps are deleted
			_, err = f.KubeClient().CoreV1().ConfigMaps(otherNs1.Name).Get(context.TODO(), "istio-ca-root-cert", metav1.GetOptions{})
			Expect(err).To(HaveOccurred())

			// Verify logs contain expected messages
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Finalizers from Istio/v1x16 in namespace d8-istio removed"))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Istio/v1x16 deleted from namespace d8-istio"))
		})
	})
})
