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
  "encodedName": "ntjxcdey3v45y",
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
  "encodedName": "iluyx5wsfhyro",
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
  "encodedName": "azro3bhrtkjow",
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
