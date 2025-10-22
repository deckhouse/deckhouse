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
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: ensure_secret_encryption_key ::", func() {
	f := HookExecutionConfigInit(`{"controlPlaneManager":{"apiserver":{"encryptionEnabled":true}, "internal":{}}}`, ``)

	Context("Empty cluster, encryptionEnabled = true", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Must create a Secret with hash and set a value", func() {
			secret := f.KubernetesResource("Secret", kubeSystemNS, secretEncryptionKeySecretName)
			Expect(secret.Exists()).To(BeTrue())

			Expect(f.ValuesGet(secretEncryptionKeyValuePath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(secretEncryptionKeyValuePath).String()).To(HaveLen(44))
		})
	})

	g := HookExecutionConfigInit(`{"controlPlaneManager":{"internal":{}}}`, ``)

	Context("Empty cluster, encryptionEnabled = false", func() {
		BeforeEach(func() {
			g.BindingContexts.Set(g.KubeStateSet(``))
			g.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(g).To(ExecuteSuccessfully())
		})

		It("Must not create a Secret with hash", func() {
			secret := g.KubernetesResource("Secret", kubeSystemNS, secretEncryptionKeySecretName)
			Expect(secret.Exists()).To(BeFalse())
			Expect(f.ValuesGet(secretEncryptionKeyValuePath).Exists()).To(BeFalse())
		})
	})

	Context("Secret already exists in snapshot", func() {
		var encodedKey string
		var rawKey []byte

		BeforeEach(func() {
			rawKey = []byte("12345678901234567890123456789012")
			encodedKey = base64.StdEncoding.EncodeToString(rawKey)

			secretYAML := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
data:
  %s: %s
`, secretEncryptionKeySecretName, kubeSystemNS, secretEncryptionKeySecretKey, encodedKey)

			f.BindingContexts.Set(f.KubeStateSet(secretYAML))
			f.RunHook()
		})

		It("Should load the existing secret key and set it into values", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet(secretEncryptionKeyValuePath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(secretEncryptionKeyValuePath).String()).To(Equal(encodedKey))
		})
	})

})
