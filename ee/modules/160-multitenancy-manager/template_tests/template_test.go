/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/releaseutil"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	initialValues = `
projects: []
projectTypes: {}
`

	userResourcesTemplate = "multitenancy-manager/templates/user-resources-templates.yaml"
)

var _ = Describe("Module :: multitenancy-manager :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("multitenancyManager.internal", initialValues)
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly for empty cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})
	})

	Context("One Project without ProjectType", func() {
		render := make(map[string]string)
		BeforeEach(func() {
			f.ValuesSetFromYaml("multitenancyManager.internal", stateOneProject+stateEmptyProjectTypes)
			f.HelmRender(WithRenderOutput(render))
		})

		It("Should return error", func() {
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).Should(ContainSubstring("No ProjectType with name 'pt1' found for Project 'project-1'."))
		})

	})

	Context("One Project and one ProjectType cluster", func() {
		render := make(map[string]string)
		BeforeEach(func() {
			f.ValuesSetFromYaml("multitenancyManager.internal", stateOneProjectType+stateOneProject)
			f.HelmRender(WithRenderOutput(render))
		})

		It("Everything must render properly for empty cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Saves initial NS for resource", func() {
			np := f.KubernetesResource("NetworkPolicy", "project-all", "project-1-isolate-ns")
			Expect(np.Exists()).To(BeTrue())
		})

		It("Sets NS for resource without it", func() {
			pod := f.KubernetesResource("Pod", "project-1", "project-1-test-resources")
			Expect(pod.Exists()).To(BeTrue())
		})

		It("Creates correct AuthorizationRules", func() {
			ar1 := f.KubernetesResource("AuthorizationRule", "project-1", "project-1-user-user-test-2-email-com")
			Expect(ar1.Exists()).To(BeTrue())
			Expect(ar1.Field("spec")).To(MatchJSON(`{"accessLevel":"User","subjects":[{"kind":"User","name":"test-2@email.com"}]}`))

			ar2 := f.KubernetesResource("AuthorizationRule", "project-1", "project-1-admin-service-account-test-3")
			Expect(ar2.Exists()).To(BeTrue())
			Expect(ar2.Field("spec")).To(MatchJSON(`{"accessLevel":"Admin","subjects":[{"kind":"ServiceAccount","namespace":"test-test","name":"test-3"}]}`))
		})

		It("Creates Project Namespace", func() {
			ar1 := f.KubernetesGlobalResource("Namespace", "project-1")
			Expect(ar1.Exists()).To(BeTrue())
		})

		It("Creates resources with long name with statically generated postfix for all same names", func() {
			roleName := retrieveObjectNameByKindFromRender("Role", render[userResourcesTemplate])
			Expect(roleName).NotTo(Equal(""))

			role := f.KubernetesResource("Role", "project-1", roleName)
			Expect(role.Exists()).To(BeTrue())

			pod := f.KubernetesResource("Pod", "project-1", roleName)
			Expect(pod.Exists()).To(BeTrue())
		})

		It("Creates resources with long name with different generated postfix for different long names", func() {
			replicasetName := retrieveObjectNameByKindFromRender("Replicaset", render[userResourcesTemplate])
			Expect(replicasetName).NotTo(Equal(""))
			Expect(replicasetName).To(HavePrefix("project-1"))

			replicaset := f.KubernetesResource("Replicaset", "project-1", replicasetName)
			Expect(replicaset.Exists()).To(BeTrue())

			roleName := retrieveObjectNameByKindFromRender("Role", render[userResourcesTemplate])
			Expect(roleName).NotTo(Equal(""))
			Expect(roleName).To(HavePrefix("project-1"))

			role := f.KubernetesResource("Role", "project-1", roleName)
			Expect(role.Exists()).To(BeTrue())

			lastSplitElement := func(s string) string { return s[strings.LastIndex(s, ",")+1:] }
			allButLastSplitElement := func(s string) string { return s[:strings.LastIndex(s, ",")+1] }

			Expect(lastSplitElement(roleName)).NotTo(Equal(lastSplitElement(replicasetName)))
			Expect(allButLastSplitElement(roleName)).To(Equal(allButLastSplitElement(replicasetName)))
		})

		It("Sets values to resources from values", func() {
			pod := f.KubernetesResource("Pod", "project-1", "project-1-test-resources")
			Expect(pod.Exists()).To(BeTrue())

			resources := pod.Field("spec.containers").Array()[0].Get("resources")
			Expect(resources.Get("requests")).To(MatchJSON(`{"cpu":1,"memory":"100Mi"}`))
			Expect(resources.Get("limits")).To(MatchJSON(`{"cpu":1,"memory":"100Mi"}`))
		})
	})
})

func retrieveObjectNameByKindFromRender(kind string, manifests string) string {
	manifestsList := releaseutil.SplitManifests(manifests)
	for _, manifest := range manifestsList {
		obj := ManifestStringToUnstructed(manifest)
		if obj.GetKind() == kind {
			return obj.GetName()
		}
	}
	return ""
}

const (
	stateOneProjectType = `
projectTypes:
  pt1:
    subjects:
    - kind: User
      name: test-2@email.com
      role: User
    - kind: ServiceAccount
      name: test-3
      namespace: test-test
      role: Admin
    openAPI:
      requests:
        type: object
        properties:
          cpu:
              oneOf:
                - type: number
                - type: string
              pattern: '^[0-9]+m?$'
          memory:
            oneOf:
              - type: number
              - type: string
            pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
    resourcesTemplate: |
      ---
      apiVersion: networking.k8s.io/v1
      kind: NetworkPolicy
      metadata:
        namespace: project-all
        name: isolate-ns
      ---
      apiVersion: v1
      kind: Pod
      metadata:
        name: test-resources
      spec:
        containers:
        - resources:
            requests:
              cpu: {{ .params.requests.cpu }}
              memory: {{ .params.requests.memory }}
            limits:
              {{ .params.requests | toYaml | nindent 8 }}
      ---
      apiVersion: rbac.authorization.k8s.io/v1
      kind: Role
      metadata:
        name: very-very-very-very-very-very-very-very-very-very-long-test-name
      ---
      apiVersion: v1
      kind: Pod
      metadata:
        name: very-very-very-very-very-very-very-very-very-very-long-test-name
      ---
      apiVersion: v1
      kind: Replicaset
      metadata:
        name: very-very-very-very-very-very-very-very-very-very-long-pod-test-name-1
`

	stateOneProject = `
projects:
  - projectName: project-1
    projectTypeName: pt1
    params:
      requests:
        cpu: 1
        memory: 100Mi
`

	stateEmptyProjectTypes = `
projectTypes: {}
`
)
