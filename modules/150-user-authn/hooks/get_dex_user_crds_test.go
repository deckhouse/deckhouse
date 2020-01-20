package hooks

import (
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex user crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "User", false)

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
apiVersion: deckhouse.io/v1alpha1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  groups:
  - Admins
  - Everyone
  password: password
`))
				f.RunHook()
			})
			It("Should fill internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON(`
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
  }
]`))
			})

			Context("With deleting User object", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
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
					f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: User
metadata:
  name: admin
spec:
  email: adminNext@example.com
  groups:
  - Admins
  - Everyone
  password: password
`))
					f.RunHook()
				})
				It("Should update entry in internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON(`
[
  [
    {
      "name": "admin",
      "spec": {
        "email": "adminNext@example.com",
        "groups": [
          "Admins",
          "Everyone"
        ],
        "password": "password",
        "userID": "admin"
      },
      "encodedName": "mfsg22lojzsxq5camv4gc3lqnrss4y3pnxf7fhheqqrcgji"
    }
  ]
]`))
				})
			})
		})
	})

	Context("Cluster with User object", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
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
apiVersion: deckhouse.io/v1alpha1
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
