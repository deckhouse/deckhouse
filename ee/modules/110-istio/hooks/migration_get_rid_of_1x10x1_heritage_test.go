/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const kialiCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: monitoringdashboards.monitoring.kiali.io
  labels:
    app: kiali
spec:
  group: monitoring.kiali.io
  names:
    kind: MonitoringDashboard
    listKind: MonitoringDashboardList
    plural: monitoringdashboards
    singular: monitoringdashboard
  scope: Namespaced
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
`

const istio1x10x1ValidationWebhook = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: istiod-d8-istio
webhooks:
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: xxx=
    service:
      name: istiod
      namespace: d8-istio
      path: /validate
      port: 443
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: validation.istio.io
  namespaceSelector: {}
  objectSelector: {}
  rules:
  - apiGroups:
    - security.istio.io
    - networking.istio.io
    apiVersions:
    - '*'
    operations:
    - CREATE
    - UPDATE
    resources:
    - '*'
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
`

var _ = Describe("Modules :: istio :: hooks :: migration_remove_kiali_crd ", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
	Context("CRD monitoringdashboards.monitoring.kiali.io exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kialiCRD))
			f.RunHook()
		})
		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "monitoringdashboards.monitoring.kiali.io").Exists()).To(BeFalse())
		})
	})
	Context("Istio 1.10.1 validation webhook istiod-d8-istio exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(istio1x10x1ValidationWebhook))
			f.RunHook()
		})
		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "istiod-d8-istio").Exists()).To(BeFalse())
		})
	})
})
