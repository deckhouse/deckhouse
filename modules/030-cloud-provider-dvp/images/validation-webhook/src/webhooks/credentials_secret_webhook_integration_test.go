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
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/meta"
)

var _ = Describe("CredentialSecret webhook", func() {
	BeforeEach(func() {
		createValidDVPWebhookCluster()
	})

	AfterEach(func() {
		deleteDVPWebhookCluster()
	})

	DescribeTable("rejects invalid credential updates",
		func(mutator func(*corev1.Secret), want string) {
			secret := &corev1.Secret{}
			Expect(testK8sClient.Get(testCtx, clientObjectKey(dvpmeta.Namespace, cpapi.CredentialSecretName), secret)).To(Succeed())

			mutator(secret)

			err := testK8sClient.Update(testCtx, secret)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), fmt.Sprintf("got %v", err))
			Expect(err.Error()).To(ContainSubstring(want))
		},
		Entry("unsupported authScheme", func(secret *corev1.Secret) {
			secret.Data[cpapi.CredentialSecretAuthSchemeKey] = []byte(cpapi.AuthSchemeAPIToken)
			secret.Data[cpapi.CredentialSecretSecretKey] = []byte("dGVzdC10b2tlbg==")
		}, `d8-credentials.data.authScheme: Invalid value: "apiToken": authScheme "apiToken" is not allowed`),
		Entry("missing authScheme", func(secret *corev1.Secret) {
			delete(secret.Data, cpapi.CredentialSecretAuthSchemeKey)
		}, `d8-credentials.data.authScheme: Invalid value: "null": authScheme is required`),
		Entry("missing kubeconfig secret", func(secret *corev1.Secret) {
			secret.Data[cpapi.CredentialSecretAuthSchemeKey] = []byte(cpapi.AuthSchemeKubeconfig)
			delete(secret.Data, cpapi.CredentialSecretSecretKey)
		}, `d8-credentials.data.secret: Invalid value: "null": secret is required for authScheme "kubeconfig"`),
		Entry("invalid kubeconfig secret", func(secret *corev1.Secret) {
			secret.Data[cpapi.CredentialSecretAuthSchemeKey] = []byte(cpapi.AuthSchemeKubeconfig)
			secret.Data[cpapi.CredentialSecretSecretKey] = []byte("not-base64!!!")
		}, `d8-credentials.data.secret: Invalid value: "not-base64!!!": secret must contain base64-encoded kubeconfig`),
	)

	It("does not require credentials on delete at runtime", func() {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: dvpmeta.Namespace}}
		Expect(testK8sClient.Delete(testCtx, secret)).To(Succeed())
	})
})
