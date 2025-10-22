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
  labels:
    test-label: test-value
    certmanager.k8s.io/certificate-name: test-cert-name
    argocd.argoproj.io/instance: test-instance
    argocd.argoproj.io/secret-type: secret-type
    app.kubernetes.io/managed-by: Helm
    app: should-be-removed
    heritage: should-be-removed
    module: should-be-removed
    name: should-be-removed
  annotations:
    test-annotation: test-value
    new-annotation: test-new-value
    kubectl.kubernetes.io/last-applied-configuration: should-be-removed
    meta.helm.sh/release-name: opendistro
    meta.helm.sh/release-namespace: test
    helm.sh/chart: my-chart-1.2.3
    helm.sh/hook: pre-install,pre-upgrade
    ci.werf.io/commit: 90we4affe93154c1200cd3db0f5ee3085c31def6
    ci.werf.io/tag: v1
    gitlab.ci.werf.io/job-url: https://gitlab.example.com/job-url
    gitlab.ci.werf.io/pipeline-url: https://gitlab.example.com/pipeline-url
    project.werf.io/env: test
    project.werf.io/git: https://gitlab.example.com/
    project.werf.io/name: opendistro
    werf.io/release-channel: 1 alpha
    werf.io/version: v1
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
  "clientSecret": "test",
  "labels": {},
  "annotations": {},
  "allowAccessToKubernetes": false
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
  "legacyEncodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn45hizltotf7fhheqqrcgji",
  "labels": {},
  "annotations": {},
  "allowAccessToKubernetes": false
}]`))
				})
			})
			Context("Should allow access to kubernetes api", func() {
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
  annotations:
    dexclient.deckhouse.io/allow-access-to-kubernetes: "true"
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
  "legacyEncodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn45hizltotf7fhheqqrcgji",
  "labels": {},
  "annotations": {
    "dexclient.deckhouse.io/allow-access-to-kubernetes": "true"
  },
  "allowAccessToKubernetes": true
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
  labels:
    test-label: test-value
    certmanager.k8s.io/certificate-name: test-cert-name
    argocd.argoproj.io/secret-type: secret-type
    app.kubernetes.io/managed-by: Helm
    app: should-be-removed
    heritage: should-be-removed
    module: should-be-removed
    name: should-be-removed
  annotations:
    test-annotation: test-value
    kubectl.kubernetes.io/last-applied-configuration: should-be-removed
    meta.helm.sh/release-name: opendistro
    meta.helm.sh/release-namespace: test
    helm.sh/chart: my-chart-1.2.3
    helm.sh/hook: pre-install,pre-upgrade
    ci.werf.io/commit: 90we4affe93154c1200cd3db0f5ee3085c31def6
    ci.werf.io/tag: v1
    gitlab.ci.werf.io/job-url: https://gitlab.example.com/job-url
    gitlab.ci.werf.io/pipeline-url: https://gitlab.example.com/pipeline-url
    project.werf.io/env: test
    project.werf.io/git: https://gitlab.example.com/
    project.werf.io/name: opendistro
    werf.io/release-channel: 1 alpha
    werf.io/version: v1
spec:
  redirectURIs:
  - https://opendistro.example.com/callback
  secretMetadata:
    labels:
      test-label: test-value
    annotations:
      test-annotation: test-value
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
  "clientSecret": "test",
  "labels": {},
  "annotations": {},
  "allowAccessToKubernetes": false
},
{
  "id": "dex-client-opendistro@test",
  "encodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn5ahizltotf7fhheqqrcgji",
  "legacyID": "dex-client-opendistro:test",
  "legacyEncodedID": "mrsxqlldnruwk3tufvxxazlomruxg5dsn45hizltotf7fhheqqrcgji",
  "name": "opendistro",
  "namespace": "test",
  "spec": {
    "redirectURIs": [
      "https://opendistro.example.com/callback"
    ],
    "secretMetadata": {
      "annotations": {
        "test-annotation": "test-value"
      },
      "labels": {
        "test-label": "test-value"
      }
    }
  },
  "clientSecret": "test",
  "labels": {
    "test-label": "test-value"
  },
  "annotations": {
    "test-annotation": "test-value"
  },
  "allowAccessToKubernetes": false
}]`))
		})
	})
})
