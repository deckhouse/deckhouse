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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: istio :: hooks :: ensure_crds_istio ::", func() {
	f := HookExecutionConfigInit(`{ "istio": { "internal": { } } }`, `{"istio":{}}`)

	Context("Empty cluster, no globalVersion in values", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("istio.internal.globalVersion is not discovered by discovery_versions_to_install.go yet"))
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeFalse())
		})
	})

	Context("Only globalVersion in values", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v0.991 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.991"))
		})
	})

	Context("globalVersion in values and additionalVersion older than global", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			f.ConfigValuesSetFromYaml("istio.additionalVersions", []byte(`["0.990"]`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v0.991 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.991"))
		})
	})

	Context("globalVersion in values and additionalVersion newer than global", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			f.ConfigValuesSetFromYaml("istio.additionalVersions", []byte(`["0.992"]`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v0.992 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.992"))
		})
	})

	Context("globalVersion 1.27 installs CRDs from 1.27 directory", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "1.27")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("d8-istio namespace exists without secret", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			_, err := dependency.TestDC.MustGetK8sClient().CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "d8-istio"},
			}, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("creates module-values-store with configured max version", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.991"))

			secret := f.KubernetesResource("Secret", "d8-istio", "module-values-store")
			Expect(secret.Exists()).To(BeTrue())
			Expect(decodeSecretData(secret.Field("data.maxIstioVersionBeenInCluster").String())).To(Equal("0.991"))
		})
	})

	Context("secret has higher version than configured", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			_, err := dependency.TestDC.MustGetK8sClient().CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "d8-istio"},
			}, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = dependency.TestDC.MustGetK8sClient().CoreV1().Secrets("d8-istio").Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "module-values-store",
					Namespace: "d8-istio",
				},
				Data: map[string][]byte{
					"maxIstioVersionBeenInCluster": []byte("0.992"),
				},
			}, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("applies CRDs of secret version and keeps watermark", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.992"))

			secret := f.KubernetesResource("Secret", "d8-istio", "module-values-store")
			Expect(secret.Exists()).To(BeTrue())
			Expect(decodeSecretData(secret.Field("data.maxIstioVersionBeenInCluster").String())).To(Equal("0.992"))
		})
	})

	Context("secret has lower version than configured", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.ValuesSet("istio.internal.globalVersion", "0.992")
			_, err := dependency.TestDC.MustGetK8sClient().CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "d8-istio"},
			}, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = dependency.TestDC.MustGetK8sClient().CoreV1().Secrets("d8-istio").Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "module-values-store",
					Namespace: "d8-istio",
				},
				Data: map[string][]byte{
					"maxIstioVersionBeenInCluster": []byte("0.991"),
				},
			}, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("applies CRDs of configured max and raises watermark", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.992"))

			secret := f.KubernetesResource("Secret", "d8-istio", "module-values-store")
			Expect(secret.Exists()).To(BeTrue())
			Expect(decodeSecretData(secret.Field("data.maxIstioVersionBeenInCluster").String())).To(Equal("0.992"))
		})
	})
})

func decodeSecretData(encoded string) string {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	Expect(err).ToNot(HaveOccurred())
	return string(decoded)
}
