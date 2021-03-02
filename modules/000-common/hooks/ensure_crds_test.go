package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

var _ = Describe("Modules :: common :: hooks :: ensure_crds ::", func() {
	const (
		properCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: testcrds.deckhouse.io
  labels:
    heritage: deckhouse
spec:
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
                b:
                  type: string
`
		existCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: testcrds.deckhouse.io
  labels:
    heritage: deckhouse
spec:
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
                b:
                  type: string
                c:
                  type: string
`
	)
	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(OnStartupContext)
			f.RunHook()
		})

		It("Hook must not fail, CRD should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			crd := f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io")
			Expect(crd.Exists()).To(BeTrue())
			Expect(crd.ToYaml()).To(MatchYAML(properCRD))
		})

	})

	Context("Cluster with existing crd", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(OnStartupContext)
			crd := make(map[string]interface{})
			err := yaml.Unmarshal([]byte(existCRD), &crd)
			Expect(err).To(BeNil())
			f.ObjectStore.PutObject(crd, object_store.NewMetaIndex("CustomResourceDefinition", "", "testcrds.deckhouse.io"))
			f.RunHook()
		})
		It("Hook must not fail, CRD should be replaced", func() {
			Expect(f).To(ExecuteSuccessfully())
			crd := f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io")
			Expect(crd.Exists()).To(BeTrue())
			Expect(crd.ToYaml()).To(MatchYAML(properCRD))
		})
	})

})
