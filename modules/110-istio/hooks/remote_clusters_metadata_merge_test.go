package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: remote_clusters_metadata_merge ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{},"discovery":{"federations":{}}}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", false)

	Context("Proper federations only", func() {
		BeforeEach(func() {
			f.ValuesSet(`istio.federation.enabled`, true)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-empty
spec:
  trustDomain: "f0"
  metadataEndpoint: "https://some-proper-host/"
status: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-only-ingress
spec:
  trustDomain: "f1"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    ingressGateways:
    - {"address": "aaa", "port": 123}
  rootCA: fff1
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-only-services
spec:
  trustDomain: "f2"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    publicServices:
    - {"hostname": "aaa", "port": 123}
  rootCA: fff2
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-only-full-0
spec:
  trustDomain: "f3"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    ingressGateways:
    - {"address": "bbb", "port": 123}
    publicServices:
    - {"hostname": "bbb", "port": 123}
    rootCA: fff3
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: federation-only-full-1
spec:
  trustDomain: "f4"
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    ingressGateways:
    - {"address": "ccc", "port": 123}
    publicServices:
    - {"hostname": "ccc", "port": 123}
    rootCA: fff4
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: multicluster-0
spec:
  metadataEndpoint: "https://some-proper-host/"
status:
  metadataCache:
    rootCA: mmm0
    apiHost: istio-api.example.com
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			stderrBuff := string(f.Session.Err.Contents())
			Expect(stderrBuff).To(Equal(""))

			Expect(f.ValuesGet("istio.internal.federations").String()).To(MatchJSON(`
[
          {
            "ingressGateways": [
              {
                "address": "bbb",
                "port": 123
              }
            ],
            "name": "federation-only-full-0",
            "publicServices": [
              {
                "hostname": "bbb",
                "port": 123
              }
            ],
            "spiffeEndpoint": "https://some-proper-host/public/spiffe-bundle-endpoint",
            "trustDomain": "f3"
          },
          {
            "ingressGateways": [
              {
                "address": "ccc",
                "port": 123
              }
            ],
            "name": "federation-only-full-1",
            "publicServices": [
              {
                "hostname": "ccc",
                "port": 123
              }
            ],
            "spiffeEndpoint": "https://some-proper-host/public/spiffe-bundle-endpoint",
            "trustDomain": "f4"
          }
        ]
`))
			Expect(f.ValuesGet("istio.internal.multiclusters").String()).To(MatchJSON(`
        [
          {
            "name": "multicluster-0",
            "spiffeEndpoint": "https://some-proper-host/public/spiffe-bundle-endpoint",
            "apiHost": "istio-api.example.com"
          }
        ]
`))
			Expect(f.ValuesGet("istio.internal.remoteRootCAs").String()).To(MatchJSON(`
        [
          "fff3",
          "fff4",
          "mmm0"
        ]
`))
		})
	})
})
