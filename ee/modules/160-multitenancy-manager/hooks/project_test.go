/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"bytes"
	"testing"

	"github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/releaseutil"
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
			f.BindingContexts.Set(f.KubeStateSet( /*legacy shema:*/ validProjectType + validProjectTypeProject + /*new shema:*/ validProjectTemplate + validProject + invalidProject))
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
		It("Should render secure project template with dedicated nodes", func() {
			validateProjectTemplate(dedicatedNodesTemplatePath, alternativeDedicatedNodesTemplatePath)
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
			status: `{"sync":false,"state":"Error","message": "template data doesn't match the OpenAPI schema for 'test-project-template' ProjectTemplate: validation failure list:\nresourceQuota.requests.cpu should match '^[0-9]+m?$'"}`,
		},
		// TODO add more cases
	}
)

// Legacy schema
// Project + ProjectType

const validProjectTypeProject = `
---
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: valid-project-type-project
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

// New schema
// Project + ProjectTemplate

const validProject = `
---
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: valid-project
spec:
  description: Valid project description
  projectTemplateName: test-project-template
  parameters:
    resourceQuota:
      requests:
        cpu: 5
        memory: 5Gi
        storage: 1Gi
      limits:
        cpu: 5
        memory: 5Gi
    administrators:
    - subject: User
      name: admin1
    - subject: User
      name: admin2
`

const invalidProject = `
---
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: invalid-project
spec:
  description: Invalid project description
  projectTemplateName: test-project-template
  parameters:
    resourceQuota:
      requests:
        cpu: wrong value
        memory: 5Gi
        storage: 1Gi
      limits:
        cpu: 5
        memory: 5Gi
    administrators:
    - subject: User
      name: admin1
    - subject: User
      name: admin2
`

const validProjectTemplate = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ProjectTemplate
metadata:
  name: test-project-template
spec:
  parametersSchema:
    openAPIV3Schema:
      type: object
      required:
        - administrators
        - resourceQuota
      properties:
        resourceQuota:
          type: object
          description: |
            Resource quota for the project.
            Refer to https://kubernetes.io/docs/concepts/policy/resource-quotas/
          properties:
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
        administrators:
          description: |
            Users and groups that will have admin access to the project.
            Administrators are eligible to manage roles and access to the project.
          type: array
          items:
            type: object
            required:
              - subject
              - name
            properties:
              subject:
                description: |
                  Kind of the target resource to apply access to the
                  environment ('Group' or 'User').
                enum:
                  - User
                  - Group
                type: string
              name:
                description: |
                  The name of the target resource to apply access
                  to the environment.
                minLength: 1
                type: string
        networkPolicy:
          description: |
            NotRestricted — Allow all traffic by default.
            Restricted — Deny all traffic by default except namespaced traffic, dns, prometheus metrics scraping and ingress-nginx.
          enum:
            - Isolated
            - NotRestricted
          type: string
          default: Isolated
        podSecurityProfile:
          description: |
            Pod security profile name.

            The Pod Security Standards define three different profiles to broadly cover the security spectrum. These profiles are cumulative and range from highly-permissive to highly-restrictive.
            - Privileged — Unrestricted policy. Provides the widest possible permission level;
            - Baseline — Minimally restrictive policy which prevents known privilege escalations. Allows for the default (minimally specified) Pod configuration;
            - Restricted — Heavily restricted policy. Follows the most current Pod hardening best practices.
          type: string
          default: Baseline
          enum:
            - Baseline
            - Restricted
            - Privileged
        extendedMonitoringEnabled:
          description: |
            Enable extended monitoring for the project.
            When enabled, the project will be monitored by the Deckhouse monitoring system and send the following alerts:
             - Controller outages and restarts
             - 5xx errors in ingress-nginx
             - Low free space on the persistent volumes
          type: boolean
          default: true
        clusterLogDestinationName:
          description: |
            If specified, the project will be monitored by the Deckhouse log shipper and send logs to the specified cluster log destination.
            The names of the custom resource must be specified in the 'clusterLogDestinationName' field.
          type: string
  resourcesTemplate: |
    ---
    apiVersion: v1
    kind: Namespace
    metadata:
      {{ with .projectName }}name: {{ . }}{{ end }}
      labels:
        {{ with .parameters.podSecurityProfile }}security.deckhouse.io/pod-policy: "{{ lower . }}"{{ end }}
        {{ if .parameters.extendedMonitoringEnabled }}extended-monitoring.deckhouse.io/enabled: ""{{ end }}
    {{- range $administrator := .parameters.administrators }}
    ---
    apiVersion: deckhouse.io/v1alpha1
    kind: AuthorizationRule
    metadata:
      name: {{ $administrator.name }}
    spec:
      accessLevel: Admin
      subjects:
      - kind: {{ $administrator.subject }}
        name: {{ $administrator.name }}
    {{- end }}
    ---
    # Max requests and limits for resource and storage consumption for all pods in a namespace.
    # Refer to https://kubernetes.io/docs/concepts/policy/resource-quotas/
    apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: all-pods
    spec:
      hard:
        {{ with .parameters.resourceQuota.requests.cpu }}requests.cpu: {{ . }}{{ end }}
        {{ with .parameters.resourceQuota.requests.memory }}requests.memory: {{ . }}{{ end }}
        {{ with .parameters.resourceQuota.requests.storage }}requests.storage: {{ . }}{{ end }}
        {{ with .parameters.resourceQuota.limits.cpu }}limits.cpu: {{ . }}{{ end }}
        {{ with .parameters.resourceQuota.limits.memory }}limits.memory: {{ . }}{{ end }}
    ---
`

