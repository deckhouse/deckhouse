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

var _ = Describe("Multitenancy Manager hooks :: handle ProjectTypes ::", func() {
	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{"projectTypes":{}}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ProjectType", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("ProjectTypes map must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("multitenancyManager.internal.projectTypes").String()).To(MatchJSON(`{}`))
		})
	})

	Context("Cluster with two ProjectTypes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateTwoProjectTypes))
			f.RunHook()
		})

		It("ProjectTypes must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("multitenancyManager.internal.projectTypes").String()).To(MatchJSON(expectedTwoProjectTypes))
		})

		It("ProjectTypes status without error", func() {
			pt1 := f.KubernetesGlobalResource("ProjectType", "pt1")
			Expect(pt1.Exists()).To(BeTrue())

			Expect(pt1.Field("status.statusSummary")).To(MatchJSON(`{"status":true}`))

			pt2 := f.KubernetesGlobalResource("ProjectType", "pt2")
			Expect(pt2.Exists()).To(BeTrue())

			Expect(pt2.Field("status.statusSummary")).To(MatchJSON(`{"status":true}`))
		})
	})

	Context("Cluster with bad open api ProjectType", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateBadOpenAPIProjectType))
			f.RunHook()
		})

		It("ProjectType doesn't stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("multitenancyManager.internal.projectTypes").String()).To(MatchJSON(`{}`))
		})

		It("ProjectType status with error", func() {
			pt3 := f.KubernetesGlobalResource("ProjectType", "pt3")
			Expect(pt3.Exists()).To(BeTrue())

			Expect(pt3.Field("status.statusSummary")).To(MatchJSON(`{"status":false,"message": "can't load open api schema from 'pt3' ProjectType spec: unmarshal spec.openAPI to spec.Schema: json: cannot unmarshal array into Go struct field .properties of type struct { spec.SchemaProps; spec.SwaggerSchemaProps }"}`))
		})
	})
})

const (
	stateTwoProjectTypes = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ProjectType
metadata:
  name: pt1
spec:
  subjects:
    - kind: User
      name: test-1
      role: Admin
  namespaceMetadata:
    labels:
      security.deckhouse.io/pod-policy: ”Baseline”
    annotations: {}
  openAPI:
    cpuRequests:
      oneOf:
        - type: number
        - type: string
      pattern: '^[0-9]+m?$'
    memoryRequests:
      oneOf:
        - type: number
        - type: string
      pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
  resourcesTemplate: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: ProjectType
metadata:
  name: pt2
spec:
  subjects:
    - kind: User
      name: test-2
      role: User
    - kind: User
      name: test-3
      role: User
  openAPI:
    cpuRequests:
      oneOf:
        - type: number
        - type: string
      pattern: '^[0-9]+m?$'
  resourcesTemplate: |
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
`
	stateBadOpenAPIProjectType = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ProjectType
metadata:
  name: pt3
spec:
  subjects:
    - kind: User
      name: test-4
      role: Editor
    - kind: User
      name: test-5
      role: Editor
  openAPI:
      req:
      - type: number
      - type: string
  resourcesTemplate: ""
`

	expectedTwoProjectTypes = `
{
    "pt1": {
        "subjects": [
            {
                "kind": "User",
                "name": "test-1",
                "role": "Admin"
            }
        ],
        "namespaceMetadata": {
            "labels": {
                "security.deckhouse.io/pod-policy": "”Baseline”"
            }
        },
        "openAPI": {
            "cpuRequests": {
                "oneOf": [
                    {
                        "type": "number"
                    },
                    {
                        "type": "string"
                    }
                ],
                "pattern": "^[0-9]+m?$"
            },
            "memoryRequests": {
                "oneOf": [
                    {
                        "type": "number"
                    },
                    {
                        "type": "string"
                    }
                ],
                "pattern": "^[0-9]+(\\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$"
            }
        }
    },
    "pt2": {
        "subjects": [
            {
                "kind": "User",
                "name": "test-2",
                "role": "User"
            },
            {
                "kind": "User",
                "name": "test-3",
                "role": "User"
            }
        ],
        "namespaceMetadata": {},
        "openAPI": {
            "cpuRequests": {
                "oneOf": [
                    {
                        "type": "number"
                    },
                    {
                        "type": "string"
                    }
                ],
                "pattern": "^[0-9]+m?$"
            }
        },
        "resourcesTemplate": "---\napiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: isolate-ns\nspec:\n  podSelector:\n    matchLabels:\n      ingress:\n      - from:\n        - podSelector: {}"
    }
}
`
)
