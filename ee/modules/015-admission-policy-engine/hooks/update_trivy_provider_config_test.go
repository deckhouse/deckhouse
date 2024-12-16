/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var (
	trivyAndProviderNs = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-operator-trivy
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-admission-policy-engine
`

	providerCm = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    heritage: deckhouse
    module: admission-policy-engine
  name: trivy-provider
  namespace: d8-admission-policy-engine
data:
  TRIVY_INSECURE: "false"
`
	trivyCmInsecureFalse = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: operator-trivy
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: operator-trivy
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: operator-trivy
  name: trivy-operator-trivy-config
  namespace: d8-operator-trivy
data:
  TRIVY_DEBUG: "false"
  TRIVY_INSECURE: "false"
  TRIVY_SKIP_DB_UPDATE: "false"
  trivy.additionalVulnerabilityReportFields: ""
`

	providerSts = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app: trivy-provider
    heritage: deckhouse
    module: admission-policy-engine
  name: trivy-provider
  namespace: d8-admission-policy-engine
spec:
  replicas: 1
  selector:
    matchLabels:
      app: trivy-provider
      app.kubernetes.io/part-of: gatekeeper
  template:
    metadata:
      annotations:
        checksum/config: 442f77dcf68414c00d900953dd287ff89192c2202babdf9f9007915e7a714b96
      creationTimestamp: null
      labels:
        app: trivy-provider
        app.kubernetes.io/part-of: gatekeeper
    spec:
      containers:
      - args:
        - --port=8443
        image: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:d3080108cfa5d1165069807ec61c790027e01f962e904f2a3ad7091f0a639c45
`
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: trivy provider config ::", func() {

	Context(":: empty cluster", func() {
		f := HookExecutionConfigInit(`{"admissionPolicyEngine": { "internal": {} }}`, "")
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			cm := f.KubernetesResource("ConfigMap", "d8-admission-policy-engine", "trivy-provider")
			Expect(cm.Exists()).To(BeFalse())
		})
	})

	Context(":: empty cluster with operator-trivy module enabled", func() {
		f := HookExecutionConfigInit(`{"global": {"enabledModules": ["operator-trivy", "foo-bar"]}, "admissionPolicyEngine": { "internal": {} }}`, "")
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			cm := f.KubernetesResource("ConfigMap", "d8-admission-policy-engine", "trivy-provider")
			Expect(cm.Exists()).To(BeFalse())
		})
	})

	Context(":: empty cluster with provider enabled", func() {
		f := HookExecutionConfigInit(`{"global": {"enabledModules": ["foo-bar"]}, "admissionPolicyEngine": { "internal": {} }}`, `{"admissionPolicyEngine": {"denyVulnerableImages": {"enabled": true}}}`)
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			cm := f.KubernetesResource("ConfigMap", "d8-admission-policy-engine", "trivy-provider")
			Expect(cm.Exists()).To(BeFalse())
		})
	})

	Context(":: empty cluster with operator-trivy enabled and provider enabled", func() {
		f := HookExecutionConfigInit(`{"global": {"enabledModules": ["operator-trivy", "foo-bar"]}, "admissionPolicyEngine": { "internal": {} }}`, `{"admissionPolicyEngine": {"denyVulnerableImages": {"enabled": true}}}`)
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trivyConfigData").String()).To(MatchJSON(`{"TRIVY_INSECURE":"false"}`))
		})
	})

	Context(":: cluster with trivy configmap, but no provider statefulset", func() {
		f := HookExecutionConfigInit(`{"global": {"enabledModules": ["operator-trivy", "foo-bar"]}, "admissionPolicyEngine": { "internal": {} }}`, `{"admissionPolicyEngine": {"denyVulnerableImages": {"enabled": true}}}`)
		BeforeEach(func() {
			f.KubeStateSet(trivyAndProviderNs + trivyCmInsecureFalse)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config").Exists()).To(BeTrue())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trivyConfigData").String()).To(MatchJSON(`{"TRIVY_INSECURE": "false"}`))
		})
	})

	Context(":: cluster with trivy configmap and provider statefulset", func() {
		f := HookExecutionConfigInit(`{"global": {"enabledModules": ["operator-trivy", "foo-bar"]}, "admissionPolicyEngine": { "internal": {} }}`, `{"admissionPolicyEngine": {"denyVulnerableImages": {"enabled": true}}}`)
		BeforeEach(func() {
			f.KubeStateSet(trivyAndProviderNs + trivyCmInsecureFalse + providerSts)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config").Exists()).To(BeTrue())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trivyConfigData").String()).To(MatchJSON(`{"TRIVY_INSECURE": "false"}`))
		})
	})

	Context(":: cluster with equal trivy and provider configmaps, and provider statefulset", func() {
		f := HookExecutionConfigInit(`{"global": {"enabledModules": ["operator-trivy", "foo-bar"]}, "admissionPolicyEngine": { "internal": {} }}`, `{"admissionPolicyEngine": {"denyVulnerableImages": {"enabled": true}}}`)
		BeforeEach(func() {
			f.KubeStateSet(trivyAndProviderNs + trivyCmInsecureFalse + providerSts + providerCm)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config").Exists()).To(BeTrue())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trivyConfigData").String()).To(MatchJSON(`{"TRIVY_INSECURE": "false"}`))
		})
	})

	Context(":: cluster with equal trivy and provider configmaps, provider statefulset and custom CA set", func() {
		f := HookExecutionConfigInit(`{"global": {"modulesImages": {"registry": {"CA": "123"}}, "enabledModules": ["operator-trivy", "foo-bar"]}, "admissionPolicyEngine": { "internal": {} }}`, `{"admissionPolicyEngine": {"denyVulnerableImages": {"enabled": true}}}`)
		BeforeEach(func() {
			f.KubeStateSet(trivyAndProviderNs + trivyCmInsecureFalse + providerSts + providerCm)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config").Exists()).To(BeTrue())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trivyConfigData").String()).To(MatchJSON(`{"TRIVY_INSECURE": "false", "TRIVY_REGISTRY_CA": "123"}`))
		})
	})

	Context(":: cluster with different trivy and provider configmaps, provider statefulset and no custom CA set", func() {
		f := HookExecutionConfigInit(`{"global": {"enabledModules": ["operator-trivy", "foo-bar"]}, "admissionPolicyEngine": { "internal": {} }}`, `{"admissionPolicyEngine": {"denyVulnerableImages": {"enabled": true}}}`)
		BeforeEach(func() {
			f.KubeStateSet(trivyAndProviderNs + `
---
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: operator-trivy
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: operator-trivy
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: operator-trivy
  name: trivy-operator-trivy-config
  namespace: d8-operator-trivy
data:
  TRIVY_DEBUG: "false"
  TRIVY_INSECURE: "true"
  TRIVY_SKIP_DB_UPDATE: "false"
  trivy.additionalVulnerabilityReportFields: ""
  trivy.insecureRegistry.0: "nexus.com"
` + providerSts + `
---
apiVersion: v1
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    heritage: deckhouse
    module: admission-policy-engine
  name: trivy-provider
  namespace: d8-admission-policy-engine
data:
  TRIVY_INSECURE: "false"
  TRIVY_REGISTRY_CA: "123"
  trivy.insecureRegistry.0: "example.com"
  trivy.insecureRegistry.1: "test.com"
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config").Exists()).To(BeTrue())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trivyConfigData").String()).To(MatchJSON(`{"TRIVY_INSECURE":"true","trivy.insecureRegistry.0": "nexus.com"}`))
		})
	})

	Context(":: cluster with operator-trivy disabled, but provider configmap exists", func() {
		f := HookExecutionConfigInit(`{"global": {"modulesImages": {"registry": {"CA": "123"}}, "enabledModules": ["foo-bar"]}, "admissionPolicyEngine": { "internal": {} }}`, `{"admissionPolicyEngine": {"denyVulnerableImages": {"enabled": true}}}`)
		BeforeEach(func() {
			f.KubeStateSet(trivyAndProviderNs + providerCm)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config").Exists()).To(BeFalse())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trivyConfigData").Exists()).To(BeFalse())
		})
	})
})
