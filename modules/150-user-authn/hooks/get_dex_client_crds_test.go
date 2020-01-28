package hooks

import (
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex client crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "DexClient", true)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("With adding DexClient object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-client-opendistro
  namespace: test
  labels:
    app: dex-client
    name: credentials
data:
  clientSecret: dGVzdA==
---
apiVersion: deckhouse.io/v1alpha1
kind: DexClient
metadata:
  name: opendistro
  namespace: test
spec:
  redirectURIs:
  - https://opendistro.example.com/callback
  - https://opendistro.example.com/callback-reserve
  allowedGroups:
  - Everyone
  - admins
  trustedPeers:
  - opendistro-sibling
`))
				f.RunHook()
			})
			It("Should fill internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.dexClientCRDs").String()).To(MatchJSON(`
[{
  "id": "dex-client-opendistro:test",
  "name": "opendistro",
  "namespace": "test",
  "spec": {
    "allowedGroups": [
	  "Everyone",
	  "admins"
    ],
    "redirectURIs": [
      "https://opendistro.example.com/callback",
      "https://opendistro.example.com/callback-reserve"
    ],
    "trustedPeers": ["opendistro-sibling"]
  },
  "encodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn45hizltotf7fhheqqrcgji",
  "clientSecret": "test"
}]`))
			})

			Context("With deleting User object", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})
				It("Should delete entry from internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.dexClientCRDs").String()).To(MatchJSON("[]"))
				})
			})
			Context("With updating DexClient object", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-client-opendistro
  namespace: test
  labels:
    app: dex-client
    name: credentials
data:
  clientSecret: dGVzdA==
---
apiVersion: deckhouse.io/v1alpha1
kind: DexClient
metadata:
  name: opendistro
  namespace: test
spec:
  redirectURIs:
  - https://opendistro.example.com/callback
`))
					f.RunHook()
				})
				It("Should update entry in internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.dexClientCRDs").String()).To(MatchJSON(`
[{
  "id": "dex-client-opendistro:test",
  "name": "opendistro",
  "namespace": "test",
  "spec": {"redirectURIs": ["https://opendistro.example.com/callback"]},
  "encodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn45hizltotf7fhheqqrcgji",
  "clientSecret": "test"
}]`))
				})
			})
		})
	})

	Context("Cluster with DexClient object", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-client-opendistro
  namespace: test
  labels:
    app: dex-client
    name: credentials
data:
  clientSecret: dGVzdA==
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-client-grafana
  namespace: test-grafana
  labels:
    app: dex-client
    name: credentials
data:
  clientSecret: dGVzdA==
---
apiVersion: deckhouse.io/v1alpha1
kind: DexClient
metadata:
  name: opendistro
  namespace: test
spec:
  redirectURIs:
  - https://opendistro.example.com/callback
---
apiVersion: deckhouse.io/v1alpha1
kind: DexClient
metadata:
  name: grafana
  namespace: test-grafana
spec:
  redirectURIs:
  - https://grafana.example.com/callback
`))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexClientCRDs").String()).To(MatchUnorderedJSON(`
[{
  "id": "dex-client-grafana:test-grafana",
  "name": "grafana",
  "namespace": "test-grafana",
  "spec": {"redirectURIs": ["https://grafana.example.com/callback"]},
  "encodedID": "mrsxqlldnruwk3tufvtxeylgmfxgcotumvzxillhojqwmylomhf7fhheqqrcgji",
  "clientSecret": "test"
},
{
  "id": "dex-client-opendistro:test",
  "name": "opendistro",
  "namespace": "test",
  "spec": {"redirectURIs": ["https://opendistro.example.com/callback"]},
  "encodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn45hizltotf7fhheqqrcgji",
  "clientSecret": "test"
}]`))
		})
	})
})
