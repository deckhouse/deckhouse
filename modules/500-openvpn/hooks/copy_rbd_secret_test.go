/*
Copyright 2021 Flant CJSC

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: openvpn :: hooks :: copy_rbd_secret ::", func() {
	var state = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: my-ssd
parameters:
  userSecretName: existing-secret
provisioner: kubernetes.io/rbd
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: my-hdd
parameters:
  userSecretName: non-existing-secret
provisioner: kubernetes.io/rbd
---
apiVersion: v1
data:
  foo: YmFyCg==
kind: Secret
metadata:
  name: existing-secret
  namespace: my-ns-a
type: kubernetes.io/rbd
---
apiVersion: v1
data:
  foo: aG9waGV5bGFsYWxleQo=
kind: Secret
metadata:
  name: existing-secret
  namespace: d8-openvpn
type: kubernetes.io/rbd
---
apiVersion: v1
data:
  foo: YmF6Cg==
kind: Secret
metadata:
  name: non-existing-secret
  namespace: my-ns-b
type: kubernetes.io/rbd
---
apiVersion: v1
data:
  foo: bm9uZXhpc3QK
kind: Secret
metadata:
  name: non-needed-secret
  namespace: my-ns-b
type: Opaque
`

	f := HookExecutionConfigInit(`{"openvpn":{}}`, `{}`)
	Context("Cluster initialization", func() {
		BeforeEach(func() {
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Secret 'non-existing-secret' must be copied to d8-openvpn", func() {
			Expect(f).To(ExecuteSuccessfully())
			copiedSecret := f.KubernetesResource("Secret", "d8-openvpn", "non-existing-secret")
			Expect(copiedSecret.Exists()).To(BeTrue())
			data, err := base64.StdEncoding.DecodeString(copiedSecret.Field("data.foo").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(data)).Should(ContainSubstring("baz"))
		})

		It("Existing secret in d8-openvpn secret must not be overwritten", func() {
			Expect(f).To(ExecuteSuccessfully())
			copiedSecret := f.KubernetesResource("Secret", "d8-openvpn", "existing-secret")
			Expect(copiedSecret.Exists()).To(BeTrue())
			data, err := base64.StdEncoding.DecodeString(copiedSecret.Field("data.foo").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(data)).Should(ContainSubstring("hopheylalaley"))
		})

		It("Non-needed (Opaque) secret should not be copied", func() {
			Expect(f).To(ExecuteSuccessfully())
			sourceSecret := f.KubernetesResource("Secret", "my-ns-b", "non-needed-secret")
			Expect(sourceSecret.Exists()).To(BeTrue())
			copiedSecret := f.KubernetesResource("Secret", "d8-openvpn", "non-needed-secret")
			Expect(copiedSecret.Exists()).To(BeFalse())
		})
	})
})