func TestPostRenderer(t *testing.T) {
	pr := &projectTemplateHelmRenderer{logger: logrus.New()}
	pr.SetProject("test-project-1")
	buf := bytes.NewBuffer(nil)

	t.Run("without desired namespace", func(t *testing.T) {
		mfs := `
---
apiVersion: v1
kind: Namespace
metadata:
  name: test-project-1
  labels:
    heritage: multitenancy-manager
  annotations:
    multitenancy-boilerplate: "true"
---
# Source: test-project-1/user-resources-templates.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: test-project-1
  name: tututu
data: {}
`

		buf.Reset()
		buf.WriteString(mfs)

		result, err := pr.Run(buf)
		require.NoError(t, err)
		mm := releaseutil.SplitManifests(result.String())
		assert.Len(t, mm, 2)
		ns := mm["manifest-0"]
		assert.YAMLEq(t, `
apiVersion: v1
kind: Namespace
metadata:
  labels:
    heritage: multitenancy-manager
  annotations:
    multitenancy-boilerplate: "true"
  name: test-project-1
`, ns)
	})

	t.Run("with desired namespace", func(t *testing.T) {
		mfs := `
---
apiVersion: v1
kind: Namespace
metadata:
  name: test-project-1
  labels:
    heritage: multitenancy-manager
  annotations:
    multitenancy-boilerplate: "true"
---
# Source: test-project-1/user-resources-templates.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: test-project-1
  labels:
    twotwotwo: nanana
  annotations:
    foo: bar
---
# Source: test-project-1/user-resources-templates.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: test-project-1
  name: tututu
  labels:
    heritage: multitenancy-manager
data: {}
`

		buf.Reset()
		buf.WriteString(mfs)

		result, err := pr.Run(buf)
		require.NoError(t, err)
		mm := releaseutil.SplitManifests(result.String())
		assert.Len(t, mm, 2)
		ns := mm["manifest-0"]
		assert.YAMLEq(t, `
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    foo: bar
  labels:
    heritage: multitenancy-manager
    twotwotwo: nanana
  name: test-project-1
`, ns)
	})

	t.Run("with a few namespaces", func(t *testing.T) {
		mfs := `
---
apiVersion: v1
kind: Namespace
metadata:
  name: test-project-1
  labels:
    heritage: multitenancy-manager
  annotations:
    multitenancy-boilerplate: "true"
---
# Source: test-project-1/user-resources-templates.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: test-project-invalid
---
# Source: test-project-1/user-resources-templates.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: test-project-1
  labels:
    twotwotwo: lalala
---
# Source: test-project-1/user-resources-templates.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: test-project-1
  name: tututu
  labels:
    heritage: multitenancy-manager
data: {}
`

		buf.Reset()
		buf.WriteString(mfs)

		result, err := pr.Run(buf)
		require.NoError(t, err)
		mm := releaseutil.SplitManifests(result.String())
		assert.Len(t, mm, 2)
		ns := mm["manifest-0"]
		assert.YAMLEq(t, `
apiVersion: v1
kind: Namespace
metadata:
  labels:
    heritage: multitenancy-manager
    twotwotwo: lalala
  name: test-project-1
`, ns)
	})
}
