/*
Copyright 2025 Flant JSC

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

const finalizer = "finalizer.ingress-nginx.deckhouse.io"

var ingressControllerNoFinalizer = `
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec: {}
`

var ingressControllerWithFinalizer = `
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main-with
  finalizers:
  - finalizer.ingress-nginx.deckhouse.io
spec: {}
`

var ingressControllerWithFakeFinalizers = `
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
  finalizers:
  - some.finalizer.io
  - do.not.touch.this
  - finalizer.ingress-nginx.deckhouse.io
spec: {}
`

var loadBalancerServiceController = `
---
apiVersion: v1
kind: Service
metadata:
  name: main-load-balancer
  namespace: d8-ingress-nginx
spec:
  selector:
    app: controller
  ports:
    - name: http
      port: 80
      targetPort: 80
`

var withFailoverServiceController = `
---
apiVersion: v1
kind: Service
metadata:
  name: controller-main-failover
  namespace: d8-ingress-nginx
spec:
  selector:
    app: controller
  ports:
    - name: http
      port: 80
      targetPort: 80
`

var admissionService = `
---
apiVersion: v1
kind: Service
metadata:
  name: main-admission
  namespace: d8-ingress-nginx
spec:
  selector:
    app: controller
  ports:
    - name: https
      port: 443
      targetPort: 443
`

var controllerDaemonSet = `
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  name: controller-main
  namespace: d8-ingress-nginx
  labels:
    app: controller
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 3
`

var controllerDaemonSetWithFailover = `
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  name: proxy-main-failover
  namespace: d8-ingress-nginx
  labels:
    app: controller
spec: {}
status:
  desiredNumberScheduled: 3
  numberAvailable: 3
  updatedNumberScheduled: 3
`

var _ = Describe("Modules :: ingress-nginx :: hooks :: handle_finalizers", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.10", "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)
	f.RegisterCRD("apps.kruise.io", "v1alpha1", "DaemonSet", true)

	Context("Given an IngressNginxController with existing child resources, a finalizer must be added.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressControllerNoFinalizer + loadBalancerServiceController + controllerDaemonSet + admissionService))
			f.RunGoHook()
		})

		It("should add finalizer 'true'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("IngressNginxController", "", "main").Field("metadata.finalizers").AsStringSlice()).Should(ContainElement(finalizer))
		})
	})

	Context("Given an IngressNginxController with no child resources, a finalizer must not be added.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressControllerNoFinalizer))
			f.RunGoHook()
		})

		It("should add finalizer 'false'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("IngressNginxController", "", "main").Field("metadata.finalizers").AsStringSlice()).ShouldNot(ContainElement(finalizer))
		})
	})

	Context("Given an IngressNginxController with no child resources, its finalizer must be removed.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressControllerWithFinalizer))
			f.RunGoHook()
		})

		It("should remove finalizer", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("IngressNginxController", "", "main").Field("metadata.finalizers").AsStringSlice()).ShouldNot(ContainElement(finalizer))
		})
	})

	Context("Given an 'IngressNginxController` resource does not contain child resources and has fake finalizers, the finalizer `finalizer.ingress-nginx.deckhouse.io ` must be deleted, other finalizers must remain.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressControllerWithFakeFinalizers))
			f.RunGoHook()
		})

		It("should remove only our finalizer 'true'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("IngressNginxController", "", "main").Field("metadata.finalizers").AsStringSlice()).Should(ContainElement("do.not.touch.this"))
			Expect(f.KubernetesResource("IngressNginxController", "", "main").Field("metadata.finalizers").AsStringSlice()).ShouldNot(ContainElement(finalizer))
		})
	})

	Context("If an IngressNginxController resource has the inlet type HostWithFailover and any child resources exist, then a finalizer should be added to the controller.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressControllerNoFinalizer + withFailoverServiceController + controllerDaemonSetWithFailover))
			f.RunGoHook()
		})

		It("should add our finalizer 'true'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("IngressNginxController", "", "main").Field("metadata.finalizers").AsStringSlice()).Should(ContainElement(finalizer))
		})
	})

})
