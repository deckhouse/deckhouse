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
	"context"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: secret-copier :: hooks :: handler ::", func() {
	var (
		stateNSYAML1 = `
apiVersion: v1
kind: Namespace
metadata:
  name: default
`
		stateNSYAML2 = `
apiVersion: v1
kind: Namespace
metadata:
  name: ns1
`
		stateNSYAML3 = `
apiVersion: v1
kind: Namespace
metadata:
  name: ns2
  labels:
    app: custom
    foo: bar
`
		stateNSYAML4 = `
apiVersion: v1
kind: Namespace
metadata:
  name: ns3t
status:
  phase: Terminating
`
		stateNSYAML5 = `
apiVersion: v1
kind: Namespace
metadata:
  name: ns4u
  labels:
    heritage: upmeter
`
		stateSecretNeutralYAML = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: neutral
  namespace: default
data:
  supersecret: YWJj
`

		stateSecretOriginalYAML1 = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s1
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
    certmanager.k8s.io/certificate-name: certname
data:
  supersecret: czFkYXRh
`
		stateSecretOriginalYAML2 = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s2
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: czJkYXRh
`
		stateSecretOriginalYAML3 = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s3
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: czNkYXRh
`
		stateSecretOriginalYAML4 = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s4
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
  annotations:
    secret-copier.deckhouse.io/target-namespace-selector: "app=custom,foo=bar"
data:
  supersecret: czRkYXRh
`
		stateSecretOriginalYAML5 = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s5
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
  annotations:
    secret-copier.deckhouse.io/target-namespace-selector: "app=malformed label selector value"
data:
  supersecret: czVkYXRh
`
		stateSecretExtraYAML1 = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: es1
  namespace: ns1
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: ZXMxZGF0YQ==
`
		stateSecretExtraYAML2 = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: es2
  namespace: ns2
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: ZXMyZGF0YQ==
`
		// Not labeled: a Secret with the name of a copy that the hook did not
		// create. It is not the hook's to overwrite.
		stateSecretForeignYAML = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s1
  namespace: ns1
data:
  supersecret: dGVzdC12YWx1ZQ==
`
		stateSecretOutDatedYAML = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s2
  namespace: ns1
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: dGVzdC12YWx1ZQ==
`
	)

	var (
		ns1             *corev1.Namespace
		ns2             *corev1.Namespace
		ns3             *corev1.Namespace
		ns4             *corev1.Namespace
		ns5             *corev1.Namespace
		secretNeutral   *corev1.Secret
		secretOriginal1 *corev1.Secret
		secretOriginal2 *corev1.Secret
		secretOriginal3 *corev1.Secret
		secretOriginal4 *corev1.Secret
		secretOriginal5 *corev1.Secret
		secretExtra1    *corev1.Secret
		secretExtra2    *corev1.Secret
		secretForeign   *corev1.Secret
		secretOutDated  *corev1.Secret

		clusterState string
	)

	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(stateNSYAML1), &ns1)
		_ = yaml.Unmarshal([]byte(stateNSYAML2), &ns2)
		_ = yaml.Unmarshal([]byte(stateNSYAML3), &ns3)
		_ = yaml.Unmarshal([]byte(stateNSYAML4), &ns4)
		_ = yaml.Unmarshal([]byte(stateNSYAML5), &ns5)
		_ = yaml.Unmarshal([]byte(stateSecretNeutralYAML), &secretNeutral)
		_ = yaml.Unmarshal([]byte(stateSecretOriginalYAML1), &secretOriginal1)
		_ = yaml.Unmarshal([]byte(stateSecretOriginalYAML2), &secretOriginal2)
		_ = yaml.Unmarshal([]byte(stateSecretOriginalYAML3), &secretOriginal3)
		_ = yaml.Unmarshal([]byte(stateSecretOriginalYAML4), &secretOriginal4)
		_ = yaml.Unmarshal([]byte(stateSecretOriginalYAML5), &secretOriginal5)
		_ = yaml.Unmarshal([]byte(stateSecretExtraYAML1), &secretExtra1)
		_ = yaml.Unmarshal([]byte(stateSecretExtraYAML2), &secretExtra2)
		_ = yaml.Unmarshal([]byte(stateSecretForeignYAML), &secretForeign)
		_ = yaml.Unmarshal([]byte(stateSecretOutDatedYAML), &secretOutDated)

		nsYAML1, _ := yaml.Marshal(&ns1)
		nsYAML2, _ := yaml.Marshal(&ns2)
		nsYAML3, _ := yaml.Marshal(&ns3)
		nsYAML4, _ := yaml.Marshal(&ns4)
		nsYAML5, _ := yaml.Marshal(&ns5)
		secretOriginalYAML1, _ := yaml.Marshal(&secretOriginal1)
		secretOriginalYAML2, _ := yaml.Marshal(&secretOriginal2)
		secretOriginalYAML3, _ := yaml.Marshal(&secretOriginal3)
		secretOriginalYAML4, _ := yaml.Marshal(&secretOriginal4)
		secretOriginalYAML5, _ := yaml.Marshal(&secretOriginal5)
		secretNeutralYAML, _ := yaml.Marshal(&secretNeutral)
		secretExtraYAML1, _ := yaml.Marshal(&secretExtra1)
		secretExtraYAML2, _ := yaml.Marshal(&secretExtra2)
		secretForeignYAML, _ := yaml.Marshal(&secretForeign)
		secretOutDatedYAML, _ := yaml.Marshal(&secretOutDated)

		clusterState = strings.Join([]string{
			string(nsYAML1),
			string(nsYAML2),
			string(nsYAML3),
			string(nsYAML4),
			string(nsYAML5),
			string(secretOriginalYAML1),
			string(secretOriginalYAML2),
			string(secretOriginalYAML3),
			string(secretOriginalYAML4),
			string(secretOriginalYAML5),
			string(secretNeutralYAML),
			string(secretExtraYAML1),
			string(secretExtraYAML2),
			string(secretForeignYAML),
			string(secretOutDatedYAML),
		}, "---\n")

	})

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Namespaces and all types of secrets are in cluster", func() {
		BeforeEach(func() {

			f.BindingContexts.Set(f.KubeStateSet(clusterState))

			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), ns1, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), ns2, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), ns3, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), ns4, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), ns5, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Secrets(secretNeutral.Namespace).Create(context.TODO(), secretNeutral, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Secrets(secretOriginal1.Namespace).Create(context.TODO(), secretOriginal1, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Secrets(secretOriginal2.Namespace).Create(context.TODO(), secretOriginal2, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Secrets(secretOriginal3.Namespace).Create(context.TODO(), secretOriginal3, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Secrets(secretExtra1.Namespace).Create(context.TODO(), secretExtra1, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Secrets(secretExtra2.Namespace).Create(context.TODO(), secretExtra2, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Secrets(secretForeign.Namespace).Create(context.TODO(), secretForeign, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Secrets(secretOutDated.Namespace).Create(context.TODO(), secretOutDated, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Five secrets must be actual", func() {
			Expect(f).To(ExecuteSuccessfully())

			_, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "es1", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns2").Get(context.TODO(), "es2", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			s, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s2", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s2data"))

			s, err = f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s3", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s3data"))

			s, err = f.KubeClient().CoreV1().Secrets("ns2").Get(context.TODO(), "s1", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s1data"))
			_, found := s.ObjectMeta.Labels["certmanager.k8s.io/certificate-name"]
			Expect(found).To(BeFalse())

			s, err = f.KubeClient().CoreV1().Secrets("ns2").Get(context.TODO(), "s2", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s2data"))

			s, err = f.KubeClient().CoreV1().Secrets("ns2").Get(context.TODO(), "s3", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s3data"))

			_, err = f.KubeClient().CoreV1().Secrets("ns3t").Get(context.TODO(), "s1", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns3t").Get(context.TODO(), "s2", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns3t").Get(context.TODO(), "s3", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns4u").Get(context.TODO(), "s1", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns4u").Get(context.TODO(), "s2", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns4u").Get(context.TODO(), "s3", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

		})

		It("A Secret with the name of a copy that the hook did not create must be left as is", func() {
			Expect(f).To(ExecuteSuccessfully())

			s, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s1", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("test-value"))
			Expect(s.Labels).To(BeEmpty())
			Expect(s.Annotations).To(BeEmpty())
			Expect(writesTo(f, "ns1", "s1")).To(BeEmpty())
		})

		It("Custom Secret #4 must be copied only in namespace having app=custom label", func() {
			Expect(f).To(ExecuteSuccessfully())

			_, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s4", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			s, err := f.KubeClient().CoreV1().Secrets("ns2").Get(context.TODO(), "s4", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s4data"))

			_, err = f.KubeClient().CoreV1().Secrets("ns3t").Get(context.TODO(), "s4", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns4u").Get(context.TODO(), "s4", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())
		})

		It("Custom Secret #5 must not be copied to any namespace because of malformed namespace selector value", func() {
			Expect(f).To(ExecuteSuccessfully())

			_, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s5", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns2").Get(context.TODO(), "s5", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns3t").Get(context.TODO(), "s5", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns4u").Get(context.TODO(), "s5", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())
		})
	})

	Context("Secrets aimed at the namespaces of Deckhouse and Kubernetes", func() {
		const namespaces = `
apiVersion: v1
kind: Namespace
metadata:
  name: default
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns1
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
---
apiVersion: v1
kind: Namespace
metadata:
  name: kube-system
`

		Context("An ordinary Secret", func() {
			const state = namespaces + `
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: registry
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  auth: dXNlcm5hbWU6cGFzc3dvcmQ=
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: registry
  namespace: kube-system
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  auth: dGVzdC12YWx1ZQ==
`
			// An outdated copy an earlier version made into kube-system.
			const earlierCopy = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: registry
  namespace: kube-system
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  auth: dGVzdC12YWx1ZQ==
`

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(state))
				createLiveSecrets(f, earlierCopy)
				f.RunHook()
			})

			It("Must be copied into them as into any other namespace, and the copies made there before must be kept up to date", func() {
				Expect(f).To(ExecuteSuccessfully())

				for _, namespace := range []string{"ns1", "d8-system", "kube-system"} {
					s, err := f.KubeClient().CoreV1().Secrets(namespace).Get(context.TODO(), "registry", metav1.GetOptions{})
					Expect(err).To(BeNil(), namespace)
					Expect(string(s.Data["auth"])).To(Equal("username:password"), namespace)
				}
				writes := writesTo(f, "kube-system", "registry")
				Expect(writes).To(HaveLen(1))
				Expect(writes[0].GetVerb()).To(Equal("update"))
			})
		})

		Context("The cluster key of licensing", func() {
			const source = `
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: cluster-key
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  seed: dGVzdC12YWx1ZQ==
`

			Context("In place", func() {
				// Not a copy, so the snapshot does not have it.
				const clusterKey = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: cluster-key
  namespace: d8-system
data:
  seed: ZGgtdGVzdC1zZWNyZXQtMDEyMzQ1Njc4
`

				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(namespaces + source))
					createLiveSecrets(f, clusterKey)
					f.RunHook()
				})

				It("Must be left as is, while the Secret of its name is copied elsewhere", func() {
					Expect(f).To(ExecuteSuccessfully())

					s, err := f.KubeClient().CoreV1().Secrets("d8-system").Get(context.TODO(), "cluster-key", metav1.GetOptions{})
					Expect(err).To(BeNil())
					Expect(string(s.Data["seed"])).To(Equal("dh-test-secret-012345678"))
					Expect(s.Labels).To(BeEmpty())
					Expect(writesTo(f, "d8-system", "cluster-key")).To(BeEmpty())

					s, err = f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "cluster-key", metav1.GetOptions{})
					Expect(err).To(BeNil())
					Expect(string(s.Data["seed"])).To(Equal("test-value"))
				})
			})

			Context("Not created yet", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(namespaces + source))
					f.RunHook()
				})

				It("Must not be created", func() {
					Expect(f).To(ExecuteSuccessfully())

					_, err := f.KubeClient().CoreV1().Secrets("d8-system").Get(context.TODO(), "cluster-key", metav1.GetOptions{})
					Expect(apierrors.IsNotFound(err)).To(BeTrue())
				})
			})

			Context("Overwritten by an earlier version", func() {
				// It carries the label, so the snapshot has it as a copy.
				const clusterKey = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: cluster-key
  namespace: d8-system
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  seed: ZGgtdGVzdC1zZWNyZXQtMDEyMzQ1Njc4
`

				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(namespaces + source + "---\n" + clusterKey))
					createLiveSecrets(f, clusterKey)
					f.RunHook()
				})

				It("Must be neither updated nor deleted", func() {
					Expect(f).To(ExecuteSuccessfully())

					s, err := f.KubeClient().CoreV1().Secrets("d8-system").Get(context.TODO(), "cluster-key", metav1.GetOptions{})
					Expect(err).To(BeNil())
					Expect(string(s.Data["seed"])).To(Equal("dh-test-secret-012345678"))
					Expect(writesTo(f, "d8-system", "cluster-key")).To(BeEmpty())
				})
			})
		})

		Context("A bootstrap token", func() {
			const state = namespaces + `
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
  name: bootstrap-token-abcdef
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  token-secret: ZGgtdGVzdC1zZWNyZXQtMDEyMzQ1Njc4
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
  name: bootstrap-token-abcdef
  namespace: kube-system
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  token-secret: ZGgtdGVzdC1zZWNyZXQtMDEyMzQ1Njc4
`
			// The copy an earlier version made into kube-system, where
			// kube-apiserver accepts it as credentials.
			const earlierCopy = `
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
  name: bootstrap-token-abcdef
  namespace: kube-system
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  token-secret: ZGgtdGVzdC1zZWNyZXQtMDEyMzQ1Njc4
`

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(state))
				createLiveSecrets(f, earlierCopy)
				f.RunHook()
			})

			It("Must not be copied anywhere, and the copies made before must be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())

				for _, namespace := range []string{"ns1", "d8-system", "kube-system"} {
					_, err := f.KubeClient().CoreV1().Secrets(namespace).Get(context.TODO(), "bootstrap-token-abcdef", metav1.GetOptions{})
					Expect(apierrors.IsNotFound(err)).To(BeTrue(), namespace)
				}
			})
		})
	})

	Context("Secrets that are not what the snapshot says", func() {
		const state = `
apiVersion: v1
kind: Namespace
metadata:
  name: default
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns1
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s2
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: czJkYXRh
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: orphan
  namespace: ns1
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: dGVzdC12YWx1ZQ==
`
		// The snapshot has ns1/orphan as a copy whose source is gone. Since
		// then somebody put a Secret of their own in its place.
		const replaced = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: orphan
  namespace: ns1
data:
  supersecret: dGVzdC12YWx1ZQ==
`
		// An outdated copy that the snapshot misses.
		const missed = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s2
  namespace: ns1
  resourceVersion: "42"
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: dGVzdC12YWx1ZQ==
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state))
			createLiveSecrets(f, replaced, missed)
			f.RunHook()
		})

		It("A Secret that replaced a copy must not be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			s, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "orphan", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("test-value"))
			Expect(writesTo(f, "ns1", "orphan")).To(BeEmpty())
		})

		It("A copy the snapshot misses must be updated at the version that was read", func() {
			Expect(f).To(ExecuteSuccessfully())

			s, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s2", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s2data"))

			writes := writesTo(f, "ns1", "s2")
			Expect(writes).To(HaveLen(1))
			Expect(writes[0].GetVerb()).To(Equal("update"))
			Expect(writes[0].(k8stesting.UpdateAction).GetObject().(*corev1.Secret).ResourceVersion).To(Equal("42"))
		})
	})

	Context("A copy that cannot be updated in place", func() {
		const state = `
apiVersion: v1
kind: Namespace
metadata:
  name: default
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns1
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s2
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: czJkYXRh
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s2
  namespace: ns1
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: dGVzdC12YWx1ZQ==
`
		const outdatedCopy = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s2
  namespace: ns1
  uid: copy-uid
  resourceVersion: "42"
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: dGVzdC12YWx1ZQ==
`

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state))
			createLiveSecrets(f, outdatedCopy)
			// The API server rejects an update of an immutable Secret this way.
			f.KubeClient().Interface.(*fake.Clientset).PrependReactor("update", "secrets", func(_ k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewInvalid(schema.GroupKind{Kind: "Secret"}, "s2", field.ErrorList{
					field.Forbidden(field.NewPath("data"), "field is immutable when `immutable` is set"),
				})
			})
			f.RunHook()
		})

		It("The copy must be recreated, and the delete must be bound to the copy that was read", func() {
			Expect(f).To(ExecuteSuccessfully())

			s, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s2", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s2data"))
			Expect(s.Annotations).To(HaveKey("secret-copier.deckhouse.io/created-at"))

			var deletes []k8stesting.DeleteAction
			for _, write := range writesTo(f, "ns1", "s2") {
				if write.GetVerb() == "delete" {
					deletes = append(deletes, write.(k8stesting.DeleteAction))
				}
			}
			Expect(deletes).To(HaveLen(1))
			preconditions := deletes[0].GetDeleteOptions().Preconditions
			Expect(preconditions).ToNot(BeNil())
			Expect(preconditions.UID).To(Equal(ptr.To(types.UID("copy-uid"))))
			Expect(preconditions.ResourceVersion).To(Equal(ptr.To("42")))
		})
	})
})

// createLiveSecrets puts Secrets into the cluster the hook reads and writes. The
// snapshot is built by KubeStateSet alone, so a Secret created only here is one
// the snapshot does not have, or has in another form.
func createLiveSecrets(f *HookExecutionConfig, manifests ...string) {
	for _, manifest := range manifests {
		secret := &corev1.Secret{}
		Expect(yaml.Unmarshal([]byte(manifest), secret)).To(Succeed())
		_, err := f.KubeClient().CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		Expect(err).To(BeNil())
	}
}

// writesTo lists the updates and deletes sent for one Secret.
func writesTo(f *HookExecutionConfig, namespace, name string) []k8stesting.Action {
	var writes []k8stesting.Action
	for _, action := range f.KubeClient().Interface.(*fake.Clientset).Actions() {
		if action.GetResource().Resource != "secrets" || action.GetNamespace() != namespace {
			continue
		}
		switch action.GetVerb() {
		case "update":
			if action.(k8stesting.UpdateAction).GetObject().(*corev1.Secret).Name == name {
				writes = append(writes, action)
			}
		case "delete":
			if action.(k8stesting.DeleteAction).GetName() == name {
				writes = append(writes, action)
			}
		}
	}
	return writes
}
