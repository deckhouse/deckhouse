package hooks

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: copy_rbd_secret ::", func() {
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
type: Opaque
---
apiVersion: v1
data:
  foo: aG9waGV5bGFsYWxleQo=
kind: Secret
metadata:
  name: existing-secret
  namespace: d8-monitoring
type: Opaque
---
apiVersion: v1
data:
  foo: YmF6Cg==
kind: Secret
metadata:
  name: non-existing-secret
  namespace: my-ns-b
type: Opaque
`

	f := HookExecutionConfigInit(`{"prometheus":{}}`, `{}`)
	Context("Cluster initialization", func() {
		BeforeEach(func() {
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Secret 'non-existing-secret' must be copied to d8-monitoring", func() {
			Expect(f).To(ExecuteSuccessfully())
			copiedSecret := f.KubernetesResource("Secret", "d8-monitoring", "non-existing-secret")
			Expect(copiedSecret.Exists()).To(BeTrue())
			data, err := base64.StdEncoding.DecodeString(copiedSecret.Field("data.foo").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(data)).Should(ContainSubstring("baz"))
		})

		It("Existing secret in d8-monitoring secret must not be overwritten", func() {
			Expect(f).To(ExecuteSuccessfully())
			copiedSecret := f.KubernetesResource("Secret", "d8-monitoring", "existing-secret")
			Expect(copiedSecret.Exists()).To(BeTrue())
			data, err := base64.StdEncoding.DecodeString(copiedSecret.Field("data.foo").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(data)).Should(ContainSubstring("hopheylalaley"))
		})
	})
})
