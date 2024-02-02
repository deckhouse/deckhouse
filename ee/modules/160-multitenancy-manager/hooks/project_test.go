/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/helm"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Multitenancy Manager hooks :: handle Projects ::", func() {
	f := HookExecutionConfigInit(`{"multitenancyManager":{}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha2", "Project", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ProjectType", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ProjectTemplate", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with valid and invalid Projects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(validProjectType + validProject + invalidProject))
			dependency.TestDC.HelmClient = helm.NewClientMock(GinkgoT())
			dependency.TestDC.HelmClient.UpgradeMock.Return(nil)
			f.RunHook()
		})

		It("Execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Projects with valid status", func() {
			for _, tc := range testCasesForProjectStatuses {
				checkProjectStatus(f, tc)
			}
		})
	})

	Context("Test default project template", func() {
		It("Should render default project template", func() {
			validateProjectTemplate(defaultProjectTemplatePath, alternativeDefaultProjectTemplatePath)
		})
		It("Should render secure project template", func() {
			validateProjectTemplate(secureProjectTemplatePath, alternativeSecureProjectTemplatePath)
		})
	})
})

func validateProjectTemplate(defaultProjectTemplatePath, alternativeDefaultProjectTemplatePath string) {
	defaultProjectTemplateRaw, err := readDefaultProjectTemplate(defaultProjectTemplatePath, alternativeDefaultProjectTemplatePath)
	Expect(err).ToNot(HaveOccurred())

	projectTemplate := &v1alpha1.ProjectTemplate{}
	obj := unstructured.Unstructured{Object: make(map[string]interface{})}

	err = yaml.Unmarshal(defaultProjectTemplateRaw, &obj.Object)
	Expect(err).ToNot(HaveOccurred())

	err = sdk.FromUnstructured(&obj, projectTemplate)
	Expect(err).ToNot(HaveOccurred())

	projectTemplateSnapshot := internal.ProjectTemplateSnapshot{
		Name: "default",
		Spec: projectTemplate.Spec,
	}

	err = internal.ValidateProjectTemplate(projectTemplateSnapshot)
	Expect(err).ToNot(HaveOccurred())
}

var (
	testCasesForProjectStatuses = []testProjectStatus{
		{
			name:   "valid-project",
			exists: true,
			status: `{"sync":true,"state":"Sync"}`,
		},
		{
			name:   "invalid-project",
			exists: true,
			status: `{"sync":false,"state":"Error","message": "template data doesn't match the OpenAPI schema for 'test-project-type' ProjectTemplate: validation failure list:\nrequests.cpu should match '^[0-9]+m?$'"}`,
		},
		// TODO add more cases
	}
)

const validProject = `
---
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: valid-project
spec:
  description: Valid project description
  projectTemplateName: test-project-type
  parameters:
    requests:
      cpu: 5
      memory: 5Gi
      storage: 1Gi
    limits:
      cpu: 5
      memory: 5Gi
`

const invalidProject = `
---
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: invalid-project
spec:
  description: Invalid project description
  projectTemplateName: test-project-type
  parameters:
    requests:
      cpu: wrong value
      memory: 5Gi
      storage: 1Gi
    limits:
      cpu: 5
      memory: 5Gi
`

const validProjectType = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ProjectType
metadata:
  name: test-project-type
spec:
  subjects:
    - kind: User
      name: admin@cluster
      role: Admin
    - kind: User
      name: user@cluster
      role: User
  namespaceMetadata:
    annotations:
      extended-monitoring.deckhouse.io/enabled: ""
    labels:
      created-from-project-type: test-project-type
  openAPI:
    requests:
      type: object
      properties:
        cpu:
          oneOf:
            - type: number
              format: int
            - type: string
          pattern: "^[0-9]+m?$"
        memory:
          oneOf:
            - type: number
              format: int
            - type: string
          pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
        storage:
          type: string
          pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
    limits:
      type: object
      properties:
        cpu:
          oneOf:
            - type: number
              format: int
            - type: string
          pattern: "^[0-9]+m?$"
        memory:
          oneOf:
            - type: number
              format: int
            - type: string
          pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
  resourcesTemplate: |
    ---
    # Max requests and limits for resource and storage consumption for all pods in a namespace.
    # Refer to https://kubernetes.io/docs/concepts/policy/resource-quotas/
    apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: all-pods
    spec:
      hard:
        {{ with .params.requests.cpu }}requests.cpu: {{ . }}{{ end }}
        {{ with .params.requests.memory }}requests.memory: {{ . }}{{ end }}
        {{ with .params.requests.storage }}requests.storage: {{ . }}{{ end }}
        {{ with .params.limits.cpu }}limits.cpu: {{ . }}{{ end }}
        {{ with .params.limits.memory }}limits.memory: {{ . }}{{ end }}
    ---
    # Max requests and limits for resource consumption per pod in a namespace.
    # All the containers in a namespace must have requests and limits specified.
    # Refer to https://kubernetes.io/docs/concepts/policy/limit-range/
    apiVersion: v1
    kind: LimitRange
    metadata:
      name: all-containers
    spec:
      limits:
        - max:
            {{ with .params.limits.cpu }}cpu: {{ . }}{{ end }}
            {{ with .params.limits.memory }}memory: {{ . }}{{ end }}
          maxLimitRequestRatio:
            cpu: 1
            memory: 1
          type: Container
    ---
    # Deny all network traffic by default except namespaced traffic and dns.
    # Refer to https://kubernetes.io/docs/concepts/services-networking/network-policies/
    kind: NetworkPolicy
    apiVersion: networking.k8s.io/v1
    metadata:
      name: deny-all-except-current-namespace
    spec:
      podSelector:
        matchLabels: {}
      policyTypes:
        - Ingress
        - Egress
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: "{{ .projectName }}"
      egress:
        - to:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: "{{ .projectName }}"
        - to:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: kube-system
          ports:
            - protocol: UDP
              port: 53
`
