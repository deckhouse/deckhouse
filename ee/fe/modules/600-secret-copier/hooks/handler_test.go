/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		// Label is missed here on purpose. The hook should reconcile resources without errors.
		stateSecretUpToDateYAML = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s1
  namespace: ns1
data:
  supersecret: czFkYXRh
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
  supersecret: b2xkX3MyX2RhdGE=
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
		secretUpToDate  *corev1.Secret
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
		_ = yaml.Unmarshal([]byte(stateSecretUpToDateYAML), &secretUpToDate)
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
		secretUpToDateYAML, _ := yaml.Marshal(&secretUpToDate)
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
			string(secretUpToDateYAML),
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
			_, _ = f.KubeClient().CoreV1().Secrets(secretUpToDate.Namespace).Create(context.TODO(), secretUpToDate, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Secrets(secretOutDated.Namespace).Create(context.TODO(), secretOutDated, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Six secrets must be actual", func() {
			Expect(f).To(ExecuteSuccessfully())

			_, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "es1", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			_, err = f.KubeClient().CoreV1().Secrets("ns2").Get(context.TODO(), "es2", metav1.GetOptions{})
			Expect(err).ToNot(BeNil())

			s, err := f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s1", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s1data"))
			_, found := s.ObjectMeta.Labels["certmanager.k8s.io/certificate-name"]
			Expect(found).To(BeFalse())

			s, err = f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s2", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s2data"))

			s, err = f.KubeClient().CoreV1().Secrets("ns1").Get(context.TODO(), "s3", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s3data"))

			s, err = f.KubeClient().CoreV1().Secrets("ns2").Get(context.TODO(), "s1", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(string(s.Data["supersecret"])).To(Equal("s1data"))
			_, found = s.ObjectMeta.Labels["certmanager.k8s.io/certificate-name"]
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
})
