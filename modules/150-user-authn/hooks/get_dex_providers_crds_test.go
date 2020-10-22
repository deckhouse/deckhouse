package hooks

import (
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex providers crds ::", func() {
	const (
		bitbucketCR = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: bitbucket
spec:
  type: BitbucketCloud
  displayName: bitbucket
  bitbucketCloud:
    clientID: plainstring
    clientSecret: plainstring
    teams:
    - only
    - team
`
		oidcCR = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: oidc-notslu-gif-ed
spec:
  type: OIDC
  displayName: google
  oidc:
    basicAuthUnsupported: true
    clientID: plainstring
    clientSecret: plainstring
    getUserInfo: true
    insecureSkipEmailVerified: true
    issuer: https://issue.example.com
    scopes:
    - profile
    - email
`
	)

	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "DexProvider", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("With adding DexProvider object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(bitbucketCR))
				f.RunHook()
			})
			It("Should fill internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.providers").String()).To(MatchJSON(`
[{
  "type": "BitbucketCloud",
  "displayName": "bitbucket",
  "bitbucketCloud": {
    "clientID": "plainstring",
    "clientSecret": "plainstring",
    "teams": [
      "only",
      "team"
    ]
  },
  "id": "bitbucket"
}]`))
			})

			Context("With deleting object", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})
				It("Should delete entry from internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.providers").String()).To(MatchJSON("[]"))
				})
			})
			Context("With adding new provider object", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(bitbucketCR + oidcCR))
					f.RunHook()
				})
				It("Should update entry in internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.providers").String()).To(MatchUnorderedJSON(`
[{
  "type": "OIDC",
  "displayName": "google",
  "oidc": {
    "basicAuthUnsupported": true,
    "clientID": "plainstring",
    "clientSecret": "plainstring",
    "getUserInfo": true,
    "insecureSkipEmailVerified": true,
    "issuer": "https://issue.example.com",
    "scopes": [
      "profile",
      "email"
    ]
  },
  "id": "oidc-notslu-gif-ed"
}, {
  "type": "BitbucketCloud",
  "displayName": "bitbucket",
  "bitbucketCloud": {
    "clientID": "plainstring",
    "clientSecret": "plainstring",
    "teams": [
      "only",
      "team"
    ]
  },
  "id": "bitbucket"
}]`))
				})
			})
		})
	})

	Context("Cluster with DexProvider object", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(bitbucketCR))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.providers").String()).To(MatchJSON(`
[{
  "type": "BitbucketCloud",
  "displayName": "bitbucket",
  "bitbucketCloud": {
    "clientID": "plainstring",
    "clientSecret": "plainstring",
    "teams": [
      "only",
      "team"
    ]
  },
  "id": "bitbucket"
}]`))
		})
	})
})
