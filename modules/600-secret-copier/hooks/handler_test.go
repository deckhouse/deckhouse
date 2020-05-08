package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Modules :: secret-copier :: hooks :: handler ::", func() {
	const (
		stateNamespaces = `
---
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
  name: ns2
---
apiVersion: v1
kind: Namespace
metadata:
  name: ns3t
status:
  phase: Terminating
`
		stateSecretsNeutral = `
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: neutral
  namespace: default
data:
  supersecret: YWJj #abc
`

		stateSecretsOriginal = `
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s1
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: s1data
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s2
  namespace: default
  labels:
    secret-copier.deckhouse.io/enabled: ""
    antiopa-secret-copier: "yes"
data:
  supersecret: s2data
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s3
  namespace: default
  labels:
    antiopa-secret-copier: "yes"
data:
  supersecret: s3data
`
		stateSecretsExtra = `
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: es1
  namespace: ns1
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: es1data
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: es2
  namespace: ns2
  labels:
    antiopa-secret-copier: "yes"
data:
  supersecret: es2data
`
		stateSecretsUpToDate = `
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s1
  namespace: ns1
  labels:
    secret-copier.deckhouse.io/enabled: ""
data:
  supersecret: s1data
`
		stateSecretsOutDated = `
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
  supersecret: old_s2_data
`
	)

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
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaces + stateSecretsOriginal + stateSecretsNeutral + stateSecretsExtra + stateSecretsOutDated + stateSecretsUpToDate))
			f.RunHook()
		})

		It("Six secrets must be actual", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesResource("Secret", "ns1", "es1").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "ns2", "es2").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("Secret", "ns1", "s1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns1", "s2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns1", "s3").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns2", "s1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns2", "s2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns2", "s3").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns3t", "s1").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "ns3t", "s2").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "ns3t", "s3").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("Secret", "ns1", "s1").Field("data.supersecret").String()).To(Equal("s1data"))
			Expect(f.KubernetesResource("Secret", "ns1", "s2").Field("data.supersecret").String()).To(Equal("s2data"))
			Expect(f.KubernetesResource("Secret", "ns1", "s3").Field("data.supersecret").String()).To(Equal("s3data"))
			Expect(f.KubernetesResource("Secret", "ns2", "s1").Field("data.supersecret").String()).To(Equal("s1data"))
			Expect(f.KubernetesResource("Secret", "ns2", "s2").Field("data.supersecret").String()).To(Equal("s2data"))
			Expect(f.KubernetesResource("Secret", "ns2", "s3").Field("data.supersecret").String()).To(Equal("s3data"))
		})
	})

	Context("Namespaces and all types of secrets are in cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(stateNamespaces + stateSecretsOriginal + stateSecretsNeutral + stateSecretsExtra + stateSecretsOutDated + stateSecretsUpToDate)
			f.BindingContexts.Set(f.RunSchedule("0 3 * * *"))
			f.RunHook()
		})

		It("Six secrets must be actual", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesResource("Secret", "ns1", "es1").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "ns2", "es2").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("Secret", "ns1", "s1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns1", "s2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns1", "s3").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns2", "s1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns2", "s2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns2", "s3").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "ns3t", "s1").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "ns3t", "s2").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "ns3t", "s3").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("Secret", "ns1", "s1").Field("data.supersecret").String()).To(Equal("s1data"))
			Expect(f.KubernetesResource("Secret", "ns1", "s2").Field("data.supersecret").String()).To(Equal("s2data"))
			Expect(f.KubernetesResource("Secret", "ns1", "s3").Field("data.supersecret").String()).To(Equal("s3data"))
			Expect(f.KubernetesResource("Secret", "ns2", "s1").Field("data.supersecret").String()).To(Equal("s1data"))
			Expect(f.KubernetesResource("Secret", "ns2", "s2").Field("data.supersecret").String()).To(Equal("s2data"))
			Expect(f.KubernetesResource("Secret", "ns2", "s3").Field("data.supersecret").String()).To(Equal("s3data"))
		})
	})
})
