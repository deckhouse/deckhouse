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
	"time"

	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex user crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "User", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("With adding User object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
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
  ttl: 30m
`))
				f.RunHook()
			})
			It("Should fill internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON(`
[{
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "groups": ["Admins", "Everyone"],
      "password": "password",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf"
}]`))

				Expect(
					f.KubernetesGlobalResource("User", "admin").Field("status.expireAt").Time(),
				).Should(
					// TODO: if you specify fakeClock, the test will be more relevant
					BeTemporally("~", time.Now().Add(30*time.Minute), 5*time.Minute),
				)
			})

			When("User resource changed", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(`
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
  ttl: 1h10m
status:
  expireAt: "2020-02-02T22:22:22Z"
`))
					f.RunHook()
				})

				It("Should not change expire time", func() {
					t, err := time.Parse(time.RFC3339, "2020-02-02T22:22:22Z")
					Expect(f).To(ExecuteSuccessfully())
					Expect(err).ShouldNot(HaveOccurred())
					Expect(
						f.KubernetesGlobalResource("User", "admin").Field("status.expireAt").Time(),
					).Should(
						BeTemporally("==", t),
					)
				})
			})

			Context("With deleting User object", func() {
				BeforeEach(func() {
					f.KubeStateSet("")
					f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
					f.RunHook()
				})
				It("Should delete entry from internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON("[]"))
				})
			})
			Context("With updating User object", func() {
				BeforeEach(func() {
					f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: adminNext@example.com
  groups:
  - Admins
  - Everyone
  password: password
`)
					f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
					f.RunHook()
				})
				It("Should update entry in internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON(`
[{
  "name": "admin",
  "spec": {
    "email": "adminNext@example.com",
    "groups": ["Admins", "Everyone"],
    "password": "password",
    "userID": "admin"
  },
  "encodedName": "mfsg22lonzsxq5camv4gc3lqnrss4y3pnxf7fhheqqrcgji"
}]`))
				})
			})
		})
	})

	Context("Cluster with User object", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
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
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: user
spec:
  email: user@example.com
  groups:
  - Everyone
  password: passwordNext
`))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "groups": [
        "Admins",
        "Everyone"
      ],
      "password": "password",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf"
  },
  {
    "name": "user",
    "spec": {
      "email": "user@example.com",
      "groups": [
        "Everyone"
      ],
      "password": "passwordNext",
      "userID": "user"
    },
    "encodedName": "ovzwk4samv4gc3lqnrss4y3pnxf7fhheqqrcgji"
  }
]`))
		})
	})
})
