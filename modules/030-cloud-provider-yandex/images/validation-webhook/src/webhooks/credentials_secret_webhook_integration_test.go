// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhooks

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
)

var _ = Describe("CredentialSecret webhook", func() {
	BeforeEach(func() {
		createValidYandexWebhookCluster()
	})

	AfterEach(func() {
		deleteYandexWebhookCluster()
	})

	DescribeTable("rejects invalid credential updates",
		func(mutator func(*corev1.Secret), want string) {
			secret := &corev1.Secret{}
			Expect(testK8sClient.Get(testCtx, clientObjectKey(ycmeta.Namespace, cpapi.CredentialSecretName), secret)).To(Succeed())

			mutator(secret)

			err := testK8sClient.Update(testCtx, secret)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), fmt.Sprintf("got %v", err))
			Expect(err.Error()).To(ContainSubstring(want))
		},
		// kubeconfig is a DVP auth scheme: Yandex accepts only serviceAccount and apiToken.
		Entry("unsupported authScheme", func(secret *corev1.Secret) {
			secret.Data[cpapi.CredentialSecretAuthSchemeKey] = []byte(cpapi.AuthSchemeKubeconfig)
		}, `d8-credentials.data.authScheme: Invalid value: "kubeconfig": authScheme "kubeconfig" is not allowed`),
		// An empty authScheme reaches CombinedCredentialValidator, which reports it as a value
		// outside the allowed set rather than as a missing key.
		Entry("missing authScheme", func(secret *corev1.Secret) {
			delete(secret.Data, cpapi.CredentialSecretAuthSchemeKey)
		}, `d8-credentials.data.authScheme: Invalid value: "": authScheme "" is not allowed`),
		Entry("missing serviceAccount secret", func(secret *corev1.Secret) {
			secret.Data[cpapi.CredentialSecretAuthSchemeKey] = []byte(cpapi.AuthSchemeServiceAccount)
			delete(secret.Data, cpapi.CredentialSecretSecretKey)
		}, `d8-credentials.data.secret: Invalid value: null: secret is required for authScheme "serviceAccount"`),
		Entry("invalid serviceAccount secret", func(secret *corev1.Secret) {
			secret.Data[cpapi.CredentialSecretAuthSchemeKey] = []byte(cpapi.AuthSchemeServiceAccount)
			secret.Data[cpapi.CredentialSecretSecretKey] = []byte("not-json!!!")
		}, `d8-credentials.data.secret: Invalid value: "masked": invalid service account`),
	)

	It("does not require credentials on delete at runtime", func() {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: ycmeta.Namespace}}
		Expect(testK8sClient.Delete(testCtx, secret)).To(Succeed())
	})
})
