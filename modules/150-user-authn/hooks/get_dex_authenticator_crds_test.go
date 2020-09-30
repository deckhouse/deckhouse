package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex authenticator crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "DexAuthenticator", true)

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
  cookie-secret: dGVzdE5leHQ=
---
apiVersion: deckhouse.io/v1alpha1
kind: DexAuthenticator
metadata:
  name: test
  namespace: test
  annotations:
    dexauthenticator.deckhouse.io/allow-access-to-kubernetes: "true"
spec:
  applicationDomain: test
  sendAuthorizationHeader: false
  applicationIngressClassName: "nginx"
  nodeSelector:
    testnode: ""
  tolerations:
  - key: foo
    operator: Equal
    value: bar
`))
			f.RunHook()
		})
		It("Should store desired СRDs into values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.dexAuthenticatorCRDs").String()).To(MatchJSON(`
[{
  "uuid": "test@test",
  "name": "test",
  "namespace": "test",
  "spec": {
    "applicationDomain": "test",
    "applicationIngressClassName": "nginx",
    "sendAuthorizationHeader": false,
    "nodeSelector":
      {
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
  "allowAccessToKubernetes": "",
  "encodedName": "orsxg5bnorsxg5bnmrsxqllbov2gqzlooruwgylun5zmx4u44scceizf",
  "credentials": {
    "cookieSecret": "testNext",
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
  cookie-secret: dGVzdE5leHQ=
---
apiVersion: deckhouse.io/v1alpha1
kind: DexAuthenticator
metadata:
  name: test
  namespace: d8-dashboard
  annotations:
    dexauthenticator.deckhouse.io/allow-access-to-kubernetes: "true"
spec:
  applicationDomain: test
  sendAuthorizationHeader: false
  applicationIngressClassName: "nginx"
`))
			f.RunHook()
		})
		It("Should store Raw СRDs into values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.dexAuthenticatorCRDs").String()).To(MatchJSON(`
[{
  "uuid": "test@d8-dashboard",
  "name": "test",
  "namespace": "d8-dashboard",
  "spec": {
    "applicationDomain": "test",
    "applicationIngressClassName": "nginx",
    "sendAuthorizationHeader": false
  },
  "allowAccessToKubernetes": "true",
  "encodedName": "orsxg5bnmq4c2zdbonuge33bojsc2zdfpawwc5lunbsw45djmnqxi33szpzjzzeeeirsk",
  "credentials": {
    "cookieSecret": "testNext",
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
  cookie-secret: dGVzdE5leHQ=
---
apiVersion: deckhouse.io/v1alpha1
kind: DexAuthenticator
metadata:
  name: test
  namespace: d8-monitoring
  annotations:
    dexauthenticator.deckhouse.io/allow-access-to-kubernetes: "true"
spec:
  applicationDomain: test
  sendAuthorizationHeader: false
  applicationIngressClassName: "nginx"
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
  cookie-secret: dGVzdE5leHQ=
`))
			f.RunHook()
		})
		It("Should store Raw СRDs into values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.dexAuthenticatorCRDs").String()).To(MatchJSON(`
[{
  "uuid": "test@d8-monitoring",
  "name": "test",
  "namespace": "d8-monitoring",
  "spec": {
    "applicationDomain": "test",
    "applicationIngressClassName": "nginx",
    "sendAuthorizationHeader": false
  },
  "allowAccessToKubernetes": "",
  "encodedName": "orsxg5bnmq4c23lpnzuxi33snfxgollemv4c2ylvorugk3tunfrwc5dpolf7fhheqqrcgji",
  "credentials": {
    "cookieSecret": "testNext",
    "appDexSecret": "test"
  }
}]`))
		})
	})
})
