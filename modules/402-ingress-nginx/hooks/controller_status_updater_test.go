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

const (
	ingressNginxControllerManifest = `
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  generation: 20
  name: main
spec:
  annotationValidationEnabled: false
  chaosMonkey: false
  disableHTTP2: false
  enableHTTP3: false
  hostPort:
    httpPort: 80
    httpsPort: 443
    realIPHeader: X-Forwarded-For
  hsts: false
  ingressClass: nginx
  inlet: HostPort
  maxReplicas: 1
  minReplicas: 1
  nodeSelector:
    node-role.kubernetes.io/master: ""
  tolerations:
  - effect: NoSchedule
    operator: Exists
  underscoresInHeaders: false
  validationEnabled: true
status:
  observedGeneration: 19
`
)

var _ = Describe("Modules :: ingress-nginx :: hooks :: ingress_controller_status_updater ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.10", "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)
	f.RegisterCRD("apps.kruise.io", "v1alpha1", "DaemonSet", true)

	Context("DaemonSet is ready", func() {
		BeforeEach(func() {
			f.ValuesSet("ingressNginx.internal.controllerState.main.generation", "20")
			f.ValuesSet("ingressNginx.internal.controllerState.main.observedGeneration", "19")
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/controller-version: "1.12"
  labels:
    app: controller
    name: main
  name: controller-main
  namespace: d8-ingress-nginx
status:
  currentNumberScheduled: 1
  desiredNumberScheduled: 1
  labelSelector: app=controller,name=main
  numberAvailable: 1
  numberMisscheduled: 0
  numberReady: 1
  observedGeneration: 2
  updatedNumberScheduled: 1
` + ingressNginxControllerManifest))
			f.RunHook()
		})

		It("Should set version to 1.12 and Ready condition to True", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesGlobalResource("IngressNginxController", "main")
			Expect(ingress).ToNot(BeNil())
			Expect(ingress.Field("status.version").String()).To(Equal("1.12"))
			Expect(ingress.Field("status.observedGeneration").String()).To(Equal("20"))
			conditions := ingress.Field("status.conditions").Array()
			Expect(conditions[0].Get("status").String()).To(Equal("True"))
			Expect(conditions[0].Get("reason").String()).To(Equal("AllPodsReady"))
			Expect(f.ValuesGet("ingressNginx.internal.appliedControllerVersion.main").String()).To(Equal("1.12"))
		})
	})

	Context("DaemonSet is not ready without applied version", func() {
		BeforeEach(func() {
			f.ValuesSet("ingressNginx.internal.controllerState.main.generation", "20")
			f.ValuesSet("ingressNginx.internal.controllerState.main.observedGeneration", "19")
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/controller-version: "1.12"
  labels:
    app: controller
    name: main
  name: controller-main
  namespace: d8-ingress-nginx
status:
  currentNumberScheduled: 1
  desiredNumberScheduled: 1
  labelSelector: app=controller,name=main
  numberAvailable: 1
  numberMisscheduled: 0
  numberReady: 0
  observedGeneration: 2
  updatedNumberScheduled: 1
` + ingressNginxControllerManifest))

			f.RunHook()
		})

		It("Should set version to unknown and Ready condition to False", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesGlobalResource("IngressNginxController", "main")
			Expect(ingress).ToNot(BeNil())
			Expect(ingress.Field("status.version").String()).To(Equal("unknown"))
			Expect(ingress.Field("status.observedGeneration").String()).To(Equal("19"))
			conditions := ingress.Field("status.conditions").Array()
			Expect(conditions[0].Get("status").String()).To(Equal("False"))
			Expect(conditions[0].Get("reason").String()).To(Equal("PodsNotReady"))
			Expect(f.ValuesGet("ingressNginx.internal.appliedControllerVersion.main").String()).To(Equal(""))
		})
	})

	Context("DaemonSet is not ready with applied version in Values", func() {
		BeforeEach(func() {
			f.ValuesSet("ingressNginx.internal.appliedControllerVersion.main", "1.10")
			f.ValuesSet("ingressNginx.internal.controllerState.main.generation", "20")
			f.ValuesSet("ingressNginx.internal.controllerState.main.observedGeneration", "19")
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: apps.kruise.io/v1alpha1
kind: DaemonSet
metadata:
  annotations:
    ingress-nginx-controller.deckhouse.io/controller-version: "1.12"
  labels:
    app: controller
    name: main
  name: controller-main
  namespace: d8-ingress-nginx
status:
  currentNumberScheduled: 1
  desiredNumberScheduled: 1
  labelSelector: app=controller,name=main
  numberAvailable: 1
  numberMisscheduled: 0
  numberReady: 0
  observedGeneration: 2
  updatedNumberScheduled: 1
` + ingressNginxControllerManifest))
			f.RunHook()
		})

		It("Should set version to 1.10 from Values and Ready condition to False", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesGlobalResource("IngressNginxController", "main")
			Expect(ingress).ToNot(BeNil())
			Expect(ingress.Field("status.version").String()).To(Equal("1.10"))
			Expect(ingress.Field("status.observedGeneration").String()).To(Equal("19"))
			conditions := ingress.Field("status.conditions").Array()
			Expect(conditions[0].Get("status").String()).To(Equal("False"))
			Expect(conditions[0].Get("reason").String()).To(Equal("PodsNotReady"))
			Expect(f.ValuesGet("ingressNginx.internal.appliedControllerVersion.main").String()).To(Equal("1.10"))
		})
	})
})
