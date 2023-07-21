/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks_test

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Multitenancy Manager hooks :: handle Projects ::", func() {
	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{"projects":[]}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Project", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ProjectType", false)

	secretCreatedWithProjectValues := func(secretProjectValues string) {
		secret := f.KubernetesResource("Secret", "d8-system", "deckhouse-multitenancy-manager")
		Expect(secret.Exists()).To(BeTrue())

		decoded, err := base64.StdEncoding.DecodeString(secret.Field("data.projectValues").String())
		Expect(err).ToNot(HaveOccurred())
		Expect(decoded).To(MatchJSON(secretProjectValues))
	}

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Projects map must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("multitenancyManager.internal.projects").String()).To(MatchJSON(`[]`))
		})

		It("Must create secret", func() {
			secretCreatedWithProjectValues(`{}`)
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
				Expect(f.ValuesGet("multitenancyManager.internal.projects").String()).To(MatchJSON("[" + expectedProject1 + "," + expectedProject2 + "]"))
			})

			It("Must create secret", func() {
				secretCreatedWithProjectValues(`{"test-1":` + expectedProject1 + `,"test-2":` + expectedProject2 + "}")
			})
		})

		Context("Cluster with two valid and two invalid OpenAPI Project", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateTwoProjects + stateInvalidOpenAPIProjects))
				f.RunHook()
			})

			It("Projects must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("multitenancyManager.internal.projects").String()).To(MatchJSON("[" + expectedProject1 + "," + expectedProject2 + "]"))
			})

			It("Projects with valid status", func() {
				for _, tc := range testCasesForProjectStatuses {
					checkProjectStatus(f, tc)
				}
			})

			It("Must create secret", func() {
				secretCreatedWithProjectValues(`{"test-1":` + expectedProject1 + `,"test-2":` + expectedProject2 + "}")
			})
		})

		Context("Cluster with two valid and two invalid OpenAPI Project with old values from secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateTwoProjects + stateInvalidOpenAPIProjects + stateOldValuesProjectSecret))
				f.RunHook()
			})

			It("Projects must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("multitenancyManager.internal.projects").String()).To(MatchJSON("[" + expectedProject1 + "," + expectedProject2 + "," + expectedProject3 + "]"))
			})

			It("Projects with valid status", func() {
				for _, tc := range testCasesForProjectStatuses {
					checkProjectStatus(f, tc)
				}
			})

			It("Must update project values secret data", func() {
				secretCreatedWithProjectValues(`{"test-1":` + expectedProject1 + `,"test-2":` + expectedProject2 + `,"test-3":` + expectedProject3 + "}")
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

	stateInvalidOpenAPIProjects = `
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
  message: "template data doesn't match the OpenAPI schema for 'pt1' ProjectType: validation failure list:\n.memoryTest is a forbidden property"
  sync: false
  state: Error
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-4
spec:
  description: abracadabra
  projectTypeName: pt1
  template:
    cpuRequests: 1
    memoryTest: 200Gi
`
	expectedProject1 = `{
  "params": {
    "cpuRequests": 1,
    "memoryRequests": "200Gi"
  },
  "projectTypeName": "pt1",
  "projectName": "test-1"
}`

	expectedProject2 = `{
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
}`

	expectedProject3 = `{
  "params": {
    "limits": {
      "cpu": 5
    },
    "requests": {
      "cpu": 5
    }
  },
  "projectTypeName": "pt3",
  "projectName": "test-3"
}`
)

var (
	stateOldValuesProjectSecret = fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-multitenancy-manager
  namespace: d8-system
data:
  projectValues: %s
`, base64.StdEncoding.EncodeToString([]byte(
		`{"test-1":{"projectName":"test-1"},"test-2":{"projectName":"test-2"},"test-3":{"projectName":"test-3","params":{"limits":{"cpu":5},"requests":{"cpu":5}},"projectTypeName":"pt3"}}`,
	)))

	testCasesForProjectStatuses = []testProjectStatus{
		{
			name:   "test-1",
			exists: true,
			status: `{"sync":false,"state":"Deploying","message":"Deckhouse is creating the project, see deckhouse logs for more details."}`,
		},
		{
			name:   "test-2",
			exists: true,
			status: `{"sync":false,"state":"Deploying","message":"Deckhouse is creating the project, see deckhouse logs for more details."}`,
		},
		{
			name:   "test-3",
			exists: true,
			status: `{"message":"template data doesn't match the OpenAPI schema for 'pt1' ProjectType: validation failure list:\n.memoryTest is a forbidden property","state":"Error","sync":false}`,
		},
		{
			name:   "test-4",
			exists: true,
			status: `{"message":"template data doesn't match the OpenAPI schema for 'pt1' ProjectType: validation failure list:\n.memoryTest is a forbidden property","state":"Error","sync":false}`,
		},
	}

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
