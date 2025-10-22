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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: handle trivy provider registry secrets", func() {
	f := HookExecutionConfigInit(`{"admissionPolicyEngine":{"internal":{"denyVulnerableImages": {}}}}`, ``)

	BeforeEach(func() {
		f.BindingContexts.Set(f.KubeStateSet(testDenyVulnerableImagesSecrets))
	})

	Context("Hook doesn't run with denyVulnerableImages disabled", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("should not store data in values", func() {
			Expect(f.ValuesGet("admissionPolicyEngine.internal.denyVulnerableImages.dockerConfigJson").String()).To(Equal(""))
		})
	})

	Context("Registry secrets data is stored in values", func() {
		BeforeEach(func() {
			_, err := f.KubeClient().CoreV1().Secrets(testDenyVulnerableImagesSecret.GetNamespace()).Create(context.Background(), testDenyVulnerableImagesSecret, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			f.ValuesSet("admissionPolicyEngine.denyVulnerableImages.enabled", true)
			f.ValuesSetFromYaml("admissionPolicyEngine.denyVulnerableImages.registrySecrets", []byte(`[{"name": "test-2", "namespace": "default"}]`))
			f.RunHook()
		})

		AfterEach(func() {
			err := f.KubeClient().CoreV1().Secrets(testDenyVulnerableImagesSecret.GetNamespace()).Delete(context.Background(), testDenyVulnerableImagesSecret.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("should store data in values", func() {
			Expect(f.ValuesGet("admissionPolicyEngine.internal.denyVulnerableImages.dockerConfigJson").String()).To(MatchJSON(testDenyVulnerableImagesSecretsValues))
		})
	})
})
var (
	testDenyVulnerableImagesSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-2",
			Namespace: "default",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.test-3.com":{"username":"test-3","password":"password-3"}}}`),
		},
	}
)

const (
	testDenyVulnerableImagesSecrets = `
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: test-1
  namespace: d8-admission-policy-engine
data:
  # base64 -w0 <<< '{"auths":{"registry.test-1.com":{"username":"test-1","password":"password-1"}}}' && echo
  .dockerconfigjson: eyJhdXRocyI6eyJyZWdpc3RyeS50ZXN0LTEuY29tIjp7InVzZXJuYW1lIjoidGVzdC0xIiwicGFzc3dvcmQiOiJwYXNzd29yZC0xIn19fQo=
---
apiVersion: v1
kind: Secret
type: kubernetes.io/dockerconfigjson
metadata:
  name: deckhouse-registry
  namespace: d8-admission-policy-engine
data:
  # base64 -w0 <<< '{"auths":{"registry.test-2.com":{"username":"test-2","password":"password-2"}}}' && echo
  .dockerconfigjson: eyJhdXRocyI6eyJyZWdpc3RyeS50ZXN0LTIuY29tIjp7InVzZXJuYW1lIjoidGVzdC0yIiwicGFzc3dvcmQiOiJwYXNzd29yZC0yIn19fQo=
---
apiVersion: v1
kind: Secret
type: kubernetes.io/dockerconfigjson
metadata:
  name: test-3
  namespace: d8-admission-policy-engine
data:
  # base64 -w0 <<< '{"auths":{"registry.test-4.com":{}}}' && echo
  .dockerconfigjson: eyJhdXRocyI6eyJyZWdpc3RyeS50ZXN0LTQuY29tIjp7fX19Cg==
`

	testDenyVulnerableImagesSecretsValues = `
{
  "auths":{
    "registry.test-2.com":{
      "username":"test-2",
      "password":"password-2",
      "auth": "dGVzdC0yOnBhc3N3b3JkLTI="
    },
    "registry.test-3.com": {
      "username": "test-3",
      "password": "password-3",
      "auth": "dGVzdC0zOnBhc3N3b3JkLTM="
    },
    "registry.test-4.com": {}
  }
}
`
)
