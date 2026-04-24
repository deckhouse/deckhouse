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

var _ = Describe("ingress-nginx :: hooks :: disable_kruise_manager ::", func() {
	const kruiseManagerDeployment = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kruise-controller-manager
  namespace: d8-ingress-nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: kruise
      control-plane: controller-manager
  template:
    metadata:
      labels:
        app: kruise
        control-plane: controller-manager
    spec:
      containers:
      - name: kruise
        image: example.test/kruise:latest
`

	Context("when legacy Kruise management is enabled", func() {
		f := HookExecutionConfigInit(`{"ingressNginx":{"internal":{"legacyKruiseManagementEnabled":true}}}`, "")

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kruiseManagerDeployment))
			f.RunHook()
		})

		It("does not patch the deployment", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(0))
			Expect(f.KubernetesResource("Deployment", "d8-ingress-nginx", "kruise-controller-manager").Field("spec.replicas").Int()).To(BeEquivalentTo(3))
		})
	})

	Context("when legacy Kruise management is disabled", func() {
		f := HookExecutionConfigInit(`{"ingressNginx":{"internal":{"legacyKruiseManagementEnabled":false}}}`, "")

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kruiseManagerDeployment))
			f.RunHook()
		})

		It("scales kruise-controller-manager to zero", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(1))
			Expect(f.KubernetesResource("Deployment", "d8-ingress-nginx", "kruise-controller-manager").Field("spec.replicas").Int()).To(BeEquivalentTo(0))
		})
	})
})
