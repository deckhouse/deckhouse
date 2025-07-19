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

var _ = Describe("User Authn hooks :: name generation functions ::", func() {
	Context("generateSafeName function", func() {
		It("Should return simple name when within 63 characters", func() {
			result := generateSafeName("test", "suffix", false)
			Expect(result).To(Equal("test-suffix"))
			Expect(len(result)).To(BeNumerically("<=", 63))
		})

		It("Should return simple name with prefix when within 63 characters", func() {
			result := generateSafeName("test", "prefix", true)
			Expect(result).To(Equal("prefix-test"))
			Expect(len(result)).To(BeNumerically("<=", 63))
		})

		It("Should truncate and add hash when exceeding 63 characters", func() {
			longName := "very-long-name-that-will-definitely-exceed-the-kubernetes-name-limit"
			result := generateSafeName(longName, "dex-authenticator", false)
			Expect(len(result)).To(BeNumerically("<=", 63))
			Expect(result).To(ContainSubstring("dex-authenticator"))
			Expect(result).To(MatchRegexp(`.*-[a-f0-9]{8}-dex-authenticator$`))
		})

		It("Should truncate and add hash with prefix when exceeding 63 characters", func() {
			longName := "very-long-name-that-will-definitely-exceed-the-kubernetes-name-limit"
			result := generateSafeName(longName, "dex-authenticator", true)
			Expect(len(result)).To(BeNumerically("<=", 63))
			Expect(result).To(HavePrefix("dex-authenticator-"))
			Expect(result).To(MatchRegexp(`^dex-authenticator-.*-[a-f0-9]{8}$`))
		})

		It("Should handle minimum edge case", func() {
			result := generateSafeName("a", "very-long-fixed-part-that-makes-calculation-tight", false)
			Expect(len(result)).To(BeNumerically("<=", 63))
		})
	})

	Context("dexAuthenticatorNameWithNamespace function", func() {
		It("Should create correct name for short inputs", func() {
			result := dexAuthenticatorNameWithNamespace("test", "namespace")
			Expect(result).To(Equal("test-namespace-dex-authenticator"))
		})

		It("Should create truncated name with hash for long inputs", func() {
			result := dexAuthenticatorNameWithNamespace("very-long-authenticator-name", "very-long-namespace-name")
			Expect(len(result)).To(BeNumerically("<=", 63))
			Expect(result).To(HaveSuffix("-dex-authenticator"))
			Expect(result).To(MatchRegexp(`.*-[a-f0-9]{8}-dex-authenticator$`))
		})
	})

	Context("dexAuthenticatorNameReverse function", func() {
		It("Should create correct name for short inputs", func() {
			result := dexAuthenticatorNameReverse("test")
			Expect(result).To(Equal("dex-authenticator-test"))
		})

		It("Should create truncated name with hash for long inputs", func() {
			result := dexAuthenticatorNameReverse("very-long-authenticator-name-that-exceeds-limit")
			Expect(len(result)).To(BeNumerically("<=", 63))
			Expect(result).To(HavePrefix("dex-authenticator-"))
			Expect(result).To(MatchRegexp(`^dex-authenticator-.*-[a-f0-9]{8}$`))
		})
	})
})

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
  "encodedName": "9a096589",
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
  "encodedName": "e422aa71",
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
  "encodedName": "13c2b776",
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
  "credentials": {
    "cookieSecret": "testNexttestNexttestNext",
    "appDexSecret": "test"
  }
}]`))
		})
	})
})
