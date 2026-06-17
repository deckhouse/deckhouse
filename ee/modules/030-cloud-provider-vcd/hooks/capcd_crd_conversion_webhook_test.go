/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vcd :: hooks :: capcd crd conversion webhook ::", func() {
	f := HookExecutionConfigInit(`{"cloudProviderVcd":{"internal": {}}}`, `{}`)

	const (
		caBundleBase64 = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCmZha2UtY2EtY2VydAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="

		capcdWebhookTLSSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: capcd-controller-manager-webhook-tls
  namespace: d8-cloud-provider-vcd
type: kubernetes.io/tls
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCmZha2UtY2EtY2VydAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCmZha2UtdGxzLWNlcnQKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  tls.key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpmYWtlLWtleQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
`
		capcdWebhookService = `
---
apiVersion: v1
kind: Service
metadata:
  name: capcd-controller-manager-webhook-service
  namespace: d8-cloud-provider-vcd
spec:
  ports:
    - port: 443
      targetPort: webhook-server
  selector:
    app: capcd-controller-manager
`
		vcdClusterCRDWithoutCA = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: vcdclusters.infrastructure.cluster.x-k8s.io
  labels:
    heritage: deckhouse
    module: cloud-provider-vcd
spec:
  group: infrastructure.cluster.x-k8s.io
  names:
    kind: VCDCluster
    listKind: VCDClusterList
    plural: vcdclusters
    singular: vcdcluster
  scope: Namespaced
  conversion:
    strategy: Webhook
    webhook:
      conversionReviewVersions:
        - v1
        - v1beta1
      clientConfig:
        service:
          namespace: d8-cloud-provider-vcd
          name: capcd-controller-manager-webhook-service
          path: /convert
  versions:
    - name: v1alpha1
      served: true
      storage: false
      schema:
        openAPIV3Schema:
          type: object
    - name: v1beta2
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`
		vcdClusterCRDWithOldCA = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: vcdclusters.infrastructure.cluster.x-k8s.io
  labels:
    heritage: deckhouse
    module: cloud-provider-vcd
spec:
  group: infrastructure.cluster.x-k8s.io
  names:
    kind: VCDCluster
    listKind: VCDClusterList
    plural: vcdclusters
    singular: vcdcluster
  scope: Namespaced
  conversion:
    strategy: Webhook
    webhook:
      conversionReviewVersions:
        - v1
        - v1beta1
      clientConfig:
        caBundle: b2xkLWNhLWJ1bmRsZQ==
        service:
          namespace: d8-cloud-provider-vcd
          name: capcd-controller-manager-webhook-service
          path: /convert
  versions:
    - name: v1alpha1
      served: true
      storage: false
      schema:
        openAPIV3Schema:
          type: object
    - name: v1beta2
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`
	)

	Context("No CRD exists", func() {
		BeforeEach(func() {
			f.KubeStateSet(capcdWebhookTLSSecret + capcdWebhookService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Hook should execute successfully and do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("CRD exists but no TLS secret", func() {
		BeforeEach(func() {
			f.KubeStateSet(vcdClusterCRDWithoutCA + capcdWebhookService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Hook should execute successfully and do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("CRD exists but no webhook service", func() {
		BeforeEach(func() {
			f.KubeStateSet(vcdClusterCRDWithoutCA + capcdWebhookTLSSecret)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Hook should execute successfully and do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("CRD exists without CA bundle, all resources present", func() {
		BeforeEach(func() {
			f.KubeStateSet(vcdClusterCRDWithoutCA + capcdWebhookTLSSecret + capcdWebhookService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Hook should execute successfully and patch the CRD", func() {
			Expect(f).To(ExecuteSuccessfully())

			crd := f.KubernetesGlobalResource("CustomResourceDefinition", "vcdclusters.infrastructure.cluster.x-k8s.io")
			Expect(crd.Exists()).To(BeTrue())
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.caBundle`).Exists()).To(BeTrue())
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.caBundle`).String()).To(Equal(caBundleBase64))
		})
	})

	Context("CRD exists with old CA bundle, all resources present", func() {
		BeforeEach(func() {
			f.KubeStateSet(vcdClusterCRDWithOldCA + capcdWebhookTLSSecret + capcdWebhookService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Hook should execute successfully and update the CA bundle", func() {
			Expect(f).To(ExecuteSuccessfully())

			crd := f.KubernetesGlobalResource("CustomResourceDefinition", "vcdclusters.infrastructure.cluster.x-k8s.io")
			Expect(crd.Exists()).To(BeTrue())
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.caBundle`).Exists()).To(BeTrue())
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.caBundle`).String()).To(Equal(caBundleBase64))
		})
	})

	Context("CRD already has correct CA bundle", func() {
		BeforeEach(func() {
			vcdClusterCRDWithCorrectCA := `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: vcdclusters.infrastructure.cluster.x-k8s.io
  labels:
    heritage: deckhouse
    module: cloud-provider-vcd
spec:
  group: infrastructure.cluster.x-k8s.io
  names:
    kind: VCDCluster
    listKind: VCDClusterList
    plural: vcdclusters
    singular: vcdcluster
  scope: Namespaced
  conversion:
    strategy: Webhook
    webhook:
      conversionReviewVersions:
        - v1
        - v1beta1
      clientConfig:
        caBundle: ` + caBundleBase64 + `
        service:
          namespace: d8-cloud-provider-vcd
          name: capcd-controller-manager-webhook-service
          path: /convert
  versions:
    - name: v1alpha1
      served: true
      storage: false
      schema:
        openAPIV3Schema:
          type: object
    - name: v1beta2
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`
			f.KubeStateSet(vcdClusterCRDWithCorrectCA + capcdWebhookTLSSecret + capcdWebhookService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Hook should execute successfully and not patch the CRD", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "vcdclusters.infrastructure.cluster.x-k8s.io").Exists()).To(BeTrue())
		})
	})
})
