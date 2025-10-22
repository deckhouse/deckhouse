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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: common :: hooks :: ensure_crds ::", func() {
	const (
		installedCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  creationTimestamp: null
  labels:
    heritage: deckhouse
  name: testcrds.deckhouse.io
spec:
  group: deckhouse.io
  names:
    kind: TestCrd
    plural: testcrds
    singular: testcrd
  scope: Cluster
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Test CRD
        properties:
          spec:
            properties:
              a:
                description: a
                type: string
              b:
                type: string
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null
`
		existInClusterCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: testcrds.deckhouse.io
  labels:
    heritage: deckhouse
spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        caBundle: Q0FDQUNBCg== # CACACA
        service:
          name: webhook-handler
          namespace: system
          path: /testcrds.deckhouse.io
          port: 443
      conversionReviewVersions:
      - v1
  group: deckhouse.io
  scope: Cluster
  names:
    plural: testcrds
    singular: testcrd
    kind: TestCrd
  preserveUnknownFields: false
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          description: 'Test CRD'
          required:
            - spec
          properties:
            spec:
              type: object
              properties:
                a:
                  type: string
                c:
                  type: string
`
		withPreservedConversionCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  creationTimestamp: null
  labels:
    heritage: deckhouse
  name: testcrds.deckhouse.io
spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        caBundle: Q0FDQUNBCg== # CACACA
        service:
          name: webhook-handler
          namespace: system
          path: /testcrds.deckhouse.io
          port: 443
      conversionReviewVersions:
      - v1
  group: deckhouse.io
  names:
    kind: TestCrd
    plural: testcrds
    singular: testcrd
  scope: Cluster
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Test CRD
        properties:
          spec:
            properties:
              a:
                description: a
                type: string
              b:
                type: string
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").ToYaml()).To(MatchYAML(installedCRD))
		})

	})

	Context("Cluster with existing crd", func() {
		BeforeEach(func() {
			f.KubeStateSet(existInClusterCRD)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})
		It("Hook must not fail, spec.strategy must be preserved", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").ToYaml()).To(MatchYAML(withPreservedConversionCRD))
		})
	})

})
