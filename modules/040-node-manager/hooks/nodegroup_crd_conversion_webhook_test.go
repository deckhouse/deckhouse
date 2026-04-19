/*
Copyright 2025 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: nodegroup crd conversion webhook ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)

	const (
		caBundleBase64 = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCmZha2UtY2EtY2VydAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="

		nodeControllerTLSSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: node-controller-webhook-tls
  namespace: d8-cloud-instance-manager
type: kubernetes.io/tls
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCmZha2UtY2EtY2VydAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCmZha2UtdGxzLWNlcnQKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  tls.key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpmYWtlLWtleQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
`
		nodeControllerService = `
---
apiVersion: v1
kind: Service
metadata:
  name: node-controller-webhook
  namespace: d8-cloud-instance-manager
spec:
  ports:
    - port: 443
      targetPort: 9443
  selector:
    app: node-controller
`
		nodegroupsCRDNoConversion = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nodegroups.deckhouse.io
  labels:
    heritage: deckhouse
    module: node-manager
spec:
  group: deckhouse.io
  names:
    kind: NodeGroup
    listKind: NodeGroupList
    plural: nodegroups
    singular: nodegroup
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
`
		nodegroupsCRDWithOldConversion = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nodegroups.deckhouse.io
  labels:
    heritage: deckhouse
    module: node-manager
spec:
  group: deckhouse.io
  names:
    kind: NodeGroup
    listKind: NodeGroupList
    plural: nodegroups
    singular: nodegroup
  scope: Cluster
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        caBundle: b2xkLWNhLWJ1bmRsZQ==
        service:
          namespace: d8-cloud-instance-manager
          name: conversion-webhook-handler
          path: /convert
      conversionReviewVersions:
        - v1
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
`
	)

	Context("No CRD exists", func() {
		BeforeEach(func() {
			f.KubeStateSet(nodeControllerTLSSecret + nodeControllerService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Hook should execute successfully and do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("CRD exists but no TLS secret", func() {
		BeforeEach(func() {
			f.KubeStateSet(nodegroupsCRDNoConversion + nodeControllerService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Hook should execute successfully and do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("CRD exists but no webhook service", func() {
		BeforeEach(func() {
			f.KubeStateSet(nodegroupsCRDNoConversion + nodeControllerTLSSecret)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Hook should execute successfully and do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("CRD without conversion, TLS secret and service all exist", func() {
		BeforeEach(func() {
			f.KubeStateSet(nodegroupsCRDNoConversion + nodeControllerTLSSecret + nodeControllerService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Should patch CRD with conversion webhook pointing to node-controller", func() {
			Expect(f).To(ExecuteSuccessfully())

			crd := f.KubernetesResource("CustomResourceDefinition", "", "nodegroups.deckhouse.io")

			Expect(crd.Field(`spec.conversion.strategy`).String()).To(Equal("Webhook"))
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.caBundle`).Exists()).To(BeTrue())
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.service.name`).String()).To(Equal("node-controller-webhook"))
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.service.namespace`).String()).To(Equal("d8-cloud-instance-manager"))
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.service.path`).String()).To(Equal("/convert"))
		})
	})

	Context("CRD with old conversion webhook, TLS secret and service all exist", func() {
		BeforeEach(func() {
			f.KubeStateSet(nodegroupsCRDWithOldConversion + nodeControllerTLSSecret + nodeControllerService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Should update CRD conversion webhook to node-controller with new CA bundle", func() {
			Expect(f).To(ExecuteSuccessfully())

			crd := f.KubernetesResource("CustomResourceDefinition", "", "nodegroups.deckhouse.io")

			Expect(crd.Field(`spec.conversion.strategy`).String()).To(Equal("Webhook"))
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.caBundle`).String()).To(Equal(caBundleBase64))
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.service.name`).String()).To(Equal("node-controller-webhook"))
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.service.namespace`).String()).To(Equal("d8-cloud-instance-manager"))
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.service.path`).String()).To(Equal("/convert"))
		})
	})

	Context("CRD already has correct CA bundle from node-controller", func() {
		BeforeEach(func() {
			crdWithCorrectCA := `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nodegroups.deckhouse.io
  labels:
    heritage: deckhouse
    module: node-manager
spec:
  group: deckhouse.io
  names:
    kind: NodeGroup
    listKind: NodeGroupList
    plural: nodegroups
    singular: nodegroup
  scope: Cluster
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        caBundle: ` + caBundleBase64 + `
        service:
          namespace: d8-cloud-instance-manager
          name: node-controller-webhook
          path: /convert
          port: 443
      conversionReviewVersions:
        - v1
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
`
			f.KubeStateSet(crdWithCorrectCA + nodeControllerTLSSecret + nodeControllerService)
			f.BindingContexts.Set(f.GenerateAfterAllContext())
			f.RunHook()
		})
		It("Should execute successfully without patching (CA already matches)", func() {
			Expect(f).To(ExecuteSuccessfully())

			crd := f.KubernetesResource("CustomResourceDefinition", "", "nodegroups.deckhouse.io")
			Expect(crd.Field(`spec.conversion.webhook.clientConfig.service.name`).String()).To(Equal("node-controller-webhook"))
		})
	})
})
