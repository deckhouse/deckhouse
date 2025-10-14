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
	"crypto/sha256"
	"encoding/hex"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex authenticator crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v2alpha1", "DexAuthenticator", true)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("With dex credentials secret after deploying DexAuthenticator object", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-authenticator-test
  namespace: test
  labels:
    app: dex-authenticator
    name: credentials
data:
  client-secret: dGVzdA==
  cookie-secret: dGVzdE5leHR0ZXN0TmV4dHRlc3ROZXh0
---
apiVersion: deckhouse.io/v2alpha1
kind: DexAuthenticator
metadata:
  name: test
  namespace: test
  annotations:
    dexauthenticator.deckhouse.io/allow-access-to-kubernetes: "true"
spec:
  applications:
  - domain: test
    ingressClassName: "nginx"
  sendAuthorizationHeader: false
  nodeSelector:
    testnode: ""
  tolerations:
  - key: foo
    operator: Equal
    value: bar
`))
			f.RunHook()
		})
		It("Should store desired CRDs into values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.dexAuthenticatorCRDs").String()).To(MatchJSON(`
[{
  "uuid": "test@test",
  "name": "test",
  "namespace": "test",
  "spec": {
    "applications": [
      {
        "domain": "test",
        "ingressClassName": "nginx"
      }
    ],
    "sendAuthorizationHeader": false,
    "nodeSelector": {
      "testnode": ""
    },
    "tolerations": [
      {
        "key": "foo",
        "operator": "Equal",
        "value": "bar"
      }
    ]
  },
  "allowAccessToKubernetes": true,
  "encodedName": "orsxg5bnorsxg5bnmrsxqllbov2gqzlooruwgylun5zmx4u44scceizf",
  "credentials": {
    "cookieSecret": "testNexttestNexttestNext",
    "appDexSecret": "test"
  }
}]`))
		})
	})

	Context("After deploying DexAuthenticator and secret in Allowed Namespace", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-authenticator-test
  namespace: d8-dashboard
  labels:
    app: dex-authenticator
    name: credentials
data:
  client-secret: dGVzdA==
  cookie-secret: dGVzdE5leHR0ZXN0TmV4dHRlc3ROZXh0
---
apiVersion: deckhouse.io/v2alpha1
kind: DexAuthenticator
metadata:
  name: test
  namespace: d8-dashboard
  annotations:
    dexauthenticator.deckhouse.io/allow-access-to-kubernetes: "true"
spec:
  applications:
  - domain: test
    ingressClassName: "nginx"
  sendAuthorizationHeader: false
`))
			f.RunHook()
		})
		It("Should store Raw CRDs into values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.dexAuthenticatorCRDs").String()).To(MatchJSON(`
[{
  "uuid": "test@d8-dashboard",
  "name": "test",
  "namespace": "d8-dashboard",
  "spec": {
    "applications": [
      {
        "domain": "test",
        "ingressClassName": "nginx"
      }
    ],
    "sendAuthorizationHeader": false
  },
  "allowAccessToKubernetes": true,
  "encodedName": "orsxg5bnmq4c2zdbonuge33bojsc2zdfpawwc5lunbsw45djmnqxi33szpzjzzeeeirsk",
  "credentials": {
    "cookieSecret": "testNexttestNexttestNext",
    "appDexSecret": "test"
  }
}]`))
		})
	})

	Context("After deploying DexAuthenticator and secret in Allowed Namespace", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-authenticator-test
  namespace: d8-monitoring
  labels:
    app: dex-authenticator
    name: credentials
data:
  client-secret: dGVzdA==
  cookie-secret: dGVzdE5leHR0ZXN0TmV4dHRlc3ROZXh0
---
apiVersion: deckhouse.io/v2alpha1
kind: DexAuthenticator
metadata:
  name: test
  namespace: d8-monitoring
  annotations:
    dexauthenticator.deckhouse.io/allow-access-to-kubernetes: "true"
spec:
  applications:
  - domain: test
    ingressClassName: "nginx"
  sendAuthorizationHeader: false
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-authenticator-test-2
  namespace: d8-monitoring
  labels:
    app: dex-authenticator
    name: credentials
data:
  client-secret: dGVzdA==
  cookie-secret: dGVzdE5leHR0ZXN0TmV4dHRlc3ROZXh0
`))
			f.RunHook()
		})
		It("Should store Raw CRDs into values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.dexAuthenticatorCRDs").String()).To(MatchJSON(`
[{
  "uuid": "test@d8-monitoring",
  "name": "test",
  "namespace": "d8-monitoring",
  "spec": {
    "applications": [
      {
        "domain": "test",
        "ingressClassName": "nginx"
      }
    ],
    "sendAuthorizationHeader": false
  },
  "allowAccessToKubernetes": true,
  "encodedName": "orsxg5bnmq4c23lpnzuxi33snfxgollemv4c2ylvorugk3tunfrwc5dpolf7fhheqqrcgji",
  "credentials": {
    "cookieSecret": "testNexttestNexttestNext",
    "appDexSecret": "test"
  }
}]`))
		})
	})

	Context("With DexAuthenticator with multiple applications", func() {
		const longName = "this-is-a-very-very-long-name-that-will-be-truncated-for-sure"
		const shortName = "short-name"

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v2alpha1
kind: DexAuthenticator
metadata:
  name: ` + longName + `
  namespace: test
spec:
  applications:
  - domain: long.name.one.com
    signOutURL: /logout
  - domain: long.name.two.com
---
apiVersion: deckhouse.io/v2alpha1
kind: DexAuthenticator
metadata:
  name: ` + shortName + `
  namespace: test
spec:
  applications:
  - domain: short.name.one.com
`))
			f.RunHook()
		})
		It("Should fill names map with base, ingress and sign-out ingress names", func() {
			Expect(f).To(ExecuteSuccessfully())
			namesMap := f.ValuesGet("userAuthn.internal.dexAuthenticatorNames")
			Expect(namesMap.Exists()).To(BeTrue())

			// Long name checks
			longID := longName + "@test"
			Expect(len(namesMap.Get(longID).Get("name").String())).Should(BeNumerically("<=", 63))
			Expect(namesMap.Get(longID).Get("truncated").Bool()).Should(BeTrue())
			Expect(namesMap.Get(longID).Get("hash").String()).ShouldNot(BeEmpty())

			Expect(len(namesMap.Get(longID).Get("ingressNames").Get("0").Get("name").String())).Should(BeNumerically("<=", 63))
			Expect(namesMap.Get(longID).Get("ingressNames").Get("0").Get("truncated").Bool()).Should(BeTrue())

			Expect(len(namesMap.Get(longID).Get("signOutIngressNames").Get("0").Get("name").String())).Should(BeNumerically("<=", 63))
			Expect(namesMap.Get(longID).Get("signOutIngressNames").Get("0").Get("truncated").Bool()).Should(BeTrue())

			Expect(len(namesMap.Get(longID).Get("ingressNames").Get("1").Get("name").String())).Should(BeNumerically("<=", 63))
			Expect(namesMap.Get(longID).Get("ingressNames").Get("1").Get("truncated").Bool()).Should(BeTrue())

			// Short name checks
			shortID := shortName + "@test"
			Expect(namesMap.Get(shortID).Get("name").String()).Should(Equal(shortName + "-dex-authenticator"))
			Expect(namesMap.Get(shortID).Get("truncated").Bool()).Should(BeFalse())

			Expect(namesMap.Get(shortID).Get("ingressNames").Get("0").Get("name").String()).Should(Equal(shortName + "-dex-authenticator"))
			Expect(namesMap.Get(shortID).Get("ingressNames").Get("0").Get("truncated").Bool()).Should(BeFalse())
		})
	})
})

