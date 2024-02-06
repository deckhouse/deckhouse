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

var _ = Describe("User Authn hooks :: get dex client crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "DexClient", true)

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
apiVersion: deckhouse.io/v1
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
  "id": "dex-client-opendistro@test",
  "encodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn5ahizltotf7fhheqqrcgji",
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
  "legacyID": "dex-client-opendistro:test",
  "legacyEncodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn45hizltotf7fhheqqrcgji",
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
apiVersion: deckhouse.io/v1
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
  "id": "dex-client-opendistro@test",
  "encodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn5ahizltotf7fhheqqrcgji",
  "name": "opendistro",
  "namespace": "test",
  "spec": {
    "redirectURIs": [
      "https://opendistro.example.com/callback"
    ]
  },
  "clientSecret": "test",
  "legacyID": "dex-client-opendistro:test",
  "legacyEncodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn45hizltotf7fhheqqrcgji"
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
apiVersion: deckhouse.io/v1
kind: DexClient
metadata:
  name: opendistro
  namespace: test
spec:
  redirectURIs:
  - https://opendistro.example.com/callback
---
apiVersion: deckhouse.io/v1
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
  "id": "dex-client-grafana@test-grafana",
  "encodedID": "mrsxqlldnruwk3tufvtxeylgmfxgcqdumvzxillhojqwmylomhf7fhheqqrcgji",
  "legacyID": "dex-client-grafana:test-grafana",
  "legacyEncodedID": "mrsxqlldnruwk3tufvtxeylgmfxgcotumvzxillhojqwmylomhf7fhheqqrcgji",
  "name": "grafana",
  "namespace": "test-grafana",
  "spec": {"redirectURIs": ["https://grafana.example.com/callback"]},
  "clientSecret": "test"
},
{
  "id": "dex-client-opendistro@test",
  "encodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn5ahizltotf7fhheqqrcgji",
  "legacyID": "dex-client-opendistro:test",
  "legacyEncodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn45hizltotf7fhheqqrcgji",
  "name": "opendistro",
  "namespace": "test",
  "spec": {"redirectURIs": ["https://opendistro.example.com/callback"]},
  "clientSecret": "test"
}]`))
		})
	})
})
