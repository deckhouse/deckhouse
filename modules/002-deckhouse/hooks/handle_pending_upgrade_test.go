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

var _ = Describe("Modules :: deckhouse :: hooks :: wait for deckhouse update ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Deckhouse release is upgrading", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: Secret
type: helm.sh/release.v1
metadata:
  creationTimestamp: "2023-03-02T12:01:00Z"
  labels:
    name: deckhouse
    owner: helm
    status: superseded
  name: sh.helm.release.v1.deckhouse.v1
  namespace: d8-system
---
apiVersion: v1
kind: Secret
type: helm.sh/release.v1
metadata:
  creationTimestamp: "2023-03-02T12:02:00Z"
  labels:
    name: deckhouse
    owner: helm
    status: deployed
  name: sh.helm.release.v1.deckhouse.v2
  namespace: d8-system
---
apiVersion: v1
kind: Secret
type: helm.sh/release.v1
metadata:
  creationTimestamp: "2023-03-02T12:03:00Z"
  labels:
    name: deckhouse
    owner: helm
    status: pending-upgrade
  name: sh.helm.release.v1.deckhouse.v3
  namespace: d8-system
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should delete the secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Secret", "d8-system", "sh.helm.release.v1.deckhouse.v3").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", "sh.helm.release.v1.deckhouse.v2").Exists()).To(BeTrue())
		})
	})

	Context("Deckhouse release is deployed", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: Secret
type: helm.sh/release.v1
metadata:
  creationTimestamp: "2023-03-02T12:04:00Z"
  labels:
    name: deckhouse
    owner: helm
    status: superseded
  name: sh.helm.release.v1.deckhouse.v4
  namespace: d8-system
---
apiVersion: v1
kind: Secret
type: helm.sh/release.v1
metadata:
  creationTimestamp: "2023-03-02T12:05:00Z"
  labels:
    name: deckhouse
    owner: helm
    status: superseded
  name: sh.helm.release.v1.deckhouse.v5
  namespace: d8-system
---
apiVersion: v1
kind: Secret
type: helm.sh/release.v1
metadata:
  creationTimestamp: "2023-03-02T12:06:00Z"
  labels:
    name: deckhouse
    owner: helm
    status: deployed
  name: sh.helm.release.v1.deckhouse.v6
  namespace: d8-system
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should keep the secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Secret", "d8-system", "sh.helm.release.v1.deckhouse.v6").Exists()).To(BeTrue())
		})
	})

})