var _ = Describe("safeDNS1123Name", func() {
	It("keeps names <=63 as-is", func() {
		name := strings.Repeat("a", 63)
		safe, truncated, hash5 := SafeDNS1123Name(name)
		Expect(safe).To(Equal(name))
		Expect(truncated).To(BeFalse())
		Expect(hash5).To(Equal(""))
	})

	It("truncates names >63 to 57 and appends -hash5", func() {
		original := strings.Repeat("a", 64)
		h := sha256.Sum256([]byte(original))
		expected := strings.Repeat("a", 57) + "-" + hex.EncodeToString(h[:])[:5]
		safe, truncated, hash5 := SafeDNS1123Name(original)
		Expect(truncated).To(BeTrue())
		Expect(len(safe)).To(BeNumerically("<=", 63))
		Expect(safe).To(Equal(expected))
		Expect(hash5).To(Equal(expected[len(expected)-5:]))
	})

	It("normalizes invalid characters and case only when truncating is needed", func() {
		in := "A_B.C-"
		// len("A_B.C-") is 6 <= 63, so function returns input as-is
		safe, truncated, hash5 := SafeDNS1123Name(in)
		Expect(safe).To(Equal(in))
		Expect(truncated).To(BeFalse())
		Expect(hash5).To(Equal(""))
	})

	It("is deterministic for same input", func() {
		in := strings.Repeat("x", 70)
		s1, t1, h1 := SafeDNS1123Name(in)
		s2, t2, h2 := SafeDNS1123Name(in)
		Expect(s1).To(Equal(s2))
		Expect(t1).To(Equal(t2))
		Expect(h1).To(Equal(h2))
	})
})
