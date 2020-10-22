package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: migrate providers ::", func() {
	const values = `
{
  "userAuthn": {
    "internal": {
      "providers": {}
    },
    "providers": [
      {
        "id": "oidc notSlu_gif!ed",
        "name": "google",
        "type": "OIDC",
        "oidc": {
          "issuer": "https://issue.example.com",
          "clientID": "plainstring",
          "clientSecret": "plainstring",
          "basicAuthUnsupported": true,
          "insecureSkipEmailVerified": true,
          "getUserInfo": true,
          "scopes": [
            "profile",
            "email"
          ]
        },
        "userIDKey": "subsub",
        "userNameKey": "noname"
      },
      {
        "id": "bitbucket",
        "name": "bitbucket",
        "type": "BitbucketCloud",
        "bitbucketCloud": {
          "clientID": "plainstring",
          "clientSecret": "plainstring",
          "teams": [
            "only",
            "team"
          ]
        }
      }
    ]
  }
}`
	f := HookExecutionConfigInit(values, values)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "DexProvider", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(OnStartupGeneratedBindingContext())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ConfigValuesGet("userAuthn.providers").Exists()).To(BeFalse())

			bitbucketProvider := f.KubernetesGlobalResource("DexProvider", "bitbucket")
			Expect(bitbucketProvider.Exists()).To(BeTrue())
			Expect(bitbucketProvider.ToYaml()).To(MatchYAML(`
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
`))

			oidcProvider := f.KubernetesGlobalResource("DexProvider", "oidc-notslu-gif-ed")
			Expect(oidcProvider.Exists()).To(BeTrue())
			Expect(oidcProvider.ToYaml()).To(MatchYAML(`
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
`))
		})
	})
})
