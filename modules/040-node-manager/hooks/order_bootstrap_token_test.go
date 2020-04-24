package hooks

import (
	"encoding/base64"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var stateNGStatic = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: static0
spec:
  nodeType: Static
`

var stateNGCloud = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: cloud0
spec:
  nodeType: Cloud
`

var stateTokenJunk = `
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
  creationTimestamp: "2000-01-01T00:00:00Z"
  labels:
    node-manager.deckhouse.io/node-group: some-ng
  name: bootstrap-token-junk
  namespace: kube-system
data: {}
`

func stateTokenExpired() string {
	return `
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
  creationTimestamp: "2020-01-01T00:00:00Z"
  labels:
    node-manager.deckhouse.io/node-group: static0
  name: bootstrap-token-aaaaaa
  namespace: kube-system
data:
  auth-extra-groups: c3lzdGVtOmJvb3RzdHJhcHBlcnM6a3ViZWFkbTpkZWZhdWx0LW5vZGUtdG9rZW4sc3lzdGVtOmJvb3RzdHJhcHBlcnM6Y2xvdWQtaW5zdGFuY2UtbWFuYWdlcjptYWNoaW5lLWJvb3RzdHJhcA==
  expiration: ` + base64.StdEncoding.EncodeToString([]byte(time.Now().UTC().Add(-time.Hour).Format(time.RFC3339))) + `
  token-id: YWFhYWFh # aaaaaa
  token-secret: YWFhYWFhYWFhYWFhYWFhYQ== # aaaaaaaaaaaaaaaa
  usage-bootstrap-authentication: dHJ1ZQ==
  usage-bootstrap-signing: dHJ1ZQ==
`
}

func stateTokenAlmostExpired() string {
	return `
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
 creationTimestamp: "2020-01-02T00:00:00Z"
 labels:
    node-manager.deckhouse.io/node-group: static0
 name: bootstrap-token-kkkkkk
 namespace: kube-system
data:
 auth-extra-groups: c3lzdGVtOmJvb3RzdHJhcHBlcnM6a3ViZWFkbTpkZWZhdWx0LW5vZGUtdG9rZW4sc3lzdGVtOmJvb3RzdHJhcHBlcnM6Y2xvdWQtaW5zdGFuY2UtbWFuYWdlcjptYWNoaW5lLWJvb3RzdHJhcA==
 expiration: ` + base64.StdEncoding.EncodeToString([]byte(time.Now().UTC().Add(time.Hour).Format(time.RFC3339))) + `
 token-id: a2tra2tr # kkkkkk
 token-secret: a2tra2tra2tra2tra2traw== # kkkkkkkkkkkkkkkk
 usage-bootstrap-authentication: dHJ1ZQ==
 usage-bootstrap-signing: dHJ1ZQ==
`
}

func stateTokenActual() string {
	return `
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
 creationTimestamp: "2020-01-03T00:00:00Z"
 labels:
    node-manager.deckhouse.io/node-group: static0
 name: bootstrap-token-ssssss
 namespace: kube-system
data:
 auth-extra-groups: c3lzdGVtOmJvb3RzdHJhcHBlcnM6a3ViZWFkbTpkZWZhdWx0LW5vZGUtdG9rZW4sc3lzdGVtOmJvb3RzdHJhcHBlcnM6Y2xvdWQtaW5zdGFuY2UtbWFuYWdlcjptYWNoaW5lLWJvb3RzdHJhcA==
 expiration: ` + base64.StdEncoding.EncodeToString([]byte(time.Now().UTC().Add(5*time.Hour).Format(time.RFC3339))) + `
 token-id: c3Nzc3Nz # sssss
 token-secret: c3Nzc3Nzc3Nzc3Nzc3Nzcw== # ssssssssssssssss
 usage-bootstrap-authentication: dHJ1ZQ==
 usage-bootstrap-signing: dHJ1ZQ==
`
}

var _ = Describe("Modules :: node-group :: hooks :: order_bootstrap_token ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail, new token must not have generated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.bootstrapTokens").String()).To(Equal("{}"))
		})
	})

	Context("Cluster has two NodeGroups with nodeType=Static and nodeType=Cloud", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGStatic + stateNGCloud))
			f.RunHook()
		})

		It("A new token for NG static0 must have generated.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("nodeManager.internal.bootstrapTokens").Map()).To(HaveLen(1))

			bootstrapToken := f.ValuesGet("nodeManager.internal.bootstrapTokens.static0").String()
			Expect(bootstrapToken).To(HaveLen(23))

			tokenSlice := strings.Split(bootstrapToken, ".")
			tokenID := tokenSlice[0]
			tokenSecret := tokenSlice[1]

			tokenResource := f.KubernetesResource("Secret", "kube-system", "bootstrap-token-"+tokenID)
			Expect(tokenResource.Exists()).To(BeTrue())

			tokenIDBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.token-id").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(tokenIDBytes)).To(Equal(tokenID))

			tokenSecretBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.token-secret").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(tokenSecretBytes)).To(Equal(tokenSecret))

			authExtraGroupsBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.auth-extra-groups").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(authExtraGroupsBytes)).To(Equal("system:bootstrappers:kubeadm:default-node-token,system:bootstrappers:node-manager:static0"))

			usageBootstrapAuthenticationBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.usage-bootstrap-authentication").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(usageBootstrapAuthenticationBytes)).To(Equal("true"))

			usageBootstrapSigningBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.usage-bootstrap-signing").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(usageBootstrapSigningBytes)).To(Equal("true"))

			experationBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.expiration").String())
			Expect(err).ShouldNot(HaveOccurred())
			t, err := time.Parse(time.RFC3339, string(experationBytes))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(t).Should(BeTemporally("~", time.Now().UTC().Add(time.Hour*4), time.Minute))
		})
	})

	Context("Cluster has expired token and two NodeGroups with nodeType=Static and nodeType=Cloud", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateTokenExpired() + stateNGCloud + stateNGStatic))
			f.RunHook()
		})

		It("Expired token must be deleted. A new token for NodeGroup static0 must have generated.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.BindingContexts.Get("0.snapshots.ngs").Array()).To(HaveLen(2))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens").Array()).To(HaveLen(1))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.0.filterResult.validFor").Int()).To(BeNumerically("<", 0))
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-aaaaaa").Exists()).To(BeFalse())

			Expect(f.ValuesGet("nodeManager.internal.bootstrapTokens").Map()).To(HaveLen(1))

			bootstrapToken := f.ValuesGet("nodeManager.internal.bootstrapTokens.static0").String()
			Expect(bootstrapToken).To(HaveLen(23))

			tokenSlice := strings.Split(bootstrapToken, ".")
			tokenID := tokenSlice[0]
			tokenSecret := tokenSlice[1]

			tokenResource := f.KubernetesResource("Secret", "kube-system", "bootstrap-token-"+tokenID)
			Expect(tokenResource.Exists()).To(BeTrue())

			tokenIDBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.token-id").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(tokenIDBytes)).To(Equal(tokenID))

			tokenSecretBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.token-secret").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(tokenSecretBytes)).To(Equal(tokenSecret))

			authExtraGroupsBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.auth-extra-groups").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(authExtraGroupsBytes)).To(Equal("system:bootstrappers:kubeadm:default-node-token,system:bootstrappers:node-manager:static0"))

			usageBootstrapAuthenticationBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.usage-bootstrap-authentication").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(usageBootstrapAuthenticationBytes)).To(Equal("true"))

			usageBootstrapSigningBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.usage-bootstrap-signing").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(usageBootstrapSigningBytes)).To(Equal("true"))

			experationBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.expiration").String())
			Expect(err).ShouldNot(HaveOccurred())
			t, err := time.Parse(time.RFC3339, string(experationBytes))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(t).Should(BeTemporally("~", time.Now().UTC().Add(time.Hour*4), time.Minute))
		})
	})

	Context("Cluster has expired and almost expired tokens, also two nodes with nodeType = Cloud and Static", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateTokenExpired() + stateTokenAlmostExpired() + stateNGCloud + stateNGStatic))
			f.RunHook()
		})

		It("Expired token must be deleted. Almost expired token must be kept. A new token must have generated.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.BindingContexts.Get("0.snapshots.ngs").Array()).To(HaveLen(2))

			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens").Array()).To(HaveLen(2))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.0.filterResult.validFor").Int()).To(BeNumerically("<", 3300))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.1.filterResult.validFor").Int()).To(BeNumerically(">", 3300))
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-aaaaaa").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-kkkkkk").Exists()).To(BeTrue())

			Expect(f.ValuesGet("nodeManager.internal.bootstrapTokens").Map()).To(HaveLen(1))

			bootstrapToken := f.ValuesGet("nodeManager.internal.bootstrapTokens.static0").String()
			Expect(len(bootstrapToken)).To(Equal(23))
			Expect(bootstrapToken).ToNot(Equal("kkkkkk.kkkkkkkkkkkkkkkk"))

			tokenSlice := strings.Split(bootstrapToken, ".")
			tokenID := tokenSlice[0]
			tokenSecret := tokenSlice[1]

			tokenResource := f.KubernetesResource("Secret", "kube-system", "bootstrap-token-"+tokenID)
			Expect(tokenResource.Exists()).To(BeTrue())

			tokenIDBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.token-id").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(tokenIDBytes)).To(Equal(tokenID))

			tokenSecretBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.token-secret").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(tokenSecretBytes)).To(Equal(tokenSecret))

			authExtraGroupsBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.auth-extra-groups").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(authExtraGroupsBytes)).To(Equal("system:bootstrappers:kubeadm:default-node-token,system:bootstrappers:node-manager:static0"))

			usageBootstrapAuthenticationBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.usage-bootstrap-authentication").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(usageBootstrapAuthenticationBytes)).To(Equal("true"))

			usageBootstrapSigningBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.usage-bootstrap-signing").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(usageBootstrapSigningBytes)).To(Equal("true"))

			experationBytes, err := base64.StdEncoding.DecodeString(tokenResource.Field("data.expiration").String())
			Expect(err).ShouldNot(HaveOccurred())
			t, err := time.Parse(time.RFC3339, string(experationBytes))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(t).Should(BeTemporally("~", time.Now().UTC().Add(time.Hour*4), time.Minute))
		})
	})

	Context("Cluster has expired, almost expired, actual and junk tokens, also two NodeGroups with nodeType = Cloud and Static ", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateTokenExpired() + stateTokenAlmostExpired() + stateTokenActual() + stateTokenJunk + stateNGStatic + stateNGCloud))
			f.RunHook()
		})

		It("Expired token must be deleted. Almost expired token must be kept. Actual token must be stored to values.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.BindingContexts.Get("0.snapshots.ngs").Array()).To(HaveLen(2))

			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens").Array()).To(HaveLen(4))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.0.filterResult.validFor").Int()).To(BeNumerically("<", -3300))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.0.filterResult.bootstrapToken").String()).To(Equal("aaaaaa.aaaaaaaaaaaaaaaa"))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.1.filterResult.validFor").Value()).To(BeNil())
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.1.filterResult.bootstrapToken").Value()).To(BeNil())
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.2.filterResult.validFor").Int()).To(BeNumerically(">", 3300))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.2.filterResult.bootstrapToken").String()).To(Equal("kkkkkk.kkkkkkkkkkkkkkkk"))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.3.filterResult.validFor").Int()).To(BeNumerically(">", 17500))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.3.filterResult.bootstrapToken").String()).To(Equal("ssssss.ssssssssssssssss"))

			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-aaaaaa").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-kkkkkk").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-ssssss").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-junk").Exists()).To(BeTrue())

			// There are two NodeGroups and only three tokens left. Nothing was added.
			Expect(len(f.ObjectStore)).To(Equal(2 + 3))

			Expect(f.ValuesGet("nodeManager.internal.bootstrapTokens").Map()).To(HaveLen(1))

			bootstrapToken := f.ValuesGet("nodeManager.internal.bootstrapTokens.static0").String()
			Expect(bootstrapToken).To(Equal("ssssss.ssssssssssssssss"))
		})
	})

	Context("Cluster has expired, almost expired and actual tokens, also two NodeGroups with noodeType = Cloud and Static. Crontab ticked.", func() {
		BeforeEach(func() {
			f.KubeStateSet(stateTokenExpired() + stateTokenAlmostExpired() + stateTokenActual() + stateTokenJunk + stateNGStatic + stateNGCloud)
			f.BindingContexts.Set(ScheduleBindingContext("bootstrap_tokens_cron"))
			f.BindingContexts.Set(f.RunSchedule("23 * * * *"))
			f.RunHook()
		})

		It("Expired token must be deleted. Almost expired token must be kept. Actual token must be stored to values.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.BindingContexts.Get("0.snapshots.ngs").Array()).To(HaveLen(2))

			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens").Array()).To(HaveLen(4))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.0.filterResult.validFor").Int()).To(BeNumerically("<", -3300))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.0.filterResult.bootstrapToken").String()).To(Equal("aaaaaa.aaaaaaaaaaaaaaaa"))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.1.filterResult.validFor").Value()).To(BeNil())
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.1.filterResult.bootstrapToken").Value()).To(BeNil())
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.2.filterResult.validFor").Int()).To(BeNumerically(">", 3300))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.2.filterResult.bootstrapToken").String()).To(Equal("kkkkkk.kkkkkkkkkkkkkkkk"))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.3.filterResult.validFor").Int()).To(BeNumerically(">", 17500))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.3.filterResult.bootstrapToken").String()).To(Equal("ssssss.ssssssssssssssss"))

			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-aaaaaa").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-kkkkkk").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-ssssss").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-junk").Exists()).To(BeTrue())

			// There are two NodeGroups and only three tokens left. Nothing was added.
			Expect(len(f.ObjectStore)).To(Equal(2 + 3))

			Expect(f.ValuesGet("nodeManager.internal.bootstrapTokens").Map()).To(HaveLen(1))

			bootstrapToken := f.ValuesGet("nodeManager.internal.bootstrapTokens.static0").String()
			Expect(bootstrapToken).To(Equal("ssssss.ssssssssssssssss"))
		})
	})
})
