// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: migrate_daemonset ", func() {
	initValuesString := `{"ingressNginx":{"defaultControllerVersion": "1.6", "internal": {}}}`
	globalValuesString := `{}`
	f := HookExecutionConfigInit(initValuesString, globalValuesString)
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	Context("Has incompatible ingress controllers", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: test
spec:
  controllerVersion: "1.6"
  ingressClass: "test"
  inlet: "HostWithFailover"
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: test
  name: controller-test-xxx
  namespace: d8-ingress-nginx
spec:
  containers:
  - name: controller
  nodeName: node-1
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: test
    ingress.deckhouse.io/block-deleting: "true"
  name: controller-test-yyy
  namespace: d8-ingress-nginx
spec:
  containers:
  - name: controller
  nodeName: node-2
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: test
    lifecycle.apps.kruise.io/state: "PreparingDelete"
  name: controller-test-zzz
  namespace: d8-ingress-nginx
spec:
  containers:
  - name: controller
  nodeName: node-3
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should add label only to the controller-test-xxx pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod1 := f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-test-xxx")
			pod2 := f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-test-yyy")
			pod3 := f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-test-zzz")

			Expect(pod1.Field("metadata.labels.ingress\\.deckhouse\\.io/block-deleting").String()).To(Equal("true"))
			Expect(pod2.Field("metadata.labels.ingress\\.deckhouse\\.io/block-deleting").String()).To(Equal("true"))
			Expect(pod3.Field("metadata.labels.ingress\\.deckhouse\\.io/block-deleting").Exists()).To(BeFalse())
		})
	})
})
