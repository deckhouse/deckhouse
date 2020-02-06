package hooks

import (
	"encoding/base64"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: order_bootstrap_token ::", func() {
	var (
		stateTokenJunk = `
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
  creationTimestamp: "2000-01-01T00:00:00Z"
  labels:
    heritage: deckhouse
    module: cloud-instance-manager
  name: bootstrap-token-junk
  namespace: kube-system
data: {}
`

		stateTokenExpired = `
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
  creationTimestamp: "2020-01-01T00:00:00Z"
  labels:
    heritage: deckhouse
    module: cloud-instance-manager
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

		stateTokenAlmostExpired = `
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
 creationTimestamp: "2020-01-02T00:00:00Z"
 labels:
   heritage: deckhouse
   module: cloud-instance-manager
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

		stateTokenActual = `
---
apiVersion: v1
kind: Secret
type: bootstrap.kubernetes.io/token
metadata:
 creationTimestamp: "2020-01-03T00:00:00Z"
 labels:
   heritage: deckhouse
   module: cloud-instance-manager
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
	)

	f := HookExecutionConfigInit(`{"cloudInstanceManager":{"internal": {}}}`, `{}`)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("A new token must have generated.", func() {
			Expect(f).To(ExecuteSuccessfully())

			bootstrapToken := f.ValuesGet("cloudInstanceManager.internal.bootstrapToken").String()
			Expect(len(bootstrapToken)).To(Equal(23))

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
			Expect(string(authExtraGroupsBytes)).To(Equal("system:bootstrappers:kubeadm:default-node-token,system:bootstrappers:cloud-instance-manager:machine-bootstrap"))

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

	Context("Cluster has expired token", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateTokenExpired))
			f.RunHook()
		})

		It("Expired token must be deleted. A new token must have generated.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(len(f.BindingContexts.Get("0.snapshots.bootstrap_tokens").Array())).To(Equal(1))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.0.filterResult.validFor").Int()).To(BeNumerically("<", 0))
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-aaaaaa").Exists()).To(BeFalse())

			bootstrapToken := f.ValuesGet("cloudInstanceManager.internal.bootstrapToken").String()
			Expect(len(bootstrapToken)).To(Equal(23))

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
			Expect(string(authExtraGroupsBytes)).To(Equal("system:bootstrappers:kubeadm:default-node-token,system:bootstrappers:cloud-instance-manager:machine-bootstrap"))

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

	Context("Cluster has expired and almost expired tokens", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateTokenExpired + stateTokenAlmostExpired))
			f.RunHook()
		})

		It("Expired token must be deleted. Almost expired token must be kept. A new token must have generated.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(len(f.BindingContexts.Get("0.snapshots.bootstrap_tokens").Array())).To(Equal(2))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.0.filterResult.validFor").Int()).To(BeNumerically("<", 3300))
			Expect(f.BindingContexts.Get("0.snapshots.bootstrap_tokens.1.filterResult.validFor").Int()).To(BeNumerically(">", 3300))
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-aaaaaa").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "kube-system", "bootstrap-token-kkkkkk").Exists()).To(BeTrue())

			bootstrapToken := f.ValuesGet("cloudInstanceManager.internal.bootstrapToken").String()
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
			Expect(string(authExtraGroupsBytes)).To(Equal("system:bootstrappers:kubeadm:default-node-token,system:bootstrappers:cloud-instance-manager:machine-bootstrap"))

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

	Context("Cluster has expired, almost expired, actual and junk tokens", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateTokenExpired + stateTokenAlmostExpired + stateTokenActual + stateTokenJunk))
			f.RunHook()
		})

		It("Expired token must be deleted. Almost expired token must be kept. Actual token must be stored to values.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(len(f.BindingContexts.Get("0.snapshots.bootstrap_tokens").Array())).To(Equal(4))
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

			// There are only three tokens left. Nothing was added.
			Expect(len(f.ObjectStore)).To(Equal(3))

			bootstrapToken := f.ValuesGet("cloudInstanceManager.internal.bootstrapToken").String()
			Expect(bootstrapToken).To(Equal("ssssss.ssssssssssssssss"))
		})
	})

	Context("Cluster has expired, almost expired and actual tokens. Crontab ticked.", func() {
		BeforeEach(func() {
			f.KubeStateSet(stateTokenExpired + stateTokenAlmostExpired + stateTokenActual + stateTokenJunk)
			f.BindingContexts.Set(ScheduleBindingContext("bootstrap_tokens_cron"))
			f.BindingContexts.Set(f.RunSchedule("23 * * * *"))
			f.RunHook()
		})

		It("Expired token must be deleted. Almost expired token must be kept. Actual token must be stored to values.", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(len(f.BindingContexts.Get("0.snapshots.bootstrap_tokens").Array())).To(Equal(4))
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

			// There are only three tokens left. Nothing was added.
			Expect(len(f.ObjectStore)).To(Equal(3))

			bootstrapToken := f.ValuesGet("cloudInstanceManager.internal.bootstrapToken").String()
			Expect(bootstrapToken).To(Equal("ssssss.ssssssssssssssss"))
		})
	})
})
