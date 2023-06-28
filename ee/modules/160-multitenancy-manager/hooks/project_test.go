/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Multitenancy Manager hooks :: handle Projects ::", func() {
	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{"projects":[]}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Project", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ProjectType", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Projects map must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("multitenancyManager.internal.projects").String()).To(MatchJSON(`[]`))
		})
	})

	Context("ProjectType with valid OpenAPI spec", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("multitenancyManager.internal.projectTypes.pt1", firstValidPT)
			f.ValuesSetFromYaml("multitenancyManager.internal.projectTypes.pt2", secondValidPT)
		})

		Context("Cluster with two valid Projects", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateTwoProjects))
				f.RunHook()
			})

			It("Projects must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("multitenancyManager.internal.projects").String()).To(MatchJSON(expectedTwoProjects))
			})
		})

		Context("Cluster with two valid and one invalid OpenAPI Project", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateTwoProjects + stateInvalidOpenAPIProject))
				f.RunHook()
			})

			It("Projects must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("multitenancyManager.internal.projects").String()).To(MatchJSON(expectedTwoProjects))
			})

			It("Valid Projects status without error", func() {
				pr1 := f.KubernetesGlobalResource("Project", "test-1")
				Expect(pr1.Exists()).To(BeTrue())

				Expect(pr1.Field("status.conditions")).To(MatchJSON(`[{"name":"Deploying","status":false}]`))
				Expect(pr1.Field("status.statusSummary")).To(MatchJSON(`{"status":false}`))

				pr2 := f.KubernetesGlobalResource("Project", "test-2")
				Expect(pr2.Exists()).To(BeTrue())

				Expect(pr2.Field("status.conditions")).To(MatchJSON(`[{"name":"Deploying","status":false}]`))
				Expect(pr2.Field("status.statusSummary")).To(MatchJSON(`{"status":false}`))
			})

			It("Invalid Project status with error", func() {
				pr3 := f.KubernetesGlobalResource("Project", "test-3")
				Expect(pr3.Exists()).To(BeTrue())

				// Expect(len(pr3.Field("status.conditions").Array())).To(Equal(1))
				Expect(pr3.Field("status.conditions")).To(MatchJSON(`[{"message":"template data doesn't match the OpenAPI schema for 'pt1' ProjectType: validation failure list:\n.memoryTest is a forbidden property","name":"Error","status":false}]`))
				Expect(pr3.Field("status.statusSummary")).To(MatchJSON(`{"message":"template data doesn't match the OpenAPI schema for 'pt1' ProjectType: validation failure list:\n.memoryTest is a forbidden property","status":false}`))
			})
		})
	})
})

const (
	stateTwoProjects = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-1
spec:
  description: abracadabra
  projectTypeName: pt1
  template:
    cpuRequests: 1
    memoryRequests: 200Gi
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-2
spec:
  description: test
  projectTypeName: pt2
  template:
    requests:
      cpu: 5
      memory: 5Gi
      storage: 1Gi
    limits:
      cpu: 5
      memory: 5Gi
`

	stateInvalidOpenAPIProject = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-3
spec:
  description: abracadabra
  projectTypeName: pt1
  template:
    cpuRequests: 1
    memoryTest: 200Gi
status:
  conditions:
    - name: Error
      message: "template data doesn't match the OpenAPI schema for 'pt1' ProjectType: validation failure list:\n.memoryTest is a forbidden property"
      status: false
    - name: Error
      message: "template data doesn't match the OpenAPI schema for 'pt1' ProjectType: validation failure list:\n.memoryTest is a forbidden property"
      status: false
`
	expectedTwoProjects = `
[
  {
    "params": {
      "cpuRequests": 1,
      "memoryRequests": "200Gi"
    },
    "projectTypeName": "pt1",
    "projectName": "test-1"
  },
  {
    "params": {
      "limits": {
        "cpu": 5,
        "memory": "5Gi"
      },
      "requests": {
        "cpu": 5,
        "memory": "5Gi",
        "storage": "1Gi"
      }
    },
    "projectTypeName": "pt2",
    "projectName": "test-2"
  }
]
`
)

var (
	firstValidPT = []byte(`
openAPI:
  cpuRequests:
    oneOf:
      - type: number
      - type: string
    pattern: "^[0-9]+m?$"
  memoryRequests:
    oneOf:
      - type: number
      - type: string
    pattern: "^[0-9]+(\\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$"
resourcesTemplate: |-
  ---
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: isolate-ns
  spec:
    podSelector:
      matchLabels:
        ingress:
        - from:
          - podSelector: {}
`)

	secondValidPT = []byte(`
namespaceMetadata:
  annotations:
    extended-monitoring.deckhouse.io/enabled: ""
  labels:
    created-from-project-type: test-project-type
openAPI:
  limits:
    properties:
      cpu:
        oneOf:
          - format: int
            type: number
          - type: string
        pattern: ^[0-9]+m?$
      memory:
        oneOf:
          - format: int
            type: number
          - type: string
        pattern: ^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$
    type: object
  requests:
    properties:
      cpu:
        oneOf:
          - format: int
            type: number
          - type: string
        pattern: ^[0-9]+m?$
      memory:
        oneOf:
          - format: int
            type: number
          - type: string
        pattern: ^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$
      storage:
        pattern: ^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$
        type: string
    type: object
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
  # Max requests and limits for resource consumption per pod in namespace.
  # All containers in a namespace must have requests and limits.
  # Refer to https://kubernetes.io/docs/concepts/policy/limit-range/
  apiVersion: v1
  kind: LimitRange
  metadata:
    name: all-containers
  spec:
    limits:
      - max:
          {{ with .params.limits.cpu }}cpu: {{ . }}{{ end }}
          {{ with .params.limits.memory }}limits.memory: {{ . }}{{ end }}
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
subjects:
  - kind: User
    name: multitenancy-admin
    role: Admin
  - kind: User
    name: multitenancy-user
    role: User
`)
)
