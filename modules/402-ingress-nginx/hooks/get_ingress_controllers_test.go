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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: get_ingress_controllers ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.10", "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("After adding ingress nginx controller object and webhook certificate", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: test
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  controllerVersion: "1.10"
  acceptRequestsFrom:
  - 127.0.0.1/32
  - 192.168.0.0/24
`))
				f.RunHook()
			})

			It("Should store ingress controller crds to values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("ingressNginx.internal.ingressControllers").String()).To(MatchJSON(`[{
"name": "test",
"spec": {
  "acceptRequestsFrom": [
    "127.0.0.1/32",
    "192.168.0.0/24"
  ],
  "annotationValidationEnabled": false,
  "chaosMonkey": false,
  "config": {},
  "controllerVersion": "1.10",
  "disableHTTP2": false,
  "enableHTTP3": false,
  "geoIP2": {},
  "hostPort": {},
  "hostPortWithProxyProtocol": {},
  "hostWithFailover": {},
  "hsts": false,
  "hstsOptions": {},
  "ingressClass": "nginx",
  "inlet": "LoadBalancer",
  "loadBalancer": {},
  "controllerLogLevel": "Info",
  "loadBalancerWithProxyProtocol": {},
  "maxReplicas": 1,
  "minReplicas": 1,
  "resourcesRequests": {
    "mode": "VPA",
    "static": {},
    "vpa": {
      "cpu": {},
      "memory": {}
    }
  },
  "underscoresInHeaders": false,
  "validationEnabled": true
}
}]`))
			})
		})
	})

	Context("IngressNginxContorller without explicitly controllerVersion", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: testd
spec:
  ingressClass: nginx
  inlet: LoadBalancer
`))
			f.RunHook()
		})

		It("Shouldn't fill controller version with default value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers.0.spec.controllerVersion").Exists()).To(BeFalse())
		})
	})

	Context("With Ingress Nginx Controller resource", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: test
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  resourcesRequests:
    mode: Static
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: test-2
spec:
  ingressClass: test
  inlet: HostPortWithProxyProtocol
  resourcesRequests:
    mode: VPA
    vpa:
      mode: Auto
      cpu:
        max: 100m
      memory:
        max: 200Mi
  hostPortWithProxyProtocol:
    httpPort: 80
    httpsPort: 443
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: test-3
spec:
  ingressClass: test
  inlet: LoadBalancerWithProxyProtocol
`))
			f.RunHook()
		})
		It("Should store ingress controller crds to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers").Array()).To(HaveLen(3))

			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers.0.name").String()).To(Equal("test"))
			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers.0.spec").String()).To(MatchJSON(`{
"annotationValidationEnabled": false,
"chaosMonkey": false,
"config": {},
"disableHTTP2": false,
"enableHTTP3": false,
"geoIP2": {},
"hostPort": {},
"hostPortWithProxyProtocol": {},
"hostWithFailover": {},
"hsts": false,
"hstsOptions": {},
"ingressClass": "nginx",
"inlet": "LoadBalancer",
"loadBalancer": {},
"loadBalancerWithProxyProtocol": {},
"maxReplicas": 1,
"minReplicas": 1,
"controllerLogLevel": "Info",
"resourcesRequests": {
  "mode": "Static",
  "static": {},
  "vpa": {
    "cpu": {},
    "memory": {}
  }
},
"underscoresInHeaders": false,
"validationEnabled": true
}`))

			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers.1.name").String()).To(Equal("test-2"))
			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers.1.spec").String()).To(MatchJSON(`{
"annotationValidationEnabled": false,
"chaosMonkey": false,
"config": {},
"disableHTTP2": false,
"enableHTTP3": false,
"geoIP2": {},
"hostPort": {},
"hostPortWithProxyProtocol": {
  "httpPort": 80,
  "httpsPort": 443
},
"hostWithFailover": {},
"hsts": false,
"hstsOptions": {},
"ingressClass": "test",
"inlet": "HostPortWithProxyProtocol",
"loadBalancer": {},
"loadBalancerWithProxyProtocol": {},
"maxReplicas": 1,
"minReplicas": 1,
"resourcesRequests": {
  "mode": "VPA",
  "static": {},
  "vpa": {
    "cpu": {
      "max": "100m",
      "min": "10m"
    },
    "memory": {
      "max": "200Mi",
      "min": "50Mi"
    },
    "mode": "Auto"
  }
},
"underscoresInHeaders": false,
"validationEnabled": true,
"controllerLogLevel": "Info"
}`))

			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers.2.name").String()).To(Equal("test-3"))
			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers.2.spec").String()).To(MatchJSON(`{
"annotationValidationEnabled": false,
"chaosMonkey": false,
"config": {},
"disableHTTP2": false,
"enableHTTP3": false,
"geoIP2": {},
"hostPort": {},
"hostPortWithProxyProtocol": {},
"hostWithFailover": {},
"hsts": false,
"hstsOptions": {},
"ingressClass": "test",
"inlet": "LoadBalancerWithProxyProtocol",
"loadBalancer": {},
"loadBalancerWithProxyProtocol": {},
"maxReplicas": 1,
"minReplicas": 1,
"resourcesRequests": {
  "mode": "VPA",
  "static": {},
  "vpa": {
    "cpu": {},
    "memory": {}
  }
},
"underscoresInHeaders": false,
"validationEnabled": true,
"controllerLogLevel": "Info"
}`))
		})
	})

	var IngressNginxControllerWithDeletionTomeStamp = `
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: test-3
  deletionTimestamp: "2025-05-26T08:35:00Z"
  finalizers:
  - finalizer.ingress-nginx.deckhouse.io
spec:
  ingressClass: test
  inlet: LoadBalancerWithProxyProtocol
`

	Context("A controller with deletion timestamp", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(IngressNginxControllerWithDeletionTomeStamp))
			f.RunGoHook()
		})

		It("controller has to be excluded from internal.ingressControllers", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers").Array()).Should(BeEmpty())
		})
	})

	Context("With suspended validation annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: test-suspended
  annotations:
    network.deckhouse.io/ingress-nginx-validation-suspended: ""
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  validationEnabled: true
`))
			f.RunHook()
		})
		It("Should disable validationEnabled when annotation is present", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers.0.name").String()).To(Equal("test-suspended"))

			name := f.ValuesGet("ingressNginx.internal.ingressControllers.0.name").String()
			validationEnabled := f.ValuesGet("ingressNginx.internal.ingressControllers.0.spec.validationEnabled").Bool()

			fmt.Println(name, validationEnabled)

			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers.0.spec.validationEnabled").Bool()).To(BeFalse())
		})
	})
})
