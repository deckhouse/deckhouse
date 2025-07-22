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
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex providers crds ::", func() {
	const (
		bitbucketCR = `
---
apiVersion: deckhouse.io/v1
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
apiVersion: deckhouse.io/v1
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
    allowedGroups:
    - not-slu-gif-ed
    scopes:
    - profile
    - email
`
	)

	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "DexProvider", false)

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
  "bitbucketCloud": {
    "clientID": "plainstring",
    "clientSecret": "plainstring",
    "includeTeamGroups": false,
    "teams": [
      "only",
      "team"
    ]
  },
  "displayName": "bitbucket",
  "id": "bitbucket",
  "type": "BitbucketCloud"
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
[
{
  "bitbucketCloud": {
    "clientID": "plainstring",
    "clientSecret": "plainstring",
    "includeTeamGroups": false,
    "teams": [
      "only",
      "team"
    ]
  },
  "displayName": "bitbucket",
  "id": "bitbucket",
  "type": "BitbucketCloud"
},
{
  "displayName": "google",
  "id": "oidc-notslu-gif-ed",
  "oidc": {
    "basicAuthUnsupported": true,
    "claimMappingOverride": false,
    "clientID": "plainstring",
    "clientSecret": "plainstring",
    "getUserInfo": true,
    "insecureSkipEmailVerified": true,
    "insecureSkipVerify": false,
    "issuer": "https://issue.example.com",
    "promptType": "consent",
	"allowedGroups": [ "not-slu-gif-ed" ],
    "scopes": [
      "profile",
      "email"
    ],
    "userIDKey": "sub",
    "userNameKey": "name"
  },
  "type": "OIDC"
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
"bitbucketCloud": {
  "clientID": "plainstring",
  "clientSecret": "plainstring",
  "includeTeamGroups": false,
  "teams": [
    "only",
    "team"
  ]
},
"displayName": "bitbucket",
"id": "bitbucket",
"type": "BitbucketCloud"
}]`))
		})
	})
})
