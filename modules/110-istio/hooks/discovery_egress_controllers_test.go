/*
Copyright 2026 Flant JSC

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

var _ = Describe("Istio hooks :: discovery_egress_controllers ::", func() {
	f := HookExecutionConfigInit(`{"istio":{ "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "EgressIstioController", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("istio.internal.egressControllers").String()).To(MatchJSON(`[]`))
		})
	})

	Context("Controllers with scheduling and resource settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: EgressIstioController
metadata:
  name: default
spec:
  egressGatewayClass: egress
  nodeSelector:
    node-role.kubernetes.io/system: ''
  tolerations:
    - effect: NoSchedule
      operator: Exists
  resourcesRequests:
    mode: VPA
    vpa:
      mode: Auto
      cpu:
        max: "500m"
        min: "100m"
      memory:
        max: "1Gi"
        min: "128Mi"
---
apiVersion: deckhouse.io/v1alpha1
kind: EgressIstioController
metadata:
  name: static
spec:
  egressGatewayClass: restricted
  resourcesRequests:
    mode: Static
    static:
      cpu: "200m"
      memory: "256Mi"
`))
			f.RunHook()
		})

		It("Should store egress controller CRs to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.egressControllers").String()).To(MatchJSON(`
[
  {
    "name": "default",
    "spec": {
      "egressGatewayClass": "egress",
      "nodeSelector": {
        "node-role.kubernetes.io/system": ""
      },
      "resourcesRequests": {
        "mode": "VPA",
        "vpa": {
          "cpu": {
            "max": "500m",
            "min": "100m"
          },
          "memory": {
            "max": "1Gi",
            "min": "128Mi"
          },
          "mode": "Auto"
        }
      },
      "tolerations": [
        {
          "effect": "NoSchedule",
          "operator": "Exists"
        }
      ]
    }
  },
  {
    "name": "static",
    "spec": {
      "egressGatewayClass": "restricted",
      "resourcesRequests": {
        "mode": "Static",
        "static": {
          "cpu": "200m",
          "memory": "256Mi"
        }
      }
    }
  }
]`))
		})
	})
})
