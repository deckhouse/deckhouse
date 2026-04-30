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

var _ = Describe("User Authn hooks :: get dex user crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "User", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Group", false)

	Context("User expiration schedule", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  groups:
  - Admins
  - Everyone
  password: password
  ttl: 60m
status:
  expireAt: "2020-02-02T22:22:22Z"
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: future
spec:
  email: future@example.com
  groups:
  - Admins
  - Everyone
  password: password
  ttl: 60m
status:
  expireAt: "2150-10-10T10:10:10Z"
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: without-ttl
spec:
  email: without-ttl@example.com
  groups:
  - Admins
  - Everyone
  password: password
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: admins
spec:
  name: Admins
  members:
  - kind: User
    name: admin
  - kind: User
    name: future
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: everyone
spec:
  name: Everyone
  members:
  - kind: User
    name: admin
  - kind: Group
    name: admins
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		When("User expired (.status.expireAt < time.Now())", func() {
			It("Should delete user CR and remove user from groups", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("User", "admin").Exists()).Should(BeFalse())
				Expect(f.KubernetesGlobalResource("Group", "admins").Field("spec.members").String()).To(MatchJSON(`
[
  {"kind":"User","name":"future"}
]`))
				Expect(f.KubernetesGlobalResource("Group", "everyone").Field("spec.members").String()).To(MatchJSON(`
[
  {"kind":"Group","name":"admins"}
]`))
			})
		})

		When("User not expired (.status.expireAt > time.Now())", func() {
			It("Should keep user CR", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("User", "future").Exists()).Should(BeTrue())
			})
		})

		When("User without ttl", func() {
			It("Should keep user CR", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("User", "without-ttl").Exists()).Should(BeTrue())
			})
		})
	})
})
