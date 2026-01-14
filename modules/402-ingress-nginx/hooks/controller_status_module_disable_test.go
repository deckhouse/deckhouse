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

const (
	ingressNginxControllerForModuleDisable = `
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

	Context("ModuleConfig is disabled", func() {
		BeforeEach(func() {
			f.ValuesSet("ingressNginx.internal.controllerState.main.generation", "20")
			f.ValuesSet("ingressNginx.internal.controllerState.main.observedGeneration", "19")
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			f.BindingContexts.Set(f.KubeStateSet(ingressNginxControllerForModuleDisable))
			f.RunHook()
		})

		It("Should set version to 'unknown' and Ready condition to False", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesGlobalResource("IngressNginxController", "main")
			Expect(ingress).ToNot(BeNil())
			Expect(ingress.Field("status.version").String()).To(Equal("unknown"))
			Expect(ingress.Field("status.observedGeneration").String()).To(Equal("19"))
			conditions := ingress.Field("status.conditions").Array()
			Expect(conditions[0].Get("status").String()).To(Equal("False"))
			Expect(conditions[0].Get("reason").String()).To(Equal("ModuleDisabled"))
		})
	})
})
