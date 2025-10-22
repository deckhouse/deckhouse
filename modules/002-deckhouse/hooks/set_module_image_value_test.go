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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: set module image value ::", func() {
	f := HookExecutionConfigInit(`
global:
  deckhouseVersion: "12345"
  modulesImages:
    registry:
      base: registry.deckhouse.io/deckhouse/fe
deckhouse:
  internal: {}
`, `{}`)

	Context("With Deckhouse pod", func() {
		Context("when image in absent values", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: deckhouse
    heritage: deckhouse
    module: deckhouse
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.deckhouse.io/deckhouse/ce:test
`, 1))
				f.RunHook()
			})

			It("Should run", func() {
				Expect(f).To(ExecuteSuccessfully())
				deployment := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(deployment.Exists()).To(BeTrue())
				Expect(f.ValuesGet("deckhouse.internal.currentReleaseImageName").String()).To(Equal("registry.deckhouse.io/deckhouse/fe:test"))
			})
		})

		Context("when image in absent values and regstry with port", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: deckhouse
    heritage: deckhouse
    module: deckhouse
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.deckhouse.io:666/deckhouse/ce:test666
`, 1))
				f.RunHook()
			})

			It("Should run", func() {
				Expect(f).To(ExecuteSuccessfully())
				deployment := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(deployment.Exists()).To(BeTrue())
				Expect(f.ValuesGet("deckhouse.internal.currentReleaseImageName").String()).To(Equal("registry.deckhouse.io/deckhouse/fe:test666"))
			})
		})

		Context("when image in present values", func() {
			BeforeEach(func() {
				f.ValuesSet("deckhouse.internal.currentReleaseImageName", "registry.deckhouse.io/deckhouse/ce/initial:test")
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: deckhouse
    heritage: deckhouse
    module: deckhouse
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.deckhouse.io/deckhouse/ce/different:test
`, 1))
				f.RunHook()
			})

			It("Should run", func() {
				Expect(f).To(ExecuteSuccessfully())
				deployment := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
				Expect(deployment.Exists()).To(BeTrue())
				Expect(f.ValuesGet("deckhouse.internal.currentReleaseImageName").String()).To(Equal("registry.deckhouse.io/deckhouse/ce/initial:test"))
			})
		})
	})
})
