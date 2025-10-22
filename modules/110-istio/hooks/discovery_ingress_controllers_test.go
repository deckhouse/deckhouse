/*
Copyright 2023 Flant JSC

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

var _ = Describe("Istio hooks :: discovery_ingress_controllers :: ::", func() {
	f := HookExecutionConfigInit(`{"istio":{ "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IngressIstioController", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("Controllers with some inlet types", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressIstioController
metadata:
  name: lb-test
spec:
  ingressGatewayClass: lb
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      aaa: bbb
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressIstioController
metadata:
  name: hp-test
spec:
  ingressGatewayClass: hp
  inlet: HostPort
  hostPort:
    httpPort: 8080
    httpsPort: 8443
`))
			f.RunHook()
		})

		It("Should store ingress controller crds to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("istio.internal.ingressControllers").String()).To(MatchJSON(`
[
  {
	"name": "hp-test",
	"spec": {
	  "hostPort": {
		"httpPort": 8080,
		"httpsPort": 8443
	  },
	  "ingressGatewayClass": "hp",
	  "inlet": "HostPort"
	}
  },
  {
	"name": "lb-test",
	"spec": {
	  "ingressGatewayClass": "lb",
	  "inlet": "LoadBalancer",
	  "loadBalancer": {
		"annotations": {
		  "aaa": "bbb"
		}
	  }
	}
  }
]`))
		})
	})

	Context("Controller with a bunch of parameters", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressIstioController
metadata:
  name: np-test
spec:
  ingressGatewayClass: np
  inlet: NodePort
  nodeSelector:
    node-role.kubernetes.io/master: ''
  nodePort:
    httpPort: 30080
    httpsPort: 30443
  tolerations:
    - effect: NoSchedule
      operator: Exists
  resourcesRequests:
    mode: VPA
    vpa:
      mode: Initial
      cpu:
        max: "100m"
        min: "50m"
      memory:
        max: "100Mi"
        min: "50Mi"
`))
			f.RunHook()
		})

		It("Should store ingress controller crds to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("istio.internal.ingressControllers").String()).To(MatchJSON(`
[
  {
	"name": "np-test",
	"spec": {
	  "ingressGatewayClass": "np",
	  "inlet": "NodePort",
	  "nodePort": {
		"httpPort": 30080,
		"httpsPort": 30443
	  },
	  "nodeSelector": {
		"node-role.kubernetes.io/master": ""
	  },
	  "resourcesRequests": {
		"mode": "VPA",
		"vpa": {
		  "cpu": {
			"max": "100m",
			"min": "50m"
		  },
		  "memory": {
			"max": "100Mi",
			"min": "50Mi"
		  },
		  "mode": "Initial"
		}
	  },
	  "tolerations": [
		{
		  "effect": "NoSchedule",
		  "operator": "Exists"
		}
	  ]
	}
  }
]`))
		})
	})

})
